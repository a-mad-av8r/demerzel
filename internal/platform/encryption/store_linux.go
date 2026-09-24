//go:build linux

package encryption

import (
	"errors"

	"github.com/keybase/dbus"
	"github.com/keybase/go-keychain/secretservice"
)

const (
	secretServiceLabel = "Demerzel encryption master key"
	secretServiceApp   = "io.demerzel"
	secretServiceUse   = "encryption.master-key.v1"
)

type systemKeyStore struct {
	service *secretservice.SecretService
}

func newSystemKeyStore() (masterKeyStore, error) {
	service, err := secretservice.NewService()
	if err != nil {
		return nil, errors.New("Linux Secret Service session bus is unavailable")
	}
	return &systemKeyStore{service: service}, nil
}

func (store *systemKeyStore) Load(installationID string) (string, error) {
	collection, err := store.defaultCollection()
	if err != nil {
		return "", err
	}
	attributes := secretAttributes(installationID)
	unlocked, locked, err := store.search(collection, attributes)
	if err != nil {
		return "", errors.New("Linux Secret Service lookup failed")
	}
	if len(unlocked) == 0 && len(locked) == 0 {
		return "", errKeyNotFound
	}
	if len(locked) != 0 {
		if err := store.service.Unlock(locked); err != nil {
			return "", errors.New("Linux Secret Service item is locked or could not be unlocked")
		}
		unlocked, locked, err = store.search(collection, attributes)
		if err != nil || len(locked) != 0 {
			return "", errors.New("Linux Secret Service item remains locked")
		}
	}
	if len(unlocked) != 1 {
		return "", errors.New("Linux Secret Service contains ambiguous master-key items")
	}

	session, err := store.service.OpenSession(secretservice.AuthenticationDHAES)
	if err != nil {
		return "", errors.New("Linux Secret Service secure session could not be opened")
	}
	defer store.service.CloseSession(session)
	keyMaterial, err := store.service.GetSecret(unlocked[0], *session)
	defer clear(keyMaterial)
	if err != nil {
		return "", errors.New("Linux Secret Service master-key retrieval failed")
	}
	return string(keyMaterial), nil
}

func (store *systemKeyStore) StoreIfAbsent(installationID, keyMaterial string) error {
	collection, err := store.defaultCollection()
	if err != nil {
		return err
	}
	unlocked, locked, err := store.search(collection, secretAttributes(installationID))
	if err != nil {
		return errors.New("Linux Secret Service lookup failed before store")
	}
	if len(unlocked) != 0 || len(locked) != 0 {
		return errKeyAlreadyExists
	}

	session, err := store.service.OpenSession(secretservice.AuthenticationDHAES)
	if err != nil {
		return errors.New("Linux Secret Service secure session could not be opened")
	}
	defer store.service.CloseSession(session)
	keyBytes := []byte(keyMaterial)
	secret, err := session.NewSecret(keyBytes)
	clear(keyBytes)
	if err != nil {
		return errors.New("Linux Secret Service could not protect the master key")
	}
	properties := secretservice.NewSecretProperties(secretServiceLabel, secretAttributes(installationID))
	if _, err := store.service.CreateItem(collection, properties, secret, secretservice.ReplaceBehaviorDoNotReplace); err != nil {
		return errors.New("Linux Secret Service could not store the master key")
	}
	return nil
}

func (store *systemKeyStore) defaultCollection() (dbus.ObjectPath, error) {
	var collection dbus.ObjectPath
	err := store.service.ServiceObj().
		Call("org.freedesktop.Secret.Service.ReadAlias", secretservice.NilFlags, "default").
		Store(&collection)
	if err != nil || collection == "/" {
		return "", errors.New("Linux Secret Service has no usable default collection")
	}
	return collection, nil
}

func (store *systemKeyStore) search(
	collection dbus.ObjectPath,
	attributes secretservice.Attributes,
) ([]dbus.ObjectPath, []dbus.ObjectPath, error) {
	var unlocked, locked []dbus.ObjectPath
	err := store.service.Obj(collection).
		Call("org.freedesktop.Secret.Collection.SearchItems", secretservice.NilFlags, attributes).
		Store(&unlocked, &locked)
	return unlocked, locked, err
}

func secretAttributes(installationID string) secretservice.Attributes {
	return secretservice.Attributes{
		"application":  secretServiceApp,
		"purpose":      secretServiceUse,
		"installation": installationID,
	}
}
