package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"gorm.io/gorm"

	"gpt-load/internal/storage/models"
	providerobservation "gpt-load/internal/subscription/providers/observation"
)

// passiveQuotaFlushBatchSize bounds how many credentials one
// FlushPassiveQuotaObservations call processes, matching the "one bounded
// batch per worker cycle" contract shared with RequestLog's own checkpoints.
const passiveQuotaFlushBatchSize = 20

// FlushPassiveQuotaObservations persists up to one bounded batch of pending
// passive quota observations into credential_observations and reports
// whether work is ready for another wake-up.
// Unresolved history remains buffered but does not request an immediate wake;
// a later worker wake retries source lookup after its cooldown.
// Each credential is its own independent transaction-free conditional update: it
// only ever touches snapshot_json, observed_at_ms, and updated_at_ms. State,
// plan/account summaries, reset credits, and the most recent active
// attempt/error metadata are always left untouched. A credential with no
// existing observation row, a stale identity generation, or no matching window
// in the row is discarded without writing anything, since manual
// or automatic active observation remains the sole owner of window creation.
func (manager *CredentialManager) FlushPassiveQuotaObservations(ctx context.Context) (bool, error) {
	if manager == nil || manager.passiveQuota == nil || manager.db == nil {
		return false, nil
	}
	batch := manager.passiveQuota.dirtyObservations(passiveQuotaFlushBatchSize)
	var snapshotErr error
	for _, observation := range batch {
		if err := manager.flushOnePassiveQuotaObservation(ctx, observation); err != nil {
			snapshotErr = err
			break
		}
	}
	historyRemaining, err := manager.flushQuotaHistory(ctx)
	return len(manager.passiveQuota.dirtyObservations(1)) > 0 || historyRemaining, errors.Join(snapshotErr, err)
}

func (manager *CredentialManager) flushOnePassiveQuotaObservation(
	ctx context.Context,
	observation PassiveQuotaObservation,
) error {
	var err error
	// Identity validation, CAS retry, and quota projection must share a mutual-exclusion boundary with target switching
	// to prevent results persisted for an old target from continuing to write new-target runtime quota after a switch.
	manager.mutations.Do(observation.CredentialID, func() {
		err = manager.flushOnePassiveQuotaObservationLocked(ctx, observation)
	})
	return err
}

func (manager *CredentialManager) flushOnePassiveQuotaObservationLocked(
	ctx context.Context,
	observation PassiveQuotaObservation,
) error {
	if manager.registry == nil {
		return nil
	}
	ref, ok := manager.registry.CredentialRef(observation.CredentialID)
	if !ok || ref.IdentityGeneration != observation.IdentityGeneration {
		manager.passiveQuota.ack(observation.CredentialID, observation.Version)
		return nil
	}
	for attempt := 0; attempt < 2; attempt++ {
		var row models.CredentialObservation
		err := manager.db.WithContext(ctx).Take(&row, "credential_id = ?", observation.CredentialID).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				manager.passiveQuota.ack(observation.CredentialID, observation.Version)
				return nil
			}
			return fmt.Errorf("read credential observation %d: %w", observation.CredentialID, err)
		}
		if row.ObservedAtMS != nil && observation.ObservedAtMS <= *row.ObservedAtMS {
			// An observation at least as new -- typically a manual refresh --
			// was persisted while this sample sat pending. Writing it now would
			// rewind observed_at_ms and overwrite newer quota values with older
			// ones. The tie is included on purpose: a passive header captured
			// just before an active refresh that completed within the same
			// millisecond truncates to the same value, and the active result is
			// the authoritative one. The CAS below only catches writes that land
			// after this read.
			manager.passiveQuota.ack(observation.CredentialID, observation.Version)
			return nil
		}
		merge, observedAtMS, mergeErr := mergePassiveQuotaSamples(row.SnapshotJSON, row.ObservedAtMS, observation)
		if mergeErr != nil {
			// A snapshot this malformed cannot be repaired by retrying the
			// same merge; drop the observation instead of retrying forever.
			manager.passiveQuota.ack(observation.CredentialID, observation.Version)
			return nil
		}
		if !merge.Matched {
			// A sample that cannot uniquely match an existing window cannot advance synchronisation time.
			// Active observation remains responsible for window creation and identity changes.
			manager.passiveQuota.ack(observation.CredentialID, observation.Version)
			return nil
		}
		// A matched window whose values did not move is still evidence the
		// credential was observed just now, so the sync time advances even
		// when the snapshot itself stays byte-identical.
		updates := map[string]any{
			"observed_at_ms": observedAtMS,
			"updated_at_ms":  manager.now().UnixMilli(),
		}
		if merge.Changed {
			updates["snapshot_json"] = models.JSON(merge.Encoded)
		}
		// observation_version is the concurrency token: every successful active
		// observation increments it. A timestamp cannot serve here because two
		// writes can land in the same millisecond, leaving updated_at_ms
		// numerically unchanged and letting this update slip past. updated_at_ms
		// stays in the predicate to also catch the metadata-only writes from the
		// active failure and reset paths, which do not move the version.
		result := manager.db.WithContext(ctx).Model(&models.CredentialObservation{}).
			Where(
				"credential_id = ? AND observation_version = ? AND updated_at_ms = ?",
				observation.CredentialID, row.ObservationVersion, row.UpdatedAtMS,
			).
			Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("update credential observation %d: %w", observation.CredentialID, result.Error)
		}
		if result.RowsAffected == 1 {
			manager.passiveQuota.ack(observation.CredentialID, observation.Version)
			if row.State == models.CredentialObservationFresh {
				manager.registry.ApplyQuotaWindows(observation.CredentialID, merge.Windows)
			}
			return nil
		}
		// An active refresh or a reset committed between our read and our
		// write. Retry once against the row it left behind.
	}
	return fmt.Errorf("passive quota observation for credential %d conflicted with a concurrent write", observation.CredentialID)
}

