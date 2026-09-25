package scheduler

import (
	"time"

	"gpt-load/internal/execution"
	"gpt-load/internal/state"
)

func modelCooldownUntil(limits map[string]time.Time, model string, operation execution.Operation, now time.Time) time.Time {
	if !operation.UsesModelCooldown() || model == "" || !limits[model].After(now) {
		return time.Time{}
	}
	return limits[model]
}

// CooldownUntil interprets dynamic cooldown exhaustion only before an attempt starts; it does not change the priority of actual failures.
func (iterator *Iterator) CooldownUntil() (time.Time, bool) {
	source, ok := iterator.credentials.(interface {
		Snapshot() []state.CredentialRuntimeView
	})
	if !ok {
		return time.Time{}, false
	}
	inspection, err := Inspect(iterator.snapshot, source.Snapshot(), iterator.query, iterator.now())
	if err != nil || inspection.Routable {
		return time.Time{}, false
	}
	var earliest time.Time
	for _, group := range inspection.Groups {
		if !group.Included {
			continue
		}
		for _, credential := range group.Credentials {
			if credential.Reason != ReasonCredentialCooldown && credential.Reason != ReasonModelCooldown {
				continue
			}
			if earliest.IsZero() || credential.CooldownUntil.Before(earliest) {
				earliest = credential.CooldownUntil
			}
		}
	}
	return earliest, !earliest.IsZero()
}
