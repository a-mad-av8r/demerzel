//go:build darwin || linux

package encryption

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"filippo.io/age"
)

func TestLoadOrCreateKeyMaterialUsesStableCustodyWithoutPlaintextFile(t *testing.T) {
	clearCustodyEnvironment(t)
	dataDir := t.TempDir()
	store := newMemoryKeyStore()
	factory := memoryStoreFactory(store)

	first, err := loadOrCreateKeyMaterial("", dataDir, factory, true)
	if err != nil {
		t.Fatalf("first loadOrCreateKeyMaterial() error = %v", err)
	}
	if len(first) != 64 {
		t.Fatalf("generated key length = %d, want 64 hex characters", len(first))
	}
	second, err := loadOrCreateKeyMaterial("", dataDir, factory, true)
	if err != nil {
		t.Fatalf("restarted loadOrCreateKeyMaterial() error = %v", err)
	}
	if second != first {
		t.Fatal("custody backend did not return the same key after restart")
	}
	for _, name := range []string{KeyFileName, AgeKeyFileName} {
		if _, err := os.Lstat(filepath.Join(dataDir, name)); !os.IsNotExist(err) {
			t.Fatalf("plaintext or unconfigured key file %q exists: %v", name, err)
		}
	}
}

func TestExternalDatabaseNeverSilentlyInitializesMissingMasterKey(t *testing.T) {
	clearCustodyEnvironment(t)
	dataDir := t.TempDir()
	store := newMemoryKeyStore()
	factory := memoryStoreFactory(store)

	if _, err := loadOrCreateKeyMaterial("", dataDir, factory, false); err == nil {
		t.Fatal("missing external database key was silently initialized")
	}
	if len(store.keys) != 0 {
		t.Fatal("failed external initialization wrote a new master key")
	}
	if _, err := os.Lstat(filepath.Join(dataDir, KeyFileName)); !os.IsNotExist(err) {
		t.Fatalf("failed external initialization wrote a plaintext key: %v", err)
	}
	first, err := loadOrCreateKeyMaterial("", dataDir, factory, true)
	if err != nil {
		t.Fatalf("explicit external initialization failed: %v", err)
	}
	restarted, err := loadOrCreateKeyMaterial("", dataDir, factory, false)
	if err != nil || restarted != first {
		t.Fatalf("existing external database key was not reused after restart: %v", err)
	}
}

