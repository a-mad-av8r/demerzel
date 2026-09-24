package encryption

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// AgeKeyFileName is the ciphertext filename used only by explicit headless setups.
	AgeKeyFileName = "encryption.key.age"

	// LegacyImportEnv opts in to importing DATA_DIR/encryption.key into custody.
	LegacyImportEnv = "DEMERZEL_ENCRYPTION_KEY_IMPORT_LEGACY"
	// AgeIdentityFileEnv names an externally managed age identity file.
	AgeIdentityFileEnv = "DEMERZEL_ENCRYPTION_KEY_AGE_IDENTITY_FILE"
	// AgeRecipientEnv names the age recipient used to encrypt a headless key.
	AgeRecipientEnv = "DEMERZEL_ENCRYPTION_KEY_AGE_RECIPIENT"

	keyCustodyLockFile = ".encryption-key.lock"
)

var (
	errKeyNotFound      = errors.New("master key not found")
	errKeyAlreadyExists = errors.New("master key already exists")
)

type masterKeyStore interface {
	Load(installationID string) (string, error)
	StoreIfAbsent(installationID, keyMaterial string) error
}

type keyStoreFactory func() (masterKeyStore, error)

func loadOrCreateKeyMaterial(explicitKey, dataDir string, newStore keyStoreFactory, allowInitialization bool) (string, error) {
	if explicitKey != "" {
		return explicitKeyUnlessLegacyCollision(explicitKey, dataDir)
	}
	if dataDir == "" {
		return "", errors.New("DATA_DIR is required when ENCRYPTION_KEY is empty")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return "", fmt.Errorf("native and age-file custody are unsupported on %s; set ENCRYPTION_KEY explicitly", runtime.GOOS)
	}

	canonicalDir, err := canonicalDataDir(dataDir)
	if err != nil {
		return "", errors.New("DATA_DIR must be an existing directory for key custody")
	}
	unlock, err := lockKeyCustody(canonicalDir)
	if err != nil {
		return "", fmt.Errorf("cannot lock master-key custody; verify DATA_DIR permissions: %w", err)
	}
	defer unlock()

	installationID := installationIdentifier(canonicalDir)
	legacyKey, legacyExists, err := readLegacyKey(canonicalDir)
	if err != nil {
		return "", err
	}

	store, err := configuredKeyStore(canonicalDir, newStore)
	if err != nil {
		return "", err
	}
	storedKey, loadErr := store.Load(installationID)
	if loadErr != nil && !errors.Is(loadErr, errKeyNotFound) {
		return "", loadErr
	}
	if loadErr == nil {
		if err := validateCustodiedKey(storedKey); err != nil {
			return "", err
		}
		if legacyExists && storedKey != legacyKey {
			return "", errors.New("DATA_DIR/encryption.key conflicts with the custody key; restore the matching key or database backup")
		}
		return storedKey, nil
	}

	if legacyExists {
		if os.Getenv(LegacyImportEnv) != "1" {
			return "", fmt.Errorf("legacy DATA_DIR/encryption.key exists; back it up, then set %s=1 to import it without deleting the source file", LegacyImportEnv)
		}
		return importLegacyKey(store, installationID, legacyKey)
	}

	if managedDatabaseExists(canonicalDir) {
		return "", errors.New("managed database exists but its master key is unavailable; refusing to generate a replacement; restore the key from backup, provide ENCRYPTION_KEY, or recover the original key")
	}

	if !allowInitialization {
		return "", errors.New("external database key is unavailable; refusing implicit initialization; recover the original key or explicitly authorize first initialization for an empty database")
	}

	keyMaterial, err := generateKeyMaterial()
	if err != nil {
		return "", errors.New("could not generate master key material")
	}
	if err := store.StoreIfAbsent(installationID, keyMaterial); err != nil {
		if !errors.Is(err, errKeyAlreadyExists) {
			return "", custodyBackendError()
		}
		return loadWinnerAfterStoreRace(store, installationID)
	}
	persistedKey, err := store.Load(installationID)
	if err != nil || persistedKey != keyMaterial {
		return "", custodyBackendError()
	}
	if err := validateCustodiedKey(persistedKey); err != nil {
		return "", err
	}
	return persistedKey, nil
}

func explicitKeyUnlessLegacyCollision(explicitKey, dataDir string) (string, error) {
	if dataDir == "" {
		return explicitKey, nil
	}
	canonicalDir, err := canonicalDataDir(dataDir)
	if errors.Is(err, os.ErrNotExist) {
		return explicitKey, nil
	}
	if err != nil {
		return "", errors.New("cannot inspect DATA_DIR for a legacy encryption key")
	}
	legacyKey, exists, err := readLegacyKey(canonicalDir)
	if err != nil {
		return "", err
	}
	if exists && explicitKey != legacyKey {
		return "", errors.New("ENCRYPTION_KEY does not match DATA_DIR/encryption.key; restore the matching key or database backup")
	}
	return explicitKey, nil
}

