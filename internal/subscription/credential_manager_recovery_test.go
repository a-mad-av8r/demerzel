package subscription

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"gpt-load/internal/channel"
	"gpt-load/internal/execution"
	"gpt-load/internal/state"
	"gpt-load/internal/storage/models"
	"gpt-load/internal/subscription/providers/codex"
)

func TestCredentialManagerManualRecoveryRestoresReadyFromOutcomeUnknown(t *testing.T) {
	manager, db, registry, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
	markCredentialAuthState(t, db, registry, row.ID, models.CredentialAuthStateOutcomeUnknown, "refresh_outcome_unknown")
	refreshCalls := 0
	manager.refresh = adaptCodexRefresh(func(_ context.Context, current codex.Credential) (codex.Credential, error) {
		refreshCalls++
		current.AccessToken = "new-access"
		current.Expire = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
		return current, nil
	})

	credential, evidence := manager.RefreshForManualRecovery(t.Context(), channel.Codex, credentialSnapshot(t, row, keyService))
	if evidence != nil || refreshCalls != 1 || mustCodexCredential(t, credential).AccessToken != "new-access" {
		t.Fatalf("credential=%#v evidence=%#v refresh=%d", credential, evidence, refreshCalls)
	}
	assertStoredAuthState(t, db, row.ID, models.CredentialAuthStateReady, "")
	entry, ok := registry.CredentialRef(row.ID)
	if !ok || entry.Version != 2 {
		t.Fatalf("registry entry = %#v ok=%t", entry, ok)
	}
}

func TestCredentialManagerManualRecoveryRestoresReadyFromReauthorizationRequired(t *testing.T) {
	manager, db, registry, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
	markCredentialAuthState(t, db, registry, row.ID, models.CredentialAuthStateReauthorizationRequired, "refresh_rejected")
	manager.refresh = adaptCodexRefresh(refreshedCredential)

	_, evidence := manager.RefreshForManualRecovery(t.Context(), channel.Codex, credentialSnapshot(t, row, keyService))
	if evidence != nil {
		t.Fatalf("evidence = %#v", evidence)
	}
	assertStoredAuthState(t, db, row.ID, models.CredentialAuthStateReady, "")
}

func TestCredentialManagerManualRecoveryRetryableFailureKeepsOriginalState(t *testing.T) {
	manager, db, registry, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
	markCredentialAuthState(t, db, registry, row.ID, models.CredentialAuthStateOutcomeUnknown, "refresh_outcome_unknown")
	manager.refresh = adaptCodexRefresh(func(context.Context, codex.Credential) (codex.Credential, error) {
		return codex.Credential{}, &codex.TokenEndpointError{
			StatusCode: http.StatusServiceUnavailable, Code: "server_error",
		}
	})

	_, evidence := manager.RefreshForManualRecovery(t.Context(), channel.Codex, credentialSnapshot(t, row, keyService))
	if evidence == nil || evidence.Code != "refresh_temporarily_unavailable" {
		t.Fatalf("evidence = %#v", evidence)
	}
	assertStoredAuthState(t, db, row.ID, models.CredentialAuthStateOutcomeUnknown, "refresh_outcome_unknown")
}

func TestCredentialManagerManualRecoveryStartFailureKeepsOriginalState(t *testing.T) {
	manager, db, registry, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
	markCredentialAuthState(t, db, registry, row.ID, models.CredentialAuthStateReauthorizationRequired, "refresh_rejected")
	registry.RemoveCredential(row.ID)
	manager.reconcileGroup = func(uint, []state.CredentialEntry) (bool, error) {
		return false, errors.New("registry unavailable")
	}
	refreshCalls := 0
	manager.refresh = adaptCodexRefresh(func(context.Context, codex.Credential) (codex.Credential, error) {
		refreshCalls++
		return codex.Credential{}, errors.New("must not be called")
	})

	_, evidence := manager.RefreshForManualRecovery(t.Context(), channel.Codex, credentialSnapshot(t, row, keyService))
	if evidence == nil || evidence.Code != "refresh_registry_mismatch" || refreshCalls != 0 {
		t.Fatalf("evidence = %#v refresh = %d", evidence, refreshCalls)
	}
	assertStoredAuthState(t, db, row.ID, models.CredentialAuthStateReauthorizationRequired, "refresh_rejected")
}