func TestLoadOrCreateKeyMaterialFailsClosedWhenBackendUnavailable(t *testing.T) {
	clearCustodyEnvironment(t)
	dataDir := t.TempDir()
	factory := func() (masterKeyStore, error) {
		return nil, errors.New("backend unavailable")
	}

	if _, err := loadOrCreateKeyMaterial("", dataDir, factory, true); err == nil {
		t.Fatal("loadOrCreateKeyMaterial() error = nil, want unavailable backend error")
	}
	if _, err := os.Lstat(filepath.Join(dataDir, KeyFileName)); !os.IsNotExist(err) {
		t.Fatalf("backend failure created a plaintext key file: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dataDir, AgeKeyFileName)); !os.IsNotExist(err) {
		t.Fatalf("backend failure created an unconfigured age file: %v", err)
	}
}

func TestLoadOrCreateKeyMaterialRejectsEmptyCustodyValue(t *testing.T) {
	clearCustodyEnvironment(t)
	dataDir := t.TempDir()
	store := newMemoryKeyStore()
	store.keys[installationIdentifier(mustCanonicalDataDir(t, dataDir))] = ""

	if _, err := loadOrCreateKeyMaterial("", dataDir, memoryStoreFactory(store), true); err == nil {
		t.Fatal("empty custody item was treated as a missing key")
	}
	if _, err := os.Lstat(filepath.Join(dataDir, KeyFileName)); !os.IsNotExist(err) {
		t.Fatalf("empty custody item caused a plaintext keyfile to be created: %v", err)
	}
}

func TestLoadOrCreateKeyMaterialImportsLegacyKeyOnlyWithOptIn(t *testing.T) {
	clearCustodyEnvironment(t)
	dataDir := t.TempDir()
	legacyKey := hex.EncodeToString(make([]byte, 32))
	legacyPath := filepath.Join(dataDir, KeyFileName)
	if err := os.WriteFile(legacyPath, []byte(legacyKey), 0o600); err != nil {
		t.Fatalf("write legacy key fixture: %v", err)
	}
	store := newMemoryKeyStore()
	factory := memoryStoreFactory(store)

	if _, err := loadOrCreateKeyMaterial("", dataDir, factory, true); err == nil {
		t.Fatal("legacy collision was accepted without explicit import opt-in")
	}
	if len(store.keys) != 0 {
		t.Fatal("legacy key was imported without explicit opt-in")
	}
	t.Setenv(LegacyImportEnv, "1")
	imported, err := loadOrCreateKeyMaterial("", dataDir, factory, true)
	if err != nil {
		t.Fatalf("explicit legacy import error = %v", err)
	}
	if imported != legacyKey {
		t.Fatal("legacy import changed the original key material")
	}
	if contents, err := os.ReadFile(legacyPath); err != nil || string(contents) != legacyKey {
		t.Fatal("legacy source file was removed or changed during import")
	}
	t.Setenv(LegacyImportEnv, "")
	restarted, err := loadOrCreateKeyMaterial("", dataDir, factory, true)
	if err != nil || restarted != legacyKey {
		t.Fatal("verified custody key was not reused while preserving the legacy file")
	}
}

func TestLoadOrCreateKeyMaterialRejectsLegacyAndDatabaseKeyCollisions(t *testing.T) {
	clearCustodyEnvironment(t)
	dataDir := t.TempDir()
	legacyKey := hex.EncodeToString(make([]byte, 32))
	legacyPath := filepath.Join(dataDir, KeyFileName)
	if err := os.WriteFile(legacyPath, []byte(legacyKey), 0o600); err != nil {
		t.Fatalf("write legacy key fixture: %v", err)
	}
	store := newMemoryKeyStore()
	store.keys[installationIdentifier(mustCanonicalDataDir(t, dataDir))] = strings.Repeat("a", 64)

	if _, err := loadOrCreateKeyMaterial("", dataDir, memoryStoreFactory(store), true); err == nil {
		t.Fatal("mismatched custody and legacy keys were accepted")
	}
	if contents, err := os.ReadFile(legacyPath); err != nil || string(contents) != legacyKey {
		t.Fatal("mismatched legacy file was changed")
	}

	dataDirWithDB := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDirWithDB, "gpt-load.db"), []byte("existing database"), 0o600); err != nil {
		t.Fatalf("write database fixture: %v", err)
	}
	emptyStore := newMemoryKeyStore()
	if _, err := loadOrCreateKeyMaterial("", dataDirWithDB, memoryStoreFactory(emptyStore), true); err == nil {
		t.Fatal("missing key for an existing managed database was silently replaced")
	}
	if len(emptyStore.keys) != 0 {
		t.Fatal("missing database key caused a new custody key to be created")
	}
}

func TestLoadOrCreateKeyMaterialRejectsCorruptKeys(t *testing.T) {
	tests := []struct {
		name     string
		contents string
	}{
		{name: "empty legacy file", contents: ""},
		{name: "short legacy file", contents: "abcd"},
		{name: "odd legacy file", contents: "abc"},
		{name: "non-hex legacy file", contents: strings.Repeat("z", 64)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearCustodyEnvironment(t)
			dataDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dataDir, KeyFileName), []byte(test.contents), 0o600); err != nil {
				t.Fatalf("write corrupt legacy key: %v", err)
			}
			if _, err := loadOrCreateKeyMaterial("", dataDir, memoryStoreFactory(newMemoryKeyStore()), true); err == nil {
				t.Fatal("corrupt legacy key was accepted")
			}
		})
	}

	clearCustodyEnvironment(t)
	dataDir := t.TempDir()
	store := newMemoryKeyStore()
	store.keys[installationIdentifier(mustCanonicalDataDir(t, dataDir))] = "not-a-master-key"
	if _, err := loadOrCreateKeyMaterial("", dataDir, memoryStoreFactory(store), true); err == nil {
		t.Fatal("corrupt custody key was accepted")
	}
}