func configuredKeyStore(dataDir string, newStore keyStoreFactory) (masterKeyStore, error) {
	identityPath := os.Getenv(AgeIdentityFileEnv)
	recipient := os.Getenv(AgeRecipientEnv)
	if identityPath == "" && recipient == "" {
		return newStore()
	}
	if identityPath == "" || recipient == "" {
		return nil, errors.New("both age identity-file and recipient settings are required")
	}
	return newAgeFileStore(dataDir, identityPath, recipient)
}

func importLegacyKey(store masterKeyStore, installationID, legacyKey string) (string, error) {
	if err := store.StoreIfAbsent(installationID, legacyKey); err != nil &&
		!errors.Is(err, errKeyAlreadyExists) {
		return "", custodyBackendError()
	}
	return loadAndMatchLegacyKey(store, installationID, legacyKey)
}

func loadAndMatchLegacyKey(store masterKeyStore, installationID, legacyKey string) (string, error) {
	storedKey, err := store.Load(installationID)
	if err != nil {
		return "", err
	}
	if err := validateCustodiedKey(storedKey); err != nil {
		return "", err
	}
	if storedKey != legacyKey {
		return "", errors.New("custody key does not match DATA_DIR/encryption.key; source file was preserved; restore the matching key or database backup")
	}
	return storedKey, nil
}

func loadWinnerAfterStoreRace(store masterKeyStore, installationID string) (string, error) {
	storedKey, err := store.Load(installationID)
	if err != nil {
		return "", err
	}
	if err := validateCustodiedKey(storedKey); err != nil {
		return "", err
	}
	return storedKey, nil
}

func custodyBackendError() error {
	return fmt.Errorf(
		"master-key custody backend is unavailable or locked; unlock/install macOS Keychain or Linux Secret Service, set both %s and %s for explicit age-file custody, or provide ENCRYPTION_KEY; no plaintext keyfile was created",
		AgeIdentityFileEnv,
		AgeRecipientEnv,
	)
}

func canonicalDataDir(dataDir string) (string, error) {
	absolutePath, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolvedPath)
	if err != nil || !info.IsDir() {
		return "", errors.New("DATA_DIR is not a directory")
	}
	return filepath.Clean(resolvedPath), nil
}

func installationIdentifier(dataDir string) string {
	identity := sha256.Sum256([]byte("demerzel/master-key/v1\x00" + filepath.Clean(dataDir)))
	return hex.EncodeToString(identity[:])
}

// InstallationIdentifier is the non-secret account locator for the native
// Keychain or Secret Service item associated with this canonical DATA_DIR.
func InstallationIdentifier(dataDir string) (string, error) {
	if dataDir == "" {
		return "", errors.New("DATA_DIR is required for the key locator")
	}
	canonicalDir, err := canonicalDataDir(dataDir)
	if err != nil {
		return "", fmt.Errorf("resolve DATA_DIR key locator: %w", err)
	}
	return installationIdentifier(canonicalDir), nil
}

func readLegacyKey(dataDir string) (string, bool, error) {
	path := filepath.Join(dataDir, KeyFileName)
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	} else if err != nil {
		return "", false, errors.New("cannot inspect legacy DATA_DIR/encryption.key; preserve it and check filesystem permissions")
	}
	file, err := openCustodyFile(path)
	if err != nil {
		return "", true, errors.New("legacy DATA_DIR/encryption.key is not a safe regular file; preserve it and recover from backup")
	}
	defer file.Close()

	contents, err := io.ReadAll(io.LimitReader(file, 129))
	defer clear(contents)
	if err != nil || len(contents) > 128 {
		return "", true, errors.New("legacy DATA_DIR/encryption.key cannot be read safely; preserve it and recover from backup")
	}
	keyMaterial := strings.TrimSpace(string(contents))
	if err := validateCustodiedKey(keyMaterial); err != nil {
		return "", true, errors.New("legacy DATA_DIR/encryption.key is corrupt; preserve it and restore a verified backup")
	}
	return keyMaterial, true, nil
}

func validateCustodiedKey(keyMaterial string) error {
	decoded, err := hex.DecodeString(keyMaterial)
	if err != nil || len(keyMaterial) != 64 || len(decoded) != 32 {
		return errors.New("custody backend returned corrupt master-key material; restore the key from backup or recover the original key")
	}
	return nil
}

func generateKeyMaterial() (string, error) {
	keyBytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, keyBytes); err != nil {
		return "", err
	}
	keyMaterial := hex.EncodeToString(keyBytes)
	for index := range keyBytes {
		keyBytes[index] = 0
	}
	return keyMaterial, nil
}

func managedDatabaseExists(dataDir string) bool {
	for _, name := range []string{"gpt-load.db", "gpt-load.db-wal", "gpt-load.db-shm", "gpt-load.db-journal"} {
		if _, err := os.Lstat(filepath.Join(dataDir, name)); err == nil || !errors.Is(err, os.ErrNotExist) {
			return true
		}
	}
	return false
}
