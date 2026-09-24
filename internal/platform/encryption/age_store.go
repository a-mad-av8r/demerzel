package encryption

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
)

const (
	maxAgeIdentityBytes     = 1 << 20
	maxAgeEncryptedKeyBytes = 1 << 20
)

type ageFileStore struct {
	path       string
	recipient  *age.X25519Recipient
	identities []age.Identity
}

func newAgeFileStore(dataDir, identityPath, recipientString string) (*ageFileStore, error) {
	canonicalIdentityPath, err := canonicalPrivateIdentityPath(identityPath)
	if err != nil || pathWithin(dataDir, canonicalIdentityPath) {
		return nil, errors.New("age identity must be a private file outside DATA_DIR")
	}
	identityFile, err := openCustodyFile(canonicalIdentityPath)
	if err != nil {
		return nil, errors.New("cannot read the external age identity file safely")
	}
	identityContents, err := io.ReadAll(io.LimitReader(identityFile, maxAgeIdentityBytes+1))
	_ = identityFile.Close()
	if err != nil || len(identityContents) == 0 || len(identityContents) > maxAgeIdentityBytes {
		return nil, errors.New("external age identity file is empty, unreadable, or too large")
	}
	identities, err := age.ParseIdentities(strings.NewReader(string(identityContents)))
	for index := range identityContents {
		identityContents[index] = 0
	}
	if err != nil || len(identities) == 0 {
		return nil, errors.New("external age identity file contains no usable identity")
	}
	recipient, err := age.ParseX25519Recipient(recipientString)
	if err != nil {
		return nil, errors.New("age recipient is invalid")
	}
	recipientMatched := false
	for _, identity := range identities {
		x25519Identity, ok := identity.(*age.X25519Identity)
		if ok && x25519Identity.Recipient().String() == recipient.String() {
			recipientMatched = true
			break
		}
	}
	if !recipientMatched {
		return nil, errors.New("age recipient does not match an X25519 identity in the external identity file")
	}
	return &ageFileStore{
		path:       filepath.Join(dataDir, AgeKeyFileName),
		recipient:  recipient,
		identities: identities,
	}, nil
}

func (store *ageFileStore) Load(string) (string, error) {
	file, err := openCustodyFile(store.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errKeyNotFound
		}
		return "", errors.New("age-encrypted master-key file is not a safe regular file")
	}
	defer file.Close()

	plaintext, err := age.Decrypt(io.LimitReader(file, maxAgeEncryptedKeyBytes), store.identities...)
	if err != nil {
		return "", errors.New("cannot decrypt age-encrypted master-key file; check the external age identity")
	}
	keyMaterial, err := io.ReadAll(io.LimitReader(plaintext, 65))
	defer clear(keyMaterial)
	if err != nil || len(keyMaterial) != 64 {
		return "", errors.New("age-encrypted master-key file is corrupt; recover its verified backup")
	}
	return string(keyMaterial), nil
}

func (store *ageFileStore) StoreIfAbsent(_ string, keyMaterial string) error {
	temporaryFile, err := os.CreateTemp(filepath.Dir(store.path), ".encryption-key-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	if err := temporaryFile.Chmod(0o600); err != nil {
		_ = temporaryFile.Close()
		return err
	}

	writer, err := age.Encrypt(temporaryFile, store.recipient)
	if err != nil {
		_ = temporaryFile.Close()
		return err
	}
	_, writeErr := io.WriteString(writer, keyMaterial)
	closeErr := writer.Close()
	if writeErr != nil || closeErr != nil {
		_ = temporaryFile.Close()
		return errors.New("cannot encrypt master key for headless custody")
	}
	if err := temporaryFile.Sync(); err != nil {
		_ = temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, store.path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return errKeyAlreadyExists
		}
		return err
	}
	return syncCustodyDirectory(filepath.Dir(store.path))
}

func canonicalPrivateIdentityPath(path string) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolvedPath), nil
}

func pathWithin(parent, path string) bool {
	relativePath, err := filepath.Rel(parent, path)
	if err != nil {
		return true
	}
	return relativePath == "." || (relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator)))
}
