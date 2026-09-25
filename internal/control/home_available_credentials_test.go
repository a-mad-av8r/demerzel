package control

import (
	"context"
	"testing"
	"time"

	"gpt-load/internal/state"
	stateloader "gpt-load/internal/state/loader"
	"gpt-load/internal/storage/models"
)

// The home page's “X/Y credentials available” must use the same classifyHealthKey buckets as the health page:
// checking only status, blacklist, and cooldown would treat “reauthorisation required” and “weight manually set to zero” credentials as available,
// while the scheduler will never select them, making the two pages contradictory side by side.
func TestReadHomeBaseAvailableCredentialsMatchHealthClassification(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	now := time.Date(2026, time.August, 16, 9, 0, 0, 0, time.UTC)
	zeroWeight := 0

	group := validControlGroup("home-available-parity")
	if err := fixture.db.Create(group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}

	credentials := []models.Credential{
		{ID: 1, GroupID: group.ID, Data: "cipher-1", Fingerprint: "hash-1", Status: models.CredentialStatusActive},
		{ID: 2, GroupID: group.ID, Data: "cipher-2", Fingerprint: "hash-2", Status: models.CredentialStatusActive},
		{ID: 3, GroupID: group.ID, Data: "cipher-3", Fingerprint: "hash-3", Status: models.CredentialStatusActive},
	}
	if err := fixture.db.Create(&credentials).Error; err != nil {
		t.Fatalf("create credentials: %v", err)
	}

	entries := make([]state.CredentialEntry, 0, len(credentials))
	for index, credential := range credentials {
		entries = append(entries, state.CredentialEntry{
			ID: credential.ID, GroupID: group.ID, Status: state.CredentialStatusActive,
			Version:            groupCollectionCredentialVersion(credential.SecretVersion),
			IdentityGeneration: groupCollectionCredentialIdentity(credential.IdentityFingerprint, *group),
			Fingerprint:        credential.Fingerprint,
			EncryptedValue:     "cipher-" + string(rune('1'+index)),
			AuthState:          state.CredentialAuthStateReady,
		})
	}
	// Credential 2 requires reauthorisation; credential 3 was manually disabled (weight 0). Neither participates in scheduling.
	entries[1].AuthState = state.CredentialAuthStateReauthorizationRequired
	entries[2].WeightManual = &zeroWeight
	if err := fixture.registry.ReplaceCredentials(entries); err != nil {
		t.Fatalf("registry.ReplaceCredentials() error = %v", err)
	}

	input, err := stateloader.BuildCompileInput(context.Background(), fixture.db, fixture.channelRegistry)
	if err != nil {
		t.Fatalf("BuildCompileInput() error = %v", err)
	}
	if _, err := fixture.manager.Publish(input); err != nil {
		t.Fatalf("manager.Publish() error = %v", err)
	}
	fixture.service.registrySnapshot = fixture.registry.Snapshot

	base, err := fixture.service.ReadHomeBase(context.Background(), now.UnixMilli())
	if err != nil {
		t.Fatalf("ReadHomeBase() error = %v", err)
	}

	if base.Inventory.CredentialCount != 3 {
		t.Fatalf("CredentialCount = %d, want 3", base.Inventory.CredentialCount)
	}
	if base.Inventory.AvailableCredentialCount != 1 {
		t.Fatalf(
			"AvailableCredentialCount = %d, want 1 (credentials requiring reauthorisation and with weight 0 are unavailable)",
			base.Inventory.AvailableCredentialCount,
		)
	}

	// Compare each bucket with the health page to ensure both conclusions agree, not merely that the totals happen to match.
	snapshot := fixture.manager.Current()
	var healthAvailable int64
	for _, view := range fixture.registry.Snapshot() {
		catalog, ok := snapshot.GroupCatalog[view.GroupID]
		if !ok {
			t.Fatalf("group %d missing from catalog", view.GroupID)
		}
		if classifyHealthKey(catalog, view, now) == healthBucketAvailable {
			healthAvailable++
		}
	}
	if base.Inventory.AvailableCredentialCount != healthAvailable {
		t.Fatalf(
			"home available = %d, health available = %d; both definitions must agree",
			base.Inventory.AvailableCredentialCount,
			healthAvailable,
		)
	}
}
