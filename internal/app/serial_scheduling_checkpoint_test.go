package app

import (
	"context"
	"testing"
	"time"

	"gpt-load/internal/state"
)

func TestFileCheckpointRestoresSerialCursorResetAndProbeBackoff(t *testing.T) {
	makeRegistry := func() *state.CredentialRegistry {
		registry := state.NewCredentialRegistry()
		if err := registry.ReplaceCredentials([]state.CredentialEntry{
			{ID: 1, GroupID: 10, Version: 1, IdentityGeneration: 1, Fingerprint: "one", EncryptedValue: "cipher-one", Status: state.CredentialStatusActive},
			{ID: 2, GroupID: 10, Version: 1, IdentityGeneration: 1, Fingerprint: "two", EncryptedValue: "cipher-two", Status: state.CredentialStatusActive},
		}); err != nil {
			t.Fatal(err)
		}
		return registry
	}
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	original := makeRegistry()
	original.SchedulingState().WithLock(func(ledger *state.SchedulingLedger) {
		ledger.SerialGroups[10] = &state.SerialGroupState{
			CursorCredentialID: 2, CursorIdentityGeneration: 1,
			Accounts: map[uint]*state.SerialAccountState{
				1: {
					IdentityGeneration: 1,
					ResetAt:            now.Add(-time.Minute), CooldownUntil: now.Add(-time.Minute),
					NextProbeAt: now.Add(5 * time.Second), ProbeFailures: 3,
					ProbeInFlight: true, ProbeVerified: true,
				},
			},
		}
	})
	dataDir := t.TempDir()
	if err := NewFileRuntimeStateCheckpoint(dataDir, original, nil, nil).Save(context.Background()); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	loaded := makeRegistry()
	if err := NewFileRuntimeStateCheckpoint(dataDir, loaded, nil, nil).Restore(context.Background()); err != nil {
		t.Fatalf("restore checkpoint: %v", err)
	}
	checkpoint := loaded.SchedulingState().CaptureCheckpoint()
	if len(checkpoint.SerialGroups) != 1 || checkpoint.SerialGroups[0].CursorCredentialID != 2 ||
		checkpoint.SerialGroups[0].CursorIdentityGeneration != 1 || len(checkpoint.SerialGroups[0].Accounts) != 1 {
		t.Fatalf("restored serial cursor = %#v", checkpoint.SerialGroups)
	}
	account := checkpoint.SerialGroups[0].Accounts[0]
	if account.CredentialID != 1 || !account.ResetAt.Equal(now.Add(-time.Minute)) ||
		!account.CooldownUntil.Equal(now.Add(-time.Minute)) || !account.NextProbeAt.Equal(now.Add(5*time.Second)) ||
		account.ProbeFailures != 3 {
		t.Fatalf("restored serial account state = %#v", account)
	}
	var probeInFlight, probeVerified bool
	loaded.SchedulingState().WithLock(func(ledger *state.SchedulingLedger) {
		accountState := ledger.SerialGroups[10].Accounts[1]
		probeInFlight, probeVerified = accountState.ProbeInFlight, accountState.ProbeVerified
	})
	if probeInFlight || probeVerified {
		t.Fatalf("restart retained a speculative in-flight probe: in_flight=%t verified=%t", probeInFlight, probeVerified)
	}
}
