package scheduler

import (
	"encoding/json"
	"testing"
	"time"

	"gpt-load/internal/execution"
	"gpt-load/internal/health"
	"gpt-load/internal/protocol"
	"gpt-load/internal/state"
)

func serialSchedulerFixture(t *testing.T) (*state.ConfigSnapshot, *state.CredentialRegistry, time.Time) {
	t.Helper()
	snapshot := schedulerSnapshot()
	group := snapshot.Groups[1]
	group.AccountSelection = state.AccountSelectionSerial
	group.SerialQuotaReservePercent = 10
	snapshot.Groups[1] = group
	registry := state.NewCredentialRegistry()
	if err := registry.ReplaceCredentials([]state.CredentialEntry{
		{ID: 11, GroupID: 1, Version: 1, IdentityGeneration: 1, Status: state.CredentialStatusActive, Fingerprint: "one", EncryptedValue: "cipher-one"},
		{ID: 12, GroupID: 1, Version: 1, IdentityGeneration: 1, Status: state.CredentialStatusActive, Fingerprint: "two", EncryptedValue: "cipher-two"},
	}); err != nil {
		t.Fatal(err)
	}
	return snapshot, registry, time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
}

func serialSchedulerQuery() Query {
	return Query{
		ClientProtocol: protocol.OpenAICompletions,
		Operation: execution.OperationChatCompletion,
		ExternalModel: modelPointer("gpt-4o"),
	}
}

func serialPickAt(snapshot *state.ConfigSnapshot, registry *state.CredentialRegistry, now time.Time) (Selection, *Iterator, error) {
	iterator := newWithClock(snapshot, registry, serialSchedulerQuery(), func() time.Time { return now })
	selection, err := iterator.Next()
	return selection, iterator, err
}

func TestSerialFailoverAdvancesOnlyOnAccountRateLimitAndRestoresCursor(t *testing.T) {
	snapshot, registry, now := serialSchedulerFixture(t)
	first, iterator, err := serialPickAt(snapshot, registry, now)
	if err != nil || first.CredentialID != 11 {
		t.Fatalf("initial serial selection = %#v, %v", first, err)
	}
	iterator.RecordAttempt(first, health.Decision{
		Category: health.FailureCategoryRateLimited,
		Scope: execution.ErrorScopeCredential,
		Retry: health.RetryNextCandidate,
		CooldownUntil: now.Add(time.Hour),
	}, now, true)
	second, _, err := serialPickAt(snapshot, registry, now)
	if err != nil || second.CredentialID != 12 {
		t.Fatalf("account-scoped 429 selection = %#v, %v", second, err)
	}

	checkpoint := registry.SchedulingState().CaptureCheckpoint()
	encoded, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	var restoredCheckpoint state.SchedulingCheckpoint
	if err := json.Unmarshal(encoded, &restoredCheckpoint); err != nil {
		t.Fatal(err)
	}
	_, restarted, _ := serialSchedulerFixture(t)
	restarted.SchedulingState().SyncGroups(snapshot)
	if restarted.SchedulingState().RestoreCheckpoint(restoredCheckpoint) != 2 {
		t.Fatal("restart did not restore the matching scheduling identities")
	}
	afterRestart, _, err := serialPickAt(snapshot, restarted, now)
	if err != nil || afterRestart.CredentialID != 12 {
		t.Fatalf("post-restart cursor = %#v, %v", afterRestart, err)
	}
}

func TestSerialTransientReplayKeepsTheSameAccount(t *testing.T) {
	snapshot, registry, now := serialSchedulerFixture(t)
	first, iterator, err := serialPickAt(snapshot, registry, now)
	if err != nil || first.CredentialID != 11 {
		t.Fatalf("initial serial selection = %#v, %v", first, err)
	}
	iterator.RecordAttempt(first, health.Decision{
		Category: health.FailureCategoryUpstreamHostError,
		Origin: execution.ErrorOriginUpstream,
		Scope: execution.ErrorScopeGroup,
		Retry: health.RetryNextCandidate,
		Effect: health.EffectSkipGroup,
	}, now, true)
	iterator.SkipGroup(first.GroupID)
	second, err := iterator.Next()
	if err != nil || second.CredentialID != first.CredentialID {
		t.Fatalf("replay selection = %#v, %v; want the same account", second, err)
	}
}

