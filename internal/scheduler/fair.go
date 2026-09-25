package scheduler

import (
	"time"

	"gpt-load/internal/state"
)

// ChargeReplay reserves a slot for an allowed explicit same-credential replay without relaxing tried deduplication for ordinary retries.
func (iterator *Iterator) ChargeReplay(selection Selection, ref state.CredentialRef) bool {
	if ref.ID != selection.CredentialID || ref.GroupID != selection.GroupID {
		return false
	}
	var charged bool
	consume := func(metas []state.CredentialMeta) {
		for _, meta := range metas {
			if meta.ID != ref.ID || meta.IdentityGeneration != ref.IdentityGeneration {
				continue
			}
			now := iterator.now()
			if selection.UpstreamModelID != nil &&
				modelCooldownUntil(meta.ModelCooldowns, *selection.UpstreamModelID, iterator.operation, now).After(now) {
				continue
			}
			weight := effectiveWeight(selection.Group.WeightManual, meta.WeightManual)
			if weight > 0 {
				previousRetryID, previousRetryGroupID := iterator.retrySameCredentialID, iterator.retrySameGroupID
				iterator.retrySameCredentialID, iterator.retrySameGroupID = ref.ID, ref.GroupID
				selected, found := iterator.selectCredential(
					[]weightedCredential{{meta: meta, weight: weight}}, ref.ID, now,
				)
				iterator.retrySameCredentialID, iterator.retrySameGroupID = previousRetryID, previousRetryGroupID
				charged = found && selected.meta.ID == ref.ID
			}
		}
	}
	groups := []uint{ref.GroupID}
	if source, ok := iterator.credentials.(interface {
		WithCredentialCandidates([]uint, func(uint) bool, time.Time, func([]state.CredentialMeta))
	}); ok {
		source.WithCredentialCandidates(groups, nil, iterator.now(), consume)
	} else {
		consume(iterator.credentials.CollectCredentialCandidates(groups, nil, iterator.now()))
	}
	return charged
}

// selectCredential operates only in memory; actual Registry callers hold the credential read lock concurrently.
func (iterator *Iterator) selectCredential(
	candidates []weightedCredential,
	preferred uint,
	now time.Time,
) (weightedCredential, bool) {
	var selected weightedCredential
	var found bool
	iterator.progress.WithLock(func(ledger *state.SchedulingLedger) {
		eligible := candidates[:0]
		baseline := ledger.Watermark
		hasBaseline := false
		for _, candidate := range candidates {
			member := ledger.Members[candidate.meta.ID]
			if member == nil || member.GroupID != candidate.meta.GroupID ||
				member.IdentityGeneration != candidate.meta.IdentityGeneration {
				continue
			}
			eligible = append(eligible, candidate)
			if !member.Pending && (!hasBaseline || member.Progress.Compare(baseline) > 0) {
				baseline, hasBaseline = member.Progress, true
			}
		}
		if len(eligible) == 0 || ledger.Sequence == ^uint64(0) {
			return
		}

		selectionPool := make([]weightedCredential, 0, len(eligible))
		serialGroups := make(map[uint]struct{})
		for _, candidate := range eligible {
			group, ok := iterator.snapshot.Groups[candidate.meta.GroupID]
			if !ok || group.AccountSelection != state.AccountSelectionSerial {
				selectionPool = append(selectionPool, candidate)
				continue
			}
			if _, visited := serialGroups[group.ID]; visited {
				continue
			}
			serialGroups[group.ID] = struct{}{}
			serialCandidate, isProbe, available := selectSerialCandidateLocked(
				ledger, eligible, group, now, iterator.tried, iterator.retrySameCredentialID,
			)
			if available {
				serialCandidate.serialProbe = isProbe
				selectionPool = append(selectionPool, serialCandidate)
			}
		}
		eligible = selectionPool
		if len(eligible) == 0 {
			return
		}

		// Members in the same batch share the leading progress of existing members and cannot inherit a lagging member's historical debt.
		for _, candidate := range eligible {
			member := ledger.Members[candidate.meta.ID]
			if member.Pending && (!ledger.GroupsKnown || ledger.Groups[member.GroupID]) {
				member.Admit(baseline)
			}
		}
		less := func(left, right weightedCredential) bool {
			a, b := ledger.Members[left.meta.ID], ledger.Members[right.meta.ID]
			if order := a.Progress.Compare(b.Progress); order != 0 {
				return order < 0
			}
			if a.LastSelected != b.LastSelected {
				return a.LastSelected < b.LastSelected
			}
			return a.ID < b.ID
		}
		first := eligible[0]
		for _, candidate := range eligible[1:] {
			if less(candidate, first) {
				first = candidate
			}
		}
		preferredFound := false
		for _, candidate := range eligible {
			if candidate.meta.ID == preferred {
				first, preferredFound = candidate, true
				break
			}
		}
		if !preferredFound && ledger.LastMember == first.meta.ID && len(eligible) > 1 && ledger.Consecutive >= 100 {
			var alternative weightedCredential
			for _, candidate := range eligible {
				if candidate.meta.ID != first.meta.ID && (alternative.meta.ID == 0 || less(candidate, alternative)) {
					alternative = candidate
				}
			}
			limit := uint64(99 + (first.weight+alternative.weight-1)/alternative.weight)
			if ledger.Consecutive >= limit {
				first = alternative
			}
		}
		probeAccount := false
		probeAlreadyInFlight := false
		if first.serialProbe || first.serialProbePending {
			serial := ledger.SerialGroups[first.meta.GroupID]
			if serial == nil {
				return
			}
			account := serial.Accounts[first.meta.ID]
			if account == nil || account.IdentityGeneration != first.meta.IdentityGeneration ||
				account.ProbeInFlight && !first.serialProbePending {
				return
			}
			probeAccount = true
			probeAlreadyInFlight = account.ProbeInFlight
			account.ProbeInFlight = true
			if first.serialProbePending {
				account.ProbeVerified = true
			}
		}
		member := ledger.Members[first.meta.ID]
		next, ok := member.Progress.Advance(uint64(first.weight))
		if !ok {
			if probeAccount {
				ledger.SerialGroups[first.meta.GroupID].Accounts[first.meta.ID].ProbeInFlight = probeAlreadyInFlight
			}
			return
		}
		member.Progress = next
		ledger.Sequence++
		member.LastSelected = ledger.Sequence
		ledger.Started = true
		if next.Compare(ledger.Watermark) > 0 {
			ledger.Watermark = next
		}
		if ledger.LastMember != member.ID {
			ledger.LastMember, ledger.Consecutive = member.ID, 1
		} else if ledger.Consecutive < 99+state.MaxWeight*state.MaxWeight {
			// Once all legal thresholds are exceeded, do not continue growing; affinity and a single candidate can still be allocated.
			ledger.Consecutive++
		}
		selected, found = first, true
	})
	return selected, found
}
