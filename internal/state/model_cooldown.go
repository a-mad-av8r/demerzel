package state

import (
	"strings"
	"time"
)

// SetModelCooldown accepts results only for the current target and recovery generation; concurrency limits may only extend the expiry.
func (r *CredentialRegistry) SetModelCooldown(ref CredentialRef, model string, until, now time.Time) (bool, bool) {
	if model == "" || strings.TrimSpace(model) != model || !until.After(now) {
		return false, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entryLocked(ref.ID)
	if !ok || entry.GroupID != ref.GroupID || entry.IdentityGeneration != ref.IdentityGeneration ||
		entry.ModelCooldownGeneration != ref.ModelCooldownGeneration {
		return false, false
	}
	pruneModelCooldowns(entry.ModelCooldowns, now)
	if !until.After(entry.ModelCooldowns[model]) {
		return true, false
	}
	if entry.ModelCooldowns == nil {
		entry.ModelCooldowns = make(map[string]time.Time)
	}
	entry.ModelCooldowns[model] = until
	return true, true
}

func (r *CredentialRegistry) ModelCooldowns(credentialID uint, now time.Time) map[string]time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entryLocked(credentialID)
	if !ok {
		return nil
	}
	pruneModelCooldowns(entry.ModelCooldowns, now)
	return cloneModelCooldowns(entry.ModelCooldowns)
}

// ClearModelCooldowns is explicit recovery; ordinary success, token refresh, and quota synchronisation must not call it.
func (r *CredentialRegistry) ClearModelCooldowns(credentialID uint) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entryLocked(credentialID)
	if !ok {
		return false
	}
	entry.ModelCooldowns = nil
	entry.ModelCooldownGeneration++
	return true
}

// ExpireModelCooldowns reuses existing runtime-state maintenance timing and adds no scheduled task.
func (r *CredentialRegistry) ExpireModelCooldowns(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, bucket := range r.buckets {
		for _, entry := range bucket {
			pruneModelCooldowns(entry.ModelCooldowns, now)
		}
	}
}

func pruneModelCooldowns(limits map[string]time.Time, now time.Time) {
	for model, until := range limits {
		if model == "" || !until.After(now) {
			delete(limits, model)
		}
	}
}

func cloneModelCooldowns(limits map[string]time.Time) map[string]time.Time {
	if len(limits) == 0 {
		return nil
	}
	cloned := make(map[string]time.Time, len(limits))
	for model, until := range limits {
		cloned[model] = until
	}
	return cloned
}

func preserveModelCooldowns(next *CredentialEntry, previous *CredentialEntry) {
	if previous == nil {
		return
	}
	next.ModelCooldownGeneration = previous.ModelCooldownGeneration
	if next.ID == previous.ID && next.GroupID == previous.GroupID && next.IdentityGeneration == previous.IdentityGeneration {
		next.ModelCooldowns = cloneModelCooldowns(previous.ModelCooldowns)
	} else {
		next.ModelCooldowns = nil
		next.ModelCooldownGeneration++
	}
}
