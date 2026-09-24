//go:build darwin || linux

package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"gpt-load/internal/platform/config"
	"gpt-load/internal/platform/encryption"
	"gpt-load/internal/storage"
)

func prepareEmptyExternalDatabase(t *testing.T, path string) {
	t.Helper()
	db, err := storage.OpenWithSource(path, config.DatabaseSourceExternal)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExternalDatabaseRequiresExplicitFirstKeyInitialization(t *testing.T) {
	dataDir := t.TempDir()
	databasePath := filepath.Join(t.TempDir(), "external.db")
	prepareEmptyExternalDatabase(t, databasePath)
	identityDir := t.TempDir()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	identityPath := filepath.Join(identityDir, "identity.txt")
	if err := os.WriteFile(identityPath, []byte(identity.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_DSN", databasePath)
	t.Setenv("AUTH_KEY", "external-db-auth-key")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv(encryption.AgeIdentityFileEnv, identityPath)
	t.Setenv(encryption.AgeRecipientEnv, identity.Recipient().String())
	t.Setenv("DEMERZEL_ENCRYPTION_KEY_INITIALIZE_EXTERNAL", "")

	if _, err := BuildContainer(); err == nil ||
		!strings.Contains(err.Error(), "refusing implicit initialization") {
		t.Fatalf("external DB started without a pre-existing key: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dataDir, encryption.AgeKeyFileName)); !os.IsNotExist(err) {
		t.Fatalf("rejected first boot persisted a new key: %v", err)
	}

	t.Setenv("DEMERZEL_ENCRYPTION_KEY_INITIALIZE_EXTERNAL", "1")
	secondContainer, err := BuildContainer()
	if err != nil {
		t.Fatal(err)
	}
	var first encryption.Service
	if err := secondContainer.Invoke(func(service encryption.Service) { first = service }); err != nil {
		t.Fatalf("explicit first boot: %v", err)
	}
	ciphertext, err := first.Encrypt("external-database-credential")
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("DEMERZEL_ENCRYPTION_KEY_INITIALIZE_EXTERNAL", "")
	thirdContainer, err := BuildContainer()
	if err != nil {
		t.Fatal(err)
	}
	var restarted encryption.Service
	if err := thirdContainer.Invoke(func(service encryption.Service) { restarted = service }); err != nil {
		t.Fatalf("restart without opt-in: %v", err)
	}
	plaintext, err := restarted.Decrypt(ciphertext)
	if err != nil || plaintext != "external-database-credential" {
		t.Fatalf("external DB custody changed after restart: %v", err)
	}
}

func TestManagedFirstBootCreatesCustodyBeforeOpeningDatabase(t *testing.T) {
	dataDir := t.TempDir()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	identityPath := filepath.Join(t.TempDir(), "identity.txt")
	if err := os.WriteFile(identityPath, []byte(identity.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_DSN", "")
	t.Setenv("AUTH_KEY", "managed-db-auth-key")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv(encryption.AgeIdentityFileEnv, identityPath)
	t.Setenv(encryption.AgeRecipientEnv, identity.Recipient().String())
	t.Setenv("DEMERZEL_ENCRYPTION_KEY_INITIALIZE_EXTERNAL", "")

	firstContainer, err := BuildContainer()
	if err != nil {
		t.Fatalf("fresh managed boot: %v", err)
	}
	var first encryption.Service
	if err := firstContainer.Invoke(func(service encryption.Service) { first = service }); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := first.Encrypt("managed-database-credential")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gpt-load.db", encryption.AgeKeyFileName} {
		if _, err := os.Stat(filepath.Join(dataDir, name)); err != nil {
			t.Fatalf("fresh boot omitted %s: %v", name, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(dataDir, encryption.KeyFileName)); !os.IsNotExist(err) {
		t.Fatalf("fresh boot wrote a plaintext master key: %v", err)
	}
	restartedContainer, err := BuildContainer()
	if err != nil {
		t.Fatalf("managed restart: %v", err)
	}
	var restarted encryption.Service
	if err := restartedContainer.Invoke(func(service encryption.Service) { restarted = service }); err != nil {
		t.Fatal(err)
	}
	plaintext, err := restarted.Decrypt(ciphertext)
	if err != nil || plaintext != "managed-database-credential" {
		t.Fatalf("managed master key changed after restart: %v", err)
	}
	agePath := filepath.Join(dataDir, encryption.AgeKeyFileName)
	if err := os.Remove(agePath); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildContainer(); err == nil ||
		!strings.Contains(err.Error(), "managed database exists") {
		t.Fatalf("lost custody key silently initialized a replacement: %v", err)
	}
	if _, err := os.Lstat(agePath); !os.IsNotExist(err) {
		t.Fatalf("lost custody key was replaced after refusal: %v", err)
	}
}