func TestCredentialManagerManualRecoveryDefinitiveRejectionOverridesOriginalState(t *testing.T) {
	manager, db, registry, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
	markCredentialAuthState(t, db, registry, row.ID, models.CredentialAuthStateOutcomeUnknown, "refresh_outcome_unknown")
	manager.refresh = adaptCodexRefresh(func(context.Context, codex.Credential) (codex.Credential, error) {
		return codex.Credential{}, &codex.TokenEndpointError{StatusCode: http.StatusBadRequest, Code: "invalid_grant"}
	})

	_, evidence := manager.RefreshForManualRecovery(t.Context(), channel.Codex, credentialSnapshot(t, row, keyService))
	if evidence == nil || evidence.Code != "refresh_rejected" {
		t.Fatalf("evidence = %#v", evidence)
	}
	assertStoredAuthState(t, db, row.ID, models.CredentialAuthStateReauthorizationRequired, "refresh_rejected")
}

// “Retain the original state” does not apply once the upstream returns a new credential: retain it in the database and mark the outcome unknown.
func TestCredentialManagerManualRecoveryKeepsRotatedSecretWhenRegistryPublicationFails(t *testing.T) {
	manager, db, registry, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
	markCredentialAuthState(t, db, registry, row.ID, models.CredentialAuthStateOutcomeUnknown, "refresh_outcome_unknown")
	manager.refresh = adaptCodexRefresh(refreshedCredential)
	manager.replaceSecret = func(uint, uint64, uint64, string, string) bool { return false }
	manager.reconcileGroup = func(uint, []state.CredentialEntry) (bool, error) {
		return false, errors.New("registry unavailable")
	}

	_, evidence := manager.RefreshForManualRecovery(t.Context(), channel.Codex, credentialSnapshot(t, row, keyService))
	if evidence == nil || evidence.Code != "refresh_registry_mismatch" {
		t.Fatalf("evidence = %#v", evidence)
	}
	var stored models.Credential
	if err := db.First(&stored, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SecretVersion != 2 || stored.AuthState != models.CredentialAuthStateOutcomeUnknown ||
		stored.AuthErrorCode != "refresh_registry_mismatch" {
		t.Fatalf("stored credential = %#v", stored)
	}
}

func TestCredentialManagerRejectsRecoveryOutsideManualEntry(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		prepare func(*CredentialManager, execution.CredentialSnapshot) *execution.ErrorEvidence
	}{
		{
			name: "data plane",
			prepare: func(manager *CredentialManager, snapshot execution.CredentialSnapshot) *execution.ErrorEvidence {
				_, evidence := manager.Prepare(context.Background(), channel.Codex, snapshot, true)
				return evidence
			},
		},
		{
			name: "control plane",
			prepare: func(manager *CredentialManager, snapshot execution.CredentialSnapshot) *execution.ErrorEvidence {
				_, evidence := manager.PrepareForControl(context.Background(), channel.Codex, snapshot, true)
				return evidence
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			manager, db, registry, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
			markCredentialAuthState(t, db, registry, row.ID, models.CredentialAuthStateOutcomeUnknown, "refresh_outcome_unknown")
			refreshCalls := 0
			manager.refresh = adaptCodexRefresh(func(context.Context, codex.Credential) (codex.Credential, error) {
				refreshCalls++
				return codex.Credential{}, errors.New("must not be called")
			})

			evidence := testCase.prepare(manager, credentialSnapshot(t, row, keyService))
			if evidence == nil || evidence.Code != string(models.CredentialAuthStateOutcomeUnknown) || refreshCalls != 0 {
				t.Fatalf("evidence = %#v refresh = %d", evidence, refreshCalls)
			}
			assertStoredAuthState(t, db, row.ID, models.CredentialAuthStateOutcomeUnknown, "refresh_outcome_unknown")
		})
	}
}

