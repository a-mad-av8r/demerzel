package scheduler

import (
	"time"

	"gpt-load/internal/execution"
	"gpt-load/internal/health"
	"gpt-load/internal/state"
)

const (
	serialFailbackGrace     = time.Minute
	serialProbeInitialDelay = 5 * time.Second
	serialProbeMaximumDelay = 5 * time.Minute
)

type SerialProbeResult uint8

const (
	SerialProbeSucceeded SerialProbeResult = iota + 1
	SerialProbeFailed
	SerialProbeUnsupported
)

func selectSerialCandidateLocked(
	ledger *state.SchedulingLedger,
	candidates []weightedCredential,
	group state.GroupView,
	now time.Time,
	tried map[uint]struct{},
	retryID uint,
) (weightedCredential, bool, bool) {
	serial := ledger.SerialGroups[group.ID]
	if serial == nil {
		serial = &state.SerialGroupState{Accounts: make(map[uint]*state.SerialAccountState)}
		ledger.SerialGroups[group.ID] = serial
	}
	if serial.Accounts == nil {
		serial.Accounts = make(map[uint]*state.SerialAccountState)
	}
	for id, account := range serial.Accounts {
		member := ledger.Members[id]
		if member == nil || member.GroupID != group.ID || member.IdentityGeneration != account.IdentityGeneration {
			delete(serial.Accounts, id)
		}
	}
	cursorMember := ledger.Members[serial.CursorCredentialID]
	if serial.CursorCredentialID != 0 && (cursorMember == nil || cursorMember.GroupID != group.ID ||
		cursorMember.IdentityGeneration != serial.CursorIdentityGeneration) {
		serial.CursorCredentialID, serial.CursorIdentityGeneration = 0, 0
	}

	for _, candidate := range candidates {
		meta := candidate.meta
		if meta.GroupID != group.ID || meta.QuotaRemaining == nil || meta.QuotaResetAt.IsZero() {
			continue
		}
		account := serialAccountState(serial, meta)
		reserve := float64(group.SerialQuotaReservePercent) / 100
		if *meta.QuotaRemaining > reserve {
			if serialAccountBlocked(account) && meta.QuotaResetAt.After(account.ResetAt) {
				nextProbeAt := later(account.ResetAt, account.CooldownUntil).Add(serialFailbackGrace)
				account.ResetAt, account.CooldownUntil = meta.QuotaResetAt, time.Time{}
				account.NextProbeAt = nextProbeAt
				account.ProbeFailures, account.ProbeInFlight, account.ProbeVerified = 0, false, false
			}
			continue
		}
		if account.SuppressedQuotaResetAt.Equal(meta.QuotaResetAt) {
			continue
		}
		if serialAccountBlocked(account) {
			if meta.QuotaResetAt.After(account.ResetAt) {
				account.ResetAt = meta.QuotaResetAt
			}
			minimumProbeAt := later(account.ResetAt, account.CooldownUntil).Add(serialFailbackGrace)
			if account.NextProbeAt.Before(minimumProbeAt) {
				account.NextProbeAt = minimumProbeAt
			}
			continue
		}
		account.ResetAt, account.CooldownUntil = meta.QuotaResetAt, meta.QuotaResetAt
		account.NextProbeAt = meta.QuotaResetAt.Add(serialFailbackGrace)
		account.ProbeFailures, account.ProbeInFlight, account.ProbeVerified = 0, false, false
	}

	var probe weightedCredential
	probeFound := false
	if retryID != 0 {
		for _, candidate := range candidates {
			if candidate.meta.GroupID != group.ID || candidate.meta.ID != retryID {
				continue
			}
			account := serial.Accounts[retryID]
			if account != nil && account.IdentityGeneration == candidate.meta.IdentityGeneration && account.ProbeVerified {
				candidate.serialProbePending = true
				return candidate, false, true
			}
		}
	}
	for _, candidate := range candidates {
		meta := candidate.meta
		if meta.GroupID != group.ID || serialCandidateTried(meta.ID, tried, retryID) {
			continue
		}
		account := serial.Accounts[meta.ID]
		if !serialAccountBlocked(account) || account.ProbeInFlight {
			continue
		}
		probeAt := account.NextProbeAt
		if probeAt.IsZero() {
			probeAt = later(account.ResetAt, account.CooldownUntil).Add(serialFailbackGrace)
		}
		if now.Before(probeAt) {
			continue
		}
		candidate.serialProbe = !account.ProbeUnsupported
		candidate.serialProbePending = account.ProbeUnsupported
		if !probeFound || meta.ID < probe.meta.ID {
			probe, probeFound = candidate, true
		}
	}
	if probeFound {
		return probe, probe.serialProbe, true
	}

	if serial.CursorCredentialID != 0 {
		cursorBlocked := serialAccountBlocked(serial.Accounts[serial.CursorCredentialID])
		for _, candidate := range candidates {
			if candidate.meta.ID != serial.CursorCredentialID || candidate.meta.GroupID != group.ID ||
				serialCandidateTried(candidate.meta.ID, tried, retryID) || cursorBlocked {
				continue
			}
			return candidate, false, true
		}
		if cursorBlocked {
			if next, ok := nextSerialCandidate(candidates, group.ID, serial.CursorCredentialID, serial, tried, retryID); ok {
				serial.CursorCredentialID = next.meta.ID
				serial.CursorIdentityGeneration = next.meta.IdentityGeneration
				return next, false, true
			}
			return weightedCredential{}, false, false
		}
		if _, alreadyTried := tried[serial.CursorCredentialID]; alreadyTried && retryID != serial.CursorCredentialID {
			// A scoped retry may try another account for this request without changing the cursor.
			return firstAvailableSerialCandidate(candidates, group.ID, serial, tried, retryID)
		}
		// The cursor may not serve this route/model or may be temporarily ineligible.
		// Use another account for this request without persisting a failover.
		return firstAvailableSerialCandidate(candidates, group.ID, serial, tried, retryID)
	}

	selected, _, ok := firstAvailableSerialCandidate(candidates, group.ID, serial, tried, retryID)
	if ok {
		serial.CursorCredentialID = selected.meta.ID
		serial.CursorIdentityGeneration = selected.meta.IdentityGeneration
	}
	return selected, false, ok
}