func TestSerialFailbackUsesSerializedBackoffProbes(t *testing.T) {
	snapshot, registry, now := serialSchedulerFixture(t)
	first, iterator, err := serialPickAt(snapshot, registry, now)
	if err != nil {
		t.Fatal(err)
	}
	iterator.RecordAttempt(first, health.Decision{
		Category: health.FailureCategoryRateLimited,
		Scope: execution.ErrorScopeCredential,
		Retry: health.RetryNextCandidate,
		CooldownUntil: now.Add(time.Minute),
	}, now, true)
	fallback, _, err := serialPickAt(snapshot, registry, now)
	if err != nil || fallback.CredentialID != 12 {
		t.Fatalf("fallback selection = %#v, %v", fallback, err)
	}

	probeNow := now.Add(2 * time.Minute)
	probe, _, err := serialPickAt(snapshot, registry, probeNow)
	if err != nil || probe.CredentialID != 11 || !probe.SerialProbe {
		t.Fatalf("first failback probe = %#v, %v", probe, err)
	}
	concurrent, _, err := serialPickAt(snapshot, registry, probeNow)
	if err != nil || concurrent.CredentialID != 12 || concurrent.SerialProbe {
		t.Fatalf("parallel request bypassed the serialized probe: %#v, %v", concurrent, err)
	}
	probeIterator := newWithClock(snapshot, registry, serialSchedulerQuery(), func() time.Time { return probeNow })
	probeIterator.RecordSerialProbe(probe, SerialProbeFailed, probeNow)
	beforeBackoff, _, err := serialPickAt(snapshot, registry, probeNow.Add(4*time.Second))
	if err != nil || beforeBackoff.CredentialID != 12 || beforeBackoff.SerialProbe {
		t.Fatalf("probe retried before its backoff: %#v, %v", beforeBackoff, err)
	}
	secondProbe, _, err := serialPickAt(snapshot, registry, probeNow.Add(5*time.Second))
	if err != nil || secondProbe.CredentialID != 11 || !secondProbe.SerialProbe {
		t.Fatalf("second failback probe = %#v, %v", secondProbe, err)
	}
	secondProbeIterator := newWithClock(
		snapshot, registry, serialSchedulerQuery(), func() time.Time { return probeNow.Add(5 * time.Second) },
	)
	secondProbeIterator.RecordSerialProbe(secondProbe, SerialProbeSucceeded, probeNow.Add(5*time.Second))
	duringInference, _, err := serialPickAt(snapshot, registry, probeNow.Add(5*time.Second))
	if err != nil || duringInference.CredentialID != 12 || duringInference.SerialProbe {
		t.Fatalf("safe GET promoted the primary before inference success: %#v, %v", duringInference, err)
	}
	inference := secondProbe
	inference.SerialProbe, inference.SerialProbePending = false, true
	secondProbeIterator.RecordAttempt(
		inference, health.Decision{Category: health.FailureCategoryOK}, probeNow.Add(5*time.Second), false,
	)
	active, _, err := serialPickAt(snapshot, registry, probeNow.Add(5*time.Second))
	if err != nil || active.CredentialID != 11 || active.SerialProbe {
		t.Fatalf("successful caller inference did not fail back: %#v, %v", active, err)
	}
}

