package control

import (
	"fmt"

	"gorm.io/gorm"

	"gpt-load/internal/automodel"
	"gpt-load/internal/outboundproxy"
	app_errors "gpt-load/internal/platform/errors"
	"gpt-load/internal/storage/models"
)

const (
	masterKeyIdentitySetting = models.InternalSystemSettingPrefix + "encryption.master_key_identity.v1"
	masterKeyIdentityDomain  = "demerzel/encryption/master-key-identity/v1"
)

// verifyMasterKeyIdentity is called after migrations, before any startup state
// is published. The marker is non-secret; its value is a domain-separated HMAC.
func (s *Service) verifyMasterKeyIdentity(tx *gorm.DB) error {
	if s.encryption == nil {
		return fmt.Errorf("master-key custody is unavailable: %w", app_errors.ErrInternalServer)
	}
	identity := s.encryption.Hash(masterKeyIdentityDomain)
	if len(identity) != 64 {
		return fmt.Errorf("master-key identity cannot be computed: %w", app_errors.ErrInternalServer)
	}
	var marker models.SystemSetting
	result := masterKeyIdentityScope(tx).Limit(1).Find(&marker)
	if result.Error != nil {
		return app_errors.ParseDBError(result.Error)
	}
	if result.RowsAffected != 0 {
		if marker.Value != identity {
			return fmt.Errorf("master key does not match the database identity; restore the original custodied key or verified backup: %w", app_errors.ErrInternalServer)
		}
		return nil
	}
	if err := s.verifyPreexistingCiphertext(tx); err != nil {
		return err
	}
	if err := tx.Create(&models.SystemSetting{Key: masterKeyIdentitySetting, Value: identity}).Error; err != nil {
		return app_errors.ParseDBError(err)
	}
	return nil
}

func (s *Service) verifyPreexistingCiphertext(tx *gorm.DB) error {
	// The migration marker predates M2. On an existing database, authenticate
	// actual ciphertext before recording an identity for an already-custodied key.
	for _, source := range []struct{ table, column, condition string }{
		{"access_keys", "key_value", "key_value <> ''"},
		{"credentials", "data", "data <> ''"},
		{"credential_stages", "encrypted_payload", "encrypted_payload <> ''"},
		{"groups", "proxy_config", "proxy_config IS NOT NULL AND proxy_config <> ''"},
		{"credentials", "proxy_config", "proxy_config IS NOT NULL AND proxy_config <> ''"},
	} {
		var row struct {
			Value string `gorm:"column:encrypted_value"`
		}
		result := tx.Table(source.table).Select(source.column + " AS encrypted_value").
			Where(source.condition).Limit(1).Find(&row)
		if result.Error != nil {
			return app_errors.ParseDBError(result.Error)
		}
		if result.RowsAffected != 0 {
			return s.authenticateLegacyCiphertext(row.Value)
		}
	}
	var setting models.SystemSetting
	result := legacyEncryptedSettingScope(tx).Limit(1).Find(&setting)
	if result.Error != nil {
		return app_errors.ParseDBError(result.Error)
	}
	if result.RowsAffected != 0 {
		return s.authenticateLegacyCiphertext(setting.Value)
	}
	return nil
}

func (s *Service) authenticateLegacyCiphertext(value string) error {
	_, err := s.encryption.Decrypt(value)
	if err != nil {
		return fmt.Errorf("existing encrypted state cannot be read by the custodied master key; restore the original key before startup: %w", app_errors.ErrInternalServer)
	}
	return nil
}

func masterKeyIdentityScope(tx *gorm.DB) *gorm.DB {
	return tx.Where(&models.SystemSetting{Key: masterKeyIdentitySetting})
}

func legacyEncryptedSettingScope(tx *gorm.DB) *gorm.DB {
	return tx.Where(map[string]any{"key": []string{outboundproxy.SystemSettingKey, automodel.SettingKey}})
}