func firstAvailableSerialCandidate(
	candidates []weightedCredential,
	groupID uint,
	serial *state.SerialGroupState,
	tried map[uint]struct{},
	retryID uint,
) (weightedCredential, bool, bool) {
	var selected weightedCredential
	for _, candidate := range candidates {
		if candidate.meta.GroupID != groupID || serialCandidateTried(candidate.meta.ID, tried, retryID) ||
			serialAccountBlocked(serial.Accounts[candidate.meta.ID]) {
			continue
		}
		if selected.meta.ID == 0 || candidate.meta.ID < selected.meta.ID {
			selected = candidate
		}
	}
	return selected, false, selected.meta.ID != 0
}

func nextSerialCandidate(
	candidates []weightedCredential,
	groupID uint,
	cursorID uint,
	serial *state.SerialGroupState,
	tried map[uint]struct{},
	retryID uint,
) (weightedCredential, bool) {
	var after, first weightedCredential
	for _, candidate := range candidates {
		if candidate.meta.GroupID != groupID || serialCandidateTried(candidate.meta.ID, tried, retryID) ||
			serialAccountBlocked(serial.Accounts[candidate.meta.ID]) {
			continue
		}
		if first.meta.ID == 0 || candidate.meta.ID < first.meta.ID {
			first = candidate
		}
		if candidate.meta.ID > cursorID && (after.meta.ID == 0 || candidate.meta.ID < after.meta.ID) {
			after = candidate
		}
	}
	if after.meta.ID != 0 {
		return after, true
	}
	return first, first.meta.ID != 0
}