// mergePassiveQuotaSamples processes handshakes and events by original time, then writes both through the same CAS.
// Compare each sample with the database time to prevent recording an old handshake as an event time or overwriting an active refresh in between.
func mergePassiveQuotaSamples(raw []byte, storedAtMS *int64, observation PassiveQuotaObservation) (passiveQuotaMerge, int64, error) {
	result := passiveQuotaMerge{Encoded: raw}
	var observedAtMS int64
	samples := [2]*PassiveQuotaSample{
		observation.Preceding,
		{ObservedAtMS: observation.ObservedAtMS, Windows: observation.Windows},
	}
	for _, sample := range samples {
		if sample == nil || (storedAtMS != nil && sample.ObservedAtMS <= *storedAtMS) {
			continue
		}
		merged, err := mergePassiveQuotaSnapshot(result.Encoded, sample.Windows)
		if err != nil {
			return passiveQuotaMerge{}, 0, err
		}
		if merged.Matched {
			observedAtMS = sample.ObservedAtMS
		}
		merged.Matched = merged.Matched || result.Matched
		merged.Changed = merged.Changed || result.Changed
		result = merged
	}
	return result, observedAtMS, nil
}

// passiveQuotaMerge is the outcome of overlaying one response's windows onto
// a stored snapshot. Matched and Changed are distinct on purpose: a response
// that names a tracked window but repeats its values is still a fresh
// observation of that credential, while a response naming no tracked window
// is no observation of this snapshot at all.
type passiveQuotaMerge struct {
	Encoded []byte
	Windows []providerobservation.QuotaWindow
	Matched bool
	Changed bool
}

