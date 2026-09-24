package encryption

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gpt-load/internal/platform/utils"
)

func TestServiceUsesDomainSeparatedFingerprintKey(t *testing.T) {
	const (
		masterKey = "domain-separated-master-key"
		plaintext = "sk-secret-value"
	)
	service, err := NewService(masterKey)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	legacyMAC := hmac.New(sha256.New, utils.DeriveAESKey(masterKey))
	_, _ = legacyMAC.Write([]byte(plaintext))
	legacyHash := hex.EncodeToString(legacyMAC.Sum(nil))

	if got := service.Hash(plaintext); got == legacyHash {
		t.Fatal("Hash() reuses the AES key; fingerprinting requires a domain-separated subkey")
	}
}

func TestServiceEncryptDecryptAndStableHash(t *testing.T) {
	service, err := NewService("a-test-master-key")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ciphertext, err := service.Encrypt("sk-secret-value")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if ciphertext == "sk-secret-value" {
		t.Fatal("Encrypt() returned plaintext")
	}

	plaintext, err := service.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "sk-secret-value" {
		t.Fatalf("Decrypt() = %q", plaintext)
	}

	first := service.Hash("sk-secret-value")
	second := service.Hash("sk-secret-value")
	if first == "" || first != second {
		t.Fatalf("Hash() is not stable: %q != %q", first, second)
	}
}

func TestNewServiceRejectsEmptyMasterKey(t *testing.T) {
	if _, err := NewService(""); err == nil {
		t.Fatal("NewService() error = nil, want error")
	}
}

func TestLoadOrCreateKeyMaterialExplicitKeyDoesNotUseBackend(t *testing.T) {
	clearCustodyEnvironment(t)
	dataDir := t.TempDir()
	called := false
	factory := func() (masterKeyStore, error) {
		called = true
		return nil, errors.New("backend unavailable")
	}

	got, err := loadOrCreateKeyMaterial("explicit-headless-test-key", dataDir, factory, false)
	if err != nil {
		t.Fatalf("explicit loadOrCreateKeyMaterial() error = %v", err)
	}
	if got != "explicit-headless-test-key" || called {
		t.Fatal("explicit key was not used directly")
	}
	if _, err := os.Lstat(filepath.Join(dataDir, KeyFileName)); !os.IsNotExist(err) {
		t.Fatalf("explicit key created a plaintext key file: %v", err)
	}
}

func TestNewServiceWithCustodyUsesExplicitMaterial(t *testing.T) {
	dataDir := t.TempDir()
	service, err := NewServiceWithCustody("explicit-service-master-key", dataDir, false)
	if err != nil {
		t.Fatalf("NewServiceWithCustody() error = %v", err)
	}
	ciphertext, err := service.Encrypt("value")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if plaintext, err := service.Decrypt(ciphertext); err != nil || plaintext != "value" {
		t.Fatalf("Decrypt() did not preserve plaintext: error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dataDir, KeyFileName)); !os.IsNotExist(err) {
		t.Fatalf("explicit service key created a plaintext file: %v", err)
	}
}

func clearCustodyEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(LegacyImportEnv, "")
	t.Setenv(AgeIdentityFileEnv, "")
	t.Setenv(AgeRecipientEnv, "")
}

func memoryStoreFactory(store *memoryKeyStore) keyStoreFactory {
	return func() (masterKeyStore, error) {
		return store, nil
	}
}

type memoryKeyStore struct {
	mu   sync.Mutex
	keys map[string]string
}

func newMemoryKeyStore() *memoryKeyStore {
	return &memoryKeyStore{keys: make(map[string]string)}
}

func (store *memoryKeyStore) Load(installationID string) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	keyMaterial, exists := store.keys[installationID]
	if !exists {
		return "", errKeyNotFound
	}
	return keyMaterial, nil
}

func (store *memoryKeyStore) StoreIfAbsent(installationID, keyMaterial string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.keys[installationID]; exists {
		return errKeyAlreadyExists
	}
	store.keys[installationID] = keyMaterial
	return nil
}

func mustCanonicalDataDir(t *testing.T, dataDir string) string {
	t.Helper()
	canonicalDir, err := canonicalDataDir(dataDir)
	if err != nil {
		t.Fatalf("canonicalDataDir() error = %v", err)
	}
	return canonicalDir
}