func serialCandidateTried(id uint, tried map[uint]struct{}, retryID uint) bool {
	if id == retryID {
		return false
	}
	_, exists := tried[id]
	return exists
}

func serialAccountState(serial *state.SerialGroupState, meta state.CredentialMeta) *state.SerialAccountState {
	account := serial.Accounts[meta.ID]
	if account == nil || account.IdentityGeneration != meta.IdentityGeneration {
		account = &state.SerialAccountState{IdentityGeneration: meta.IdentityGeneration}
		serial.Accounts[meta.ID] = account
	}
	return account
}

func serialAccountBlocked(account *state.SerialAccountState) bool {
	return account != nil && (!account.ResetAt.IsZero() || !account.CooldownUntil.IsZero())
}

func later(left, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
}

func (iterator *Iterator) RecordSerialProbe(selection Selection, result SerialProbeResult, now time.Time) {
	if iterator == nil || selection.Group.AccountSelection != state.AccountSelectionSerial || !selection.SerialProbe {
		return
	}
	iterator.progress.WithLock(func(ledger *state.SchedulingLedger) {
		serial := ledger.SerialGroups[selection.GroupID]
		if serial == nil {
			return
		}
		account := serial.Accounts[selection.CredentialID]
		if account == nil || account.IdentityGeneration != selection.IdentityGeneration || !account.ProbeInFlight {
			return
		}
		switch result {
		case SerialProbeSucceeded:
			account.ProbeVerified = true
		case SerialProbeUnsupported:
			account.ProbeVerified = true
			account.ProbeUnsupported = true
		case SerialProbeFailed:
			account.ProbeInFlight, account.ProbeVerified = false, false
			if account.ProbeFailures < 32 {
				account.ProbeFailures++
			}
			account.NextProbeAt = now.Add(serialProbeBackoff(account.ProbeFailures))
		}
	})
}

func (iterator *Iterator) AbandonSerialProbe(selection Selection, now time.Time) {
	if iterator == nil || selection.Group.AccountSelection != state.AccountSelectionSerial ||
		!selection.SerialProbePending {
		return
	}
	iterator.progress.WithLock(func(ledger *state.SchedulingLedger) {
		serial := ledger.SerialGroups[selection.GroupID]
		if serial == nil {
			return
		}
		account := serial.Accounts[selection.CredentialID]
		if account == nil || account.IdentityGeneration != selection.IdentityGeneration ||
			!account.ProbeInFlight && !account.ProbeVerified {
			return
		}
		account.ProbeInFlight, account.ProbeVerified = false, false
		if account.ProbeFailures < 32 {
			account.ProbeFailures++
		}
		resetAt := later(account.ResetAt, account.CooldownUntil).Add(serialFailbackGrace)
		account.NextProbeAt = later(resetAt, now.Add(serialProbeBackoff(account.ProbeFailures)))
	})
}

func serialProbeBackoff(failures int) time.Duration {
	delay := serialProbeInitialDelay
	for attempt := 1; attempt < failures && delay < serialProbeMaximumDelay; attempt++ {
		if delay > serialProbeMaximumDelay/2 {
			return serialProbeMaximumDelay
		}
		delay *= 2
	}
	if delay > serialProbeMaximumDelay {
		return serialProbeMaximumDelay
	}
	return delay
}