// mergePassiveQuotaSnapshot updates only existing uniquely matched windows; it neither creates nor changes windows.
// Return original raw bytes unchanged when nothing changes; re-encode only when window data changes,
// retaining other field values without guaranteeing original key order or formatting.
func mergePassiveQuotaSnapshot(
	raw []byte,
	patches []providerobservation.QuotaWindow,
) (result passiveQuotaMerge, err error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return passiveQuotaMerge{}, fmt.Errorf("decode credential observation snapshot: %w", err)
	}
	windowsRaw, ok := fields["quota_windows"]
	if !ok {
		return passiveQuotaMerge{}, errors.New("credential observation snapshot has no quota_windows field")
	}
	var existing []providerobservation.QuotaWindow
	if err := json.Unmarshal(windowsRaw, &existing); err != nil {
		return passiveQuotaMerge{}, fmt.Errorf("decode credential observation quota windows: %w", err)
	}
	merged := append([]providerobservation.QuotaWindow(nil), existing...)
	positions := make([]int, len(patches))
	namedTargets := make(map[int]bool)
	matches := make(map[int]int, len(patches))
	for index, patch := range patches {
		position := matchPassiveQuotaWindow(existing, patch)
		positions[index] = position
		if position >= 0 && patch.SourceName != "" {
			namedTargets[position] = true
		}
	}
	for index, position := range positions {
		if position < 0 {
			continue
		}
		// Only WS carries SourceName. After resolving sources, a same-target top-level copy yields to a named window;
		// different sources or periods are not deduplicated, and numeric differences do not change that priority.
		if patches[index].SourceName == "" && namedTargets[position] {
			positions[index] = -1
			continue
		}
		matches[position]++
	}
	outcome := passiveQuotaMerge{Encoded: raw, Windows: existing}
	for index, patch := range patches {
		position := positions[index]
		// Multiple response windows pointing to the same target are also ambiguous; do not overwrite with the final value.
		if position < 0 || matches[position] != 1 {
			continue
		}
		previous := merged[position]
		// primary/secondary are upstream slots only; when periods conflict, do not merge usage, reset time, or state either.
		if previous.WindowSeconds != nil && patch.WindowSeconds != nil &&
			*previous.WindowSeconds != *patch.WindowSeconds {
			continue
		}
		outcome.Matched = true
		patch.ID = previous.ID // Response slots can change; card-window identity cannot.
		next := providerobservation.MergeQuotaWindow(previous, patch)
		if !reflect.DeepEqual(next, previous) {
			outcome.Changed = true
		}
		merged[position] = next
	}
	if !outcome.Changed {
		return outcome, nil
	}
	encodedWindows, err := json.Marshal(merged)
	if err != nil {
		return passiveQuotaMerge{}, fmt.Errorf("encode credential observation quota windows: %w", err)
	}
	fields["quota_windows"] = encodedWindows
	encoded, err := json.Marshal(fields)
	if err != nil {
		return passiveQuotaMerge{}, fmt.Errorf("encode credential observation snapshot: %w", err)
	}
	outcome.Encoded, outcome.Windows = encoded, merged
	return outcome, nil
}

// matchPassiveQuotaWindow aligns active and passive data by source and actual period; slots do not participate in inference.
// Other channels without SourceID retain ID matching but cannot overwrite windows carrying a source identifier.
func matchPassiveQuotaWindow(windows []providerobservation.QuotaWindow, patch providerobservation.QuotaWindow) int {
	if patch.SourceID == "" && patch.SourceName != "" {
		patch.SourceID = passiveQuotaSourceByName(windows, patch)
		if patch.SourceID == "" {
			return -1
		}
	}
	matched := -1
	for index, window := range windows {
		if patch.SourceID != "" {
			if window.SourceID != patch.SourceID || patch.WindowSeconds == nil ||
				*patch.WindowSeconds <= 0 || window.WindowSeconds == nil ||
				*window.WindowSeconds != *patch.WindowSeconds {
				continue
			}
		} else if window.SourceID != "" || window.ID != patch.ID {
			continue
		}
		if matched >= 0 {
			return -1
		}
		matched = index
	}
	return matched
}

// Additional Codex WS quota reports only raw limit_name. Resolve the source using names and periods saved by active observation,
// then use existing SourceID matching; do not use formatted Label or infer name aliases.
func passiveQuotaSourceByName(windows []providerobservation.QuotaWindow, patch providerobservation.QuotaWindow) string {
	if patch.WindowSeconds == nil || *patch.WindowSeconds <= 0 {
		return ""
	}
	sourceID := ""
	matched := false
	for _, window := range windows {
		if window.Scope == "account" || window.Scope != patch.SourceName ||
			window.WindowSeconds == nil || *window.WindowSeconds != *patch.WindowSeconds {
			continue
		}
		if matched {
			return ""
		}
		matched = true
		sourceID = window.SourceID
	}
	return sourceID
}
