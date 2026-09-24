// Package encryption provides mandatory AES-256-GCM encryption and stable
// HMAC fingerprints for stored credentials.
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"gpt-load/internal/platform/utils"
)

// KeyFileName is the legacy plaintext key path, retained only for guarded import.
const (
	KeyFileName          = "encryption.key"
	encryptionKeyDomain  = "gpt-load/encryption/aes-256-gcm/v1"
	fingerprintKeyDomain = "gpt-load/encryption/fingerprint-hmac/v1"
)

// Service defines credential encryption and fingerprinting operations.
type Service interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
	Hash(plaintext string) string
}

// NewService creates a mandatory AES-GCM service from non-empty key material.
func NewService(keyMaterial string) (Service, error) {
	if keyMaterial == "" {
		return nil, fmt.Errorf("encryption key material is required")
	}

	rootKey := utils.DeriveAESKey(keyMaterial)
	aesKey := deriveDomainKey(rootKey, encryptionKeyDomain)
	hashKey := deriveDomainKey(rootKey, fingerprintKeyDomain)
	utils.ValidatePasswordStrength(keyMaterial, "ENCRYPTION_KEY")

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	return &aesService{hashKey: hashKey, gcm: gcm}, nil
}

// NewServiceWithCustody resolves the master key without creating one when
// initialization is forbidden (for example, for a pre-existing external DB).
func NewServiceWithCustody(explicitKey, dataDir string, allowInitialization bool) (Service, error) {
	keyMaterial, err := LoadOrCreateKeyMaterial(explicitKey, dataDir, allowInitialization)
	if err != nil {
		return nil, err
	}
	return NewService(keyMaterial)
}

// LoadOrCreateKeyMaterial loads the selected custody backend, allowing first
// initialization only when the caller has established its database policy.
func LoadOrCreateKeyMaterial(explicitKey, dataDir string, allowInitialization bool) (string, error) {
	return loadOrCreateKeyMaterial(explicitKey, dataDir, newSystemKeyStore, allowInitialization)
}

type aesService struct {
	hashKey []byte
	gcm     cipher.AEAD
}

func deriveDomainKey(rootKey []byte, domain string) []byte {
	mac := hmac.New(sha256.New, rootKey)
	_, _ = mac.Write([]byte(domain))
	return mac.Sum(nil)
}

func (s *aesService) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate encryption nonce: %w", err)
	}
	ciphertext := s.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

func (s *aesService) Decrypt(ciphertext string) (string, error) {
	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	if len(data) < s.gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, encrypted := data[:s.gcm.NonceSize()], data[s.gcm.NonceSize():]
	plaintext, err := s.gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}
	return string(plaintext), nil
}

func (s *aesService) Hash(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	mac := hmac.New(sha256.New, s.hashKey)
	_, _ = mac.Write([]byte(plaintext))
	return hex.EncodeToString(mac.Sum(nil))
}