// Residual refreshing is transient and cannot be written back as the original state, or the UI would display refreshing forever.
func TestCredentialManagerManualRecoveryNormalizesResidualRefreshing(t *testing.T) {
	manager, db, registry, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
	markCredentialAuthState(t, db, registry, row.ID, models.CredentialAuthStateRefreshing, "")
	manager.refresh = adaptCodexRefresh(func(context.Context, codex.Credential) (codex.Credential, error) {
		return codex.Credential{}, &codex.TokenEndpointError{
			StatusCode: http.StatusServiceUnavailable, Code: "server_error",
		}
	})

	_, evidence := manager.RefreshForManualRecovery(t.Context(), channel.Codex, credentialSnapshot(t, row, keyService))
	if evidence == nil || evidence.Code != "refresh_temporarily_unavailable" {
		t.Fatalf("evidence = %#v", evidence)
	}
	assertStoredAuthState(t, db, row.ID, models.CredentialAuthStateOutcomeUnknown, "refresh_interrupted")
}

// A higher database version does not mean authentication recovered: rebuild first when runtime lags rather than reporting success.
func TestCredentialManagerManualRecoveryRejectsInconsistentRuntimeVersion(t *testing.T) {
	manager, db, _, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
	if err := db.Model(&models.Credential{}).Where("id = ?", row.ID).
		Update("secret_version", row.SecretVersion+1).Error; err != nil {
		t.Fatal(err)
	}
	manager.reconcileGroup = func(uint, []state.CredentialEntry) (bool, error) {
		return false, errors.New("registry unavailable")
	}
	refreshCalls := 0
	manager.refresh = adaptCodexRefresh(func(context.Context, codex.Credential) (codex.Credential, error) {
		refreshCalls++
		return codex.Credential{}, errors.New("must not be called")
	})

	_, evidence := manager.RefreshForManualRecovery(t.Context(), channel.Codex, credentialSnapshot(t, row, keyService))
	if evidence == nil || evidence.Code != "refresh_registry_mismatch" || refreshCalls != 0 {
		t.Fatalf("evidence = %#v refresh = %d", evidence, refreshCalls)
	}
}

// Manual recovery guarantees serialisation only, not merging concurrent requests: a later request still reaches upstream after the first fails.
func TestCredentialManagerManualRecoverySerializesWithoutMergingRequests(t *testing.T) {
	manager, db, registry, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
	markCredentialAuthState(t, db, registry, row.ID, models.CredentialAuthStateOutcomeUnknown, "refresh_outcome_unknown")
	var mu sync.Mutex
	refreshCalls := 0
	manager.refresh = adaptCodexRefresh(func(context.Context, codex.Credential) (codex.Credential, error) {
		mu.Lock()
		refreshCalls++
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		return codex.Credential{}, &codex.TokenEndpointError{
			StatusCode: http.StatusServiceUnavailable, Code: "server_error",
		}
	})
	snapshot := credentialSnapshot(t, row, keyService)
	var wait sync.WaitGroup
	wait.Add(2)
	for range 2 {
		go func() {
			defer wait.Done()
			manager.RefreshForManualRecovery(context.Background(), channel.Codex, snapshot)
		}()
	}
	wait.Wait()

	mu.Lock()
	calls := refreshCalls
	mu.Unlock()
	if calls != 2 {
		t.Fatalf("refresh calls = %d, want 2", calls)
	}
	assertStoredAuthState(t, db, row.ID, models.CredentialAuthStateOutcomeUnknown, "refresh_outcome_unknown")
}

