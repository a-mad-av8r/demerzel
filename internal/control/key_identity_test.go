package control

import (
	"strings"
	"testing"

	"gpt-load/internal/platform/encryption"
	"gpt-load/internal/storage/models"
)

func TestPersistedMasterIdentityRefusesWrongKeyBeforeBoot(t *testing.T) {
	fixture := newServiceFixture(t)
	if err := fixture.service.EnsureInitialState(t.Context()); err != nil {
		t.Fatal(err)
	}
	var marker models.SystemSetting
	if err := fixture.db.Where("key = ?", masterKeyIdentitySetting).Take(&marker).Error; err != nil {
		t.Fatal(err)
	}
	wrong, err := encryption.NewService("wrong-master-key-for-boot")
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.encryption = wrong
	if err := fixture.service.EnsureInitialState(t.Context()); err == nil ||
		!strings.Contains(err.Error(), "does not match the database identity") {
		t.Fatalf("wrong master key started against existing state: %v", err)
	}
	var after models.SystemSetting
	if err := fixture.db.Where("key = ?", masterKeyIdentitySetting).Take(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.Value != marker.Value {
		t.Fatal("wrong key changed durable identity marker")
	}
}

func TestExistingEncryptedDatabaseNeedsMatchingKeyBeforeIdentityMigration(t *testing.T) {
	fixture := newServiceFixture(t)
	if err := fixture.service.EnsureInitialState(t.Context()); err != nil {
		t.Fatal(err)
	}
	var original models.AccessKey
	if err := fixture.db.Take(&original).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Where("key = ?", masterKeyIdentitySetting).Delete(&models.SystemSetting{}).Error; err != nil {
		t.Fatal(err)
	}
	wrong, err := encryption.NewService("wrong-master-key-for-legacy-migration")
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.encryption = wrong
	if err := fixture.service.EnsureInitialState(t.Context()); err == nil ||
		!strings.Contains(err.Error(), "existing encrypted state cannot be read") {
		t.Fatalf("unmarked database accepted wrong master key: %v", err)
	}
	var markerCount int64
	if err := fixture.db.Model(&models.SystemSetting{}).Where("key = ?", masterKeyIdentitySetting).
		Count(&markerCount).Error; err != nil || markerCount != 0 {
		t.Fatalf("failed boot wrote identity marker: count=%d error=%v", markerCount, err)
	}
	fixture.service.encryption = fixture.encryption
	if err := fixture.service.EnsureInitialState(t.Context()); err != nil {
		t.Fatalf("matching original key failed identity migration: %v", err)
	}
	var after models.AccessKey
	if err := fixture.db.Take(&after, original.ID).Error; err != nil || after.KeyValue != original.KeyValue {
		t.Fatalf("matching migration changed encrypted access key: %v", err)
	}
}
