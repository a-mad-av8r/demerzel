package migrations_test

import (
	"testing"

	"gpt-load/internal/storage/migrations"
)

func TestCredentialLabelMigrationPreservesEncryptedRowsAndDefaultsBlank(t *testing.T) {
	t.Parallel()
	db := openInitialTestDatabase(t)
	if err := migrations.Up0001(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO groups (id, name, channel_id, params, models, enabled, created_at_ms, updated_at_ms)
		VALUES (1, 'upstream', 'openai_compatible', '{}', '[]', true, 1, 1)`).Error; err != nil {
		t.Fatalf("seed pre-label group: %v", err)
	}
	if err := db.Exec(`INSERT INTO credentials
		(group_id, data, fingerprint, identity_fingerprint, status, created_at_ms, updated_at_ms)
		VALUES (1, 'encrypted-original', 'fingerprint-1', 'identity-1', 'active', 1, 1)`).Error; err != nil {
		t.Fatalf("seed pre-label encrypted credential: %v", err)
	}
	if err := migrations.Up0021(db); err != nil {
		t.Fatalf("add account label: %v", err)
	}
	var before struct{ Label, Data, Fingerprint string }
	if err := db.Table("credentials").Select("label", "data", "fingerprint").Take(&before).Error; err != nil {
		t.Fatal(err)
	}
	if before.Label != "" || before.Data != "encrypted-original" || before.Fingerprint != "fingerprint-1" {
		t.Fatalf("legacy credential changed by label migration: %#v", before)
	}
	if err := db.Table("credentials").Where("group_id = 1").Update("label", "Personal account").Error; err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up0021(db); err != nil {
		t.Fatalf("repeated account-label migration: %v", err)
	}
	var after struct{ Label, Data string }
	if err := db.Table("credentials").Select("label", "data").Take(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.Label != "Personal account" || after.Data != before.Data {
		t.Fatalf("repeated migration changed account data: %#v", after)
	}
}