func (iterator *Iterator) RecordAttempt(
	selection Selection,
	decision health.Decision,
	now time.Time,
	willRetry bool,
) {
	if iterator == nil || selection.Group.AccountSelection != state.AccountSelectionSerial {
		return
	}
	if selection.SerialProbePending {
		iterator.recordSerialProbeInferenceAttempt(selection, decision, now, willRetry)
		return
	}
	if decision.Category == health.FailureCategoryRateLimited && decision.Scope == execution.ErrorScopeCredential {
		iterator.progress.WithLock(func(ledger *state.SchedulingLedger) {
			serial := ledger.SerialGroups[selection.GroupID]
			if serial == nil {
				serial = &state.SerialGroupState{Accounts: make(map[uint]*state.SerialAccountState)}
				ledger.SerialGroups[selection.GroupID] = serial
			}
			if serial.Accounts == nil {
				serial.Accounts = make(map[uint]*state.SerialAccountState)
			}
			account := serialAccountState(serial, state.CredentialMeta{
				ID: selection.CredentialID, GroupID: selection.GroupID,
				IdentityGeneration: selection.IdentityGeneration,
			})
			cooldownUntil := decision.CooldownUntil
			if cooldownUntil.IsZero() {
				cooldownUntil = now.Add(time.Minute)
			}
			resetAt := later(cooldownUntil, selection.QuotaResetAt)
			account.ResetAt, account.CooldownUntil = resetAt, cooldownUntil
			account.NextProbeAt = later(resetAt, cooldownUntil).Add(serialFailbackGrace)
			account.ProbeFailures, account.ProbeInFlight, account.ProbeVerified = 0, false, false
		})
		return
	}
	if !willRetry || !serialSameAccountRetry(decision) {
		return
	}
	iterator.retrySameCredentialID = selection.CredentialID
	iterator.retrySameGroupID = selection.GroupID
}

func (iterator *Iterator) recordSerialProbeInferenceAttempt(
	selection Selection,
	decision health.Decision,
	now time.Time,
	willRetry bool,
) {
	retrySame := false
	iterator.progress.WithLock(func(ledger *state.SchedulingLedger) {
		serial := ledger.SerialGroups[selection.GroupID]
		if serial == nil {
			return
		}
		account := serial.Accounts[selection.CredentialID]
		if account == nil || account.IdentityGeneration != selection.IdentityGeneration || !account.ProbeVerified {
			return
		}
		if decision.Category == health.FailureCategoryOK {
			serial.CursorCredentialID = selection.CredentialID
			serial.CursorIdentityGeneration = selection.IdentityGeneration
			account.ResetAt, account.CooldownUntil, account.NextProbeAt = time.Time{}, time.Time{}, time.Time{}
			account.ProbeFailures, account.ProbeInFlight, account.ProbeVerified = 0, false, false
			account.ProbeUnsupported = false
			account.SuppressedQuotaResetAt = selection.QuotaResetAt
			return
		}
		if willRetry && decision.Retry == health.RetryRefreshCredential {
			return
		}
		account.ProbeInFlight = false
		if willRetry && serialSameAccountRetry(decision) {
			if account.ProbeFailures < 32 {
				account.ProbeFailures++
			}
			account.NextProbeAt = now.Add(serialProbeBackoff(account.ProbeFailures))
			retrySame = true
			return
		}
		account.ProbeVerified = false
		if decision.Category == health.FailureCategoryRateLimited &&
			decision.Scope == execution.ErrorScopeCredential {
			cooldownUntil := decision.CooldownUntil
			if cooldownUntil.IsZero() {
				cooldownUntil = now.Add(time.Minute)
			}
			resetAt := later(cooldownUntil, selection.QuotaResetAt)
			account.ResetAt, account.CooldownUntil = resetAt, cooldownUntil
		}
		if account.ProbeFailures < 32 {
			account.ProbeFailures++
		}
		minimumProbeAt := later(account.ResetAt, account.CooldownUntil).Add(serialFailbackGrace)
		backoffProbeAt := now.Add(serialProbeBackoff(account.ProbeFailures))
		account.NextProbeAt = later(minimumProbeAt, backoffProbeAt)
	})
	if retrySame {
		iterator.retrySameCredentialID = selection.CredentialID
		iterator.retrySameGroupID = selection.GroupID
	}
}
func serialSameAccountRetry(decision health.Decision) bool {
	return decision.Retry == health.RetryNextCandidate &&
		decision.Origin == execution.ErrorOriginUpstream &&
		(decision.Category == health.FailureCategoryUpstreamHostError ||
			decision.Category == health.FailureCategoryAmbiguous)
}
