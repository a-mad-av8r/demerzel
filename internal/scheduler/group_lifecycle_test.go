package scheduler

import (
	"testing"

	"gpt-load/internal/channel"
	"gpt-load/internal/state"
)

func TestFairnessGroupModelLifecycle(t *testing.T) {
	for _, scenario := range []string{
		"initially empty", "models cleared", "credentials reloaded", "old request",
		"checkpoint while empty", "checkpoint before models removed", "other model",
	} {
		t.Run(scenario, func(t *testing.T) {
			registry := state.NewCredentialRegistry()
			manager := state.NewManager()
			manager.SetSchedulingState(registry.SchedulingState())
			publish := func(models []state.ModelConfig) *state.ConfigSnapshot {
				t.Helper()
				snapshot, err := manager.Publish(state.CompileInput{ChannelRegistry: channel.NewRegistry(), Groups: []state.GroupConfig{
					{ID: 1, Name: "first", ChannelID: channel.OpenAI, ConnectionType: "api_key", Enabled: true,
						Models: []state.ModelConfig{{ID: "gpt-4o"}}},
					{ID: 2, Name: "second", ChannelID: channel.OpenAI, ConnectionType: "api_key", Enabled: true, Models: models},
				}})
				if err != nil {
					t.Fatal(err)
				}
				return snapshot
			}
			models := []state.ModelConfig{{ID: "gpt-4o"}}
			initialModels := models
			if scenario == "initially empty" {
				initialModels = nil
			}
			// Preserve real startup order: configuration publication precedes credential loading.
			snapshot := publish(initialModels)
			entries := []state.CredentialEntry{
				{ID: 11, GroupID: 1, Version: 1, IdentityGeneration: 1, Status: state.CredentialStatusActive,
					Fingerprint: "fixture-one", EncryptedValue: "cipher"},
				{ID: 12, GroupID: 2, Version: 1, IdentityGeneration: 1, Status: state.CredentialStatusActive,
					Fingerprint: "fixture-two", EncryptedValue: "cipher"},
			}
			if err := registry.ReplaceCredentials(entries); err != nil {
				t.Fatal(err)
			}
			for range 1000 {
				fairnessPick(t, snapshot, registry, 11)
			}
			beforeChange := snapshot
			checkpointBeforeChange := registry.SchedulingState().CaptureCheckpoint()
			if scenario == "other model" {
				snapshot = publish([]state.ModelConfig{{ID: "gpt-4o-mini"}})
			} else if scenario != "initially empty" {
				snapshot = publish(nil)
			}

			switch scenario {
			case "credentials reloaded":
				entries[1].Version++
				if _, err := registry.ReconcileGroup(2, entries[1:]); err != nil {
					t.Fatal(err)
				}
			case "old request":
				// An old snapshot cannot revert global group state; a request that already captured routing may still continue.
				old := New(beforeChange, registry, fairnessQuery(12))
				selected, err := old.Next()
				if err != nil || selected.CredentialID != 12 {
					t.Fatalf("frozen request selected %d, error=%v", selected.CredentialID, err)
				}
				if got := fairnessPick(t, snapshot, registry, 12); got != 11 {
					t.Fatalf("new request selected unavailable group credential %d", got)
				}
			case "checkpoint while empty", "checkpoint before models removed":
				saved := registry.SchedulingState().CaptureCheckpoint()
				if scenario == "checkpoint before models removed" {
					saved = checkpointBeforeChange
				}
				registry = state.NewCredentialRegistry()
				manager = state.NewManager()
				manager.SetSchedulingState(registry.SchedulingState())
				snapshot = publish(nil)
				if err := registry.ReplaceCredentials(entries); err != nil {
					t.Fatal(err)
				}
				if got := registry.SchedulingState().RestoreCheckpoint(saved); got != 2 {
					t.Fatalf("restored %d credentials, want 2", got)
				}
			}
			// Continue generating traffic while unavailable to verify recovery calibration is neither synchronised nor cleared early by old requests.
			for range 1000 {
				fairnessPick(t, snapshot, registry, 0)
			}
			snapshot = publish(models)
			counts := map[uint]int{}
			for range 101 {
				counts[fairnessPick(t, snapshot, registry, 0)]++
			}
			want := 51
			if scenario == "other model" {
				// The group can still serve other models; only this route is absent, so retain existing accumulated compensation.
				want = 100
			}
			if counts[12] != want || counts[11] != 101-want {
				t.Fatalf("distribution after model restoration = %v, want credential 12 to receive %d", counts, want)
			}
		})
	}
}
