//go:build darwin && !ios

package encryption

import (
	"errors"

	"github.com/keybase/go-keychain"
)

const (
	keychainService = "io.demerzel.encryption.master-key.v1"
	keychainLabel   = "Demerzel encryption master key"
)

type systemKeyStore struct{}

func newSystemKeyStore() (masterKeyStore, error) {
	return systemKeyStore{}, nil
}

func (systemKeyStore) Load(installationID string) (string, error) {
	keyMaterial, err := keychain.GetGenericPassword(keychainService, installationID, keychainLabel, "")
	if err != nil {
		return "", errors.New("macOS Keychain lookup failed")
	}
	if keyMaterial == nil {
		return "", errKeyNotFound
	}
	defer clear(keyMaterial)
	return string(keyMaterial), nil
}

func (systemKeyStore) StoreIfAbsent(installationID, keyMaterial string) error {
	materialBytes := []byte(keyMaterial)
	defer clear(materialBytes)
	item := keychain.NewGenericPassword(
		keychainService,
		installationID,
		keychainLabel,
		materialBytes,
		"",
	)
	item.SetSynchronizable(keychain.SynchronizableNo)
	item.SetAccessible(keychain.AccessibleWhenUnlocked)
	if err := keychain.AddItem(item); err != nil {
		if errors.Is(err, keychain.ErrorDuplicateItem) {
			return errKeyAlreadyExists
		}
		return errors.New("macOS Keychain item could not be created")
	}
	return nil
}