func TestLoadOrCreateKeyMaterialUsesExplicitAgeFileFallback(t *testing.T) {
	clearCustodyEnvironment(t)
	dataDir := t.TempDir()
	identityDirectory := t.TempDir()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate age identity: %v", err)
	}
	identityPath := filepath.Join(identityDirectory, "identity.txt")
	if err := os.WriteFile(identityPath, []byte(identity.String()+"\n"), 0o600); err != nil {
		t.Fatalf("write age identity: %v", err)
	}
	t.Setenv(AgeIdentityFileEnv, identityPath)
	t.Setenv(AgeRecipientEnv, identity.Recipient().String())

	first, err := LoadOrCreateKeyMaterial("", dataDir, true)
	if err != nil {
		t.Fatalf("first age fallback load error = %v", err)
	}
	second, err := LoadOrCreateKeyMaterial("", dataDir, true)
	if err != nil || second != first {
		t.Fatal("age fallback did not load the same key after restart")
	}
	ciphertext, err := os.ReadFile(filepath.Join(dataDir, AgeKeyFileName))
	if err != nil {
		t.Fatalf("read encrypted key file: %v", err)
	}
	if strings.Contains(string(ciphertext), first) {
		t.Fatal("age fallback stored the master key as plaintext")
	}
	if _, err := os.Lstat(filepath.Join(dataDir, KeyFileName)); !os.IsNotExist(err) {
		t.Fatalf("age fallback created a plaintext legacy key file: %v", err)
	}
}

func TestLoadOrCreateKeyMaterialRejectsCorruptAgeFile(t *testing.T) {
	clearCustodyEnvironment(t)
	dataDir := t.TempDir()
	identityDirectory := t.TempDir()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate age identity: %v", err)
	}
	identityPath := filepath.Join(identityDirectory, "identity.txt")
	if err := os.WriteFile(identityPath, []byte(identity.String()+"\n"), 0o600); err != nil {
		t.Fatalf("write age identity: %v", err)
	}
	t.Setenv(AgeIdentityFileEnv, identityPath)
	t.Setenv(AgeRecipientEnv, identity.Recipient().String())
	agePath := filepath.Join(dataDir, AgeKeyFileName)
	if err := os.WriteFile(agePath, []byte("not age ciphertext"), 0o600); err != nil {
		t.Fatalf("write corrupt age fixture: %v", err)
	}

	if _, err := LoadOrCreateKeyMaterial("", dataDir, true); err == nil {
		t.Fatal("corrupt age file was treated as a missing key")
	}
	if contents, err := os.ReadFile(agePath); err != nil || string(contents) != "not age ciphertext" {
		t.Fatal("corrupt age file was overwritten")
	}
}

func TestLoadOrCreateKeyMaterialSerializesConcurrentFirstBoot(t *testing.T) {
	clearCustodyEnvironment(t)
	dataDir := t.TempDir()
	store := newMemoryKeyStore()
	factory := memoryStoreFactory(store)
	const workers = 16
	start := make(chan struct{})
	keys := make(chan string, workers)
	errorsFound := make(chan error, workers)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			key, err := loadOrCreateKeyMaterial("", dataDir, factory, true)
			if err != nil {
				errorsFound <- err
				return
			}
			keys <- key
		}()
	}
	close(start)
	wait.Wait()
	close(keys)
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("concurrent first boot error = %v", err)
	}
	var want string
	for key := range keys {
		if want == "" {
			want = key
			continue
		}
		if key != want {
			t.Fatal("concurrent first boot returned different custody keys")
		}
	}
	if want == "" {
		t.Fatal("concurrent first boot returned no key")
	}
}