// ReconcileGroup compares only persisted configuration. It deems a current runtime version with a lagging authentication state unchanged,
// so repair it selectively and confirm again before reusing the new version.
func TestCredentialManagerManualRecoveryRepairsRuntimeAuthStateAtSameVersion(t *testing.T) {
	manager, db, registry, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
	snapshot := credentialSnapshot(t, row, keyService)
	if !registry.ReplaceCredentialSecretIfMatch(row.ID, row.SecretVersion, row.SecretVersion+1, row.Fingerprint, row.Data) {
		t.Fatal("seed runtime secret version")
	}
	if !registry.SetCredentialAuthState(row.ID, state.CredentialAuthStateOutcomeUnknown) {
		t.Fatal("seed runtime auth state")
	}
	if err := db.Model(&models.Credential{}).Where("id = ?", row.ID).
		Update("secret_version", row.SecretVersion+1).Error; err != nil {
		t.Fatal(err)
	}
	refreshCalls := 0
	manager.refresh = adaptCodexRefresh(func(context.Context, codex.Credential) (codex.Credential, error) {
		refreshCalls++
		return codex.Credential{}, errors.New("must not be called")
	})

	_, evidence := manager.RefreshForManualRecovery(t.Context(), channel.Codex, snapshot)
	if evidence != nil || refreshCalls != 0 {
		t.Fatalf("evidence = %#v refresh = %d", evidence, refreshCalls)
	}
	authState, known := registry.CredentialAuthStateOf(row.ID)
	if !known || authState != state.CredentialAuthStateReady {
		t.Fatalf("runtime auth state = %q known=%t", authState, known)
	}
}

// Account state and this operation's outcome must remain separate: retain historical authentication problems but report this failure.
func TestCredentialManagerManualRecoveryReportsCurrentFailureNotStoredCode(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		authState models.CredentialAuthState
		errorCode string
		wantState models.CredentialAuthState
		wantCode  string
	}{
		{
			name:      "reauthorization required",
			authState: models.CredentialAuthStateReauthorizationRequired,
			errorCode: "refresh_rejected",
			wantState: models.CredentialAuthStateReauthorizationRequired,
			wantCode:  "refresh_rejected",
		},
		{
			name:      "residual refreshing",
			authState: models.CredentialAuthStateRefreshing,
			errorCode: "",
			wantState: models.CredentialAuthStateOutcomeUnknown,
			wantCode:  "refresh_interrupted",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			manager, db, registry, keyService, row := newCredentialManagerFixture(t, credentialJSON("old-access", "old-refresh", time.Now().Add(time.Hour)))
			markCredentialAuthState(t, db, registry, row.ID, testCase.authState, testCase.errorCode)
			manager.refresh = adaptCodexRefresh(func(context.Context, codex.Credential) (codex.Credential, error) {
				return codex.Credential{}, errors.New("dial tcp: connection refused")
			})

			_, evidence := manager.RefreshForManualRecovery(t.Context(), channel.Codex, credentialSnapshot(t, row, keyService))
			if evidence == nil || evidence.Code != "refresh_outcome_unknown" {
				t.Fatalf("evidence = %#v, want this attempt reported as outcome unknown", evidence)
			}
			assertStoredAuthState(t, db, row.ID, testCase.wantState, testCase.wantCode)
		})
	}
}

func markCredentialAuthState(
	t *testing.T,
	db *gorm.DB,
	registry *state.CredentialRegistry,
	credentialID uint,
	authState models.CredentialAuthState,
	code string,
) {
	t.Helper()
	if err := db.Model(&models.Credential{}).Where("id = ?", credentialID).
		Updates(map[string]any{"auth_state": authState, "auth_error_code": code}).Error; err != nil {
		t.Fatal(err)
	}
	if !registry.SetCredentialAuthState(credentialID, state.CredentialAuthState(authState)) {
		t.Fatalf("publish auth state %q", authState)
	}
}