func TestSerialFailbackWithNoAlternateStaysClosedAndSerializesProbe(t *testing.T) {
	snapshot, registry, now := serialSchedulerFixture(t)
	if !registry.RemoveCredential(12) {
		t.Fatal("RemoveCredential(12) = false")
	}
	first, iterator, err := serialPickAt(snapshot, registry, now)
	if err != nil || first.CredentialID != 11 {
		t.Fatalf("initial selection = %#v, %v", first, err)
	}
	iterator.RecordAttempt(first, health.Decision{
		Category: health.FailureCategoryRateLimited,
		Scope: execution.ErrorScopeCredential,
		CooldownUntil: now.Add(time.Minute),
	}, now, false)
	if _, _, err := serialPickAt(snapshot, registry, now); err != ErrExhausted {
		t.Fatalf("blocked single account selection error = %v, want ErrExhausted", err)
	}

	probeNow := now.Add(2 * time.Minute)
	probe, _, err := serialPickAt(snapshot, registry, probeNow)
	if err != nil || probe.CredentialID != 11 || !probe.SerialProbe {
		t.Fatalf("single-account failback probe = %#v, %v", probe, err)
	}
	probeIterator := newWithClock(snapshot, registry, serialSchedulerQuery(), func() time.Time { return probeNow })
	probeIterator.RecordSerialProbe(probe, SerialProbeSucceeded, probeNow)
	if _, _, err := serialPickAt(snapshot, registry, probeNow); err != ErrExhausted {
		t.Fatalf("inference was admitted while the serialized probe was pending: %v", err)
	}
	inference := probe
	inference.SerialProbe, inference.SerialProbePending = false, true
	probeIterator.RecordAttempt(inference, health.Decision{
		Category: health.FailureCategoryRateLimited,
		Scope: execution.ErrorScopeCredential,
		CooldownUntil: probeNow.Add(time.Hour),
	}, probeNow, true)
	if _, _, err := serialPickAt(snapshot, registry, probeNow); err != ErrExhausted {
		t.Fatalf("single-account pool opened after a failed inference: %v", err)
	}
}

func TestSerialQuotaReserveUsesOnlyExplicitProviderObservation(t *testing.T) {
	snapshot, registry, now := serialSchedulerFixture(t)
	remaining := 0.05
	resetAt := now.Add(time.Hour)
	if !registry.SetCredentialQuotaObservation(11, &remaining, resetAt) {
		t.Fatal("provider quota observation was rejected")
	}
	selection, _, err := serialPickAt(snapshot, registry, now)
	if err != nil || selection.CredentialID != 12 {
		t.Fatalf("quota-reserve selection = %#v, %v", selection, err)
	}
	views := registry.SchedulingState().CaptureCheckpoint().SerialGroups
	if len(views) != 1 || len(views[0].Accounts) != 1 || views[0].Accounts[0].CredentialID != 11 ||
		!views[0].Accounts[0].ResetAt.Equal(resetAt) {
		t.Fatalf("quota reserve state = %#v", views)
	}
}

func TestSerialRequest429AndPermanentDenialDoNotAdvanceCursor(t *testing.T) {
	for _, test := range []struct {
		name     string
		decision health.Decision
	}{
		{
			name: "request-scoped 429",
			decision: health.Decision{
				Category: health.FailureCategoryRateLimited,
				Scope: execution.ErrorScopeRequest,
			},
		},
		{
			name: "permanent credential denial",
			decision: health.Decision{
				Category: health.FailureCategoryInvalidKey,
				Scope: execution.ErrorScopeCredential,
				Retry: health.RetryNextCandidate,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, registry, now := serialSchedulerFixture(t)
			first, iterator, err := serialPickAt(snapshot, registry, now)
			if err != nil || first.CredentialID != 11 {
				t.Fatalf("initial selection = %#v, %v", first, err)
			}
			iterator.RecordAttempt(first, test.decision, now, test.decision.Retry != health.RetryNone)
			next, _, err := serialPickAt(snapshot, registry, now)
			if err != nil || next.CredentialID != first.CredentialID {
				t.Fatalf("non-account limit advanced serial cursor: %#v, %v", next, err)
			}
		})
	}
}
