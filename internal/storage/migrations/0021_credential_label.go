package migrations

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const ID0021 = "0021_credential_label"

type credentialLabel0021 struct {
	Label string `gorm:"column:label;type:varchar(255);not null;default:''"`
}

func (credentialLabel0021) TableName() string { return "credentials" }

func Up0021(db *gorm.DB) error {
	model := &credentialLabel0021{}
	if !db.Migrator().HasTable(model) {
		return fmt.Errorf("add credential label: credentials table is missing")
	}
	if !db.Migrator().HasColumn(model, "label") {
		if err := db.Migrator().AddColumn(model, "Label"); err != nil {
			return fmt.Errorf("add credentials.label: %w", err)
		}
	}
	return Validate0021(db)
}

func ValidateRecoverable0021(db *gorm.DB) error {
	model := &credentialLabel0021{}
	if !db.Migrator().HasTable(model) {
		return fmt.Errorf("validate credential label: credentials table is missing")
	}
	if !db.Migrator().HasColumn(model, "label") {
		return nil
	}
	return Validate0021(db)
}

func Validate0021(db *gorm.DB) error {
	columns, err := db.Migrator().ColumnTypes("credentials")
	if err != nil {
		return fmt.Errorf("inspect credentials.label: %w", err)
	}
	for _, column := range columns {
		if !strings.EqualFold(column.Name(), "label") {
			continue
		}
		if !strings.Contains(strings.ToLower(column.DatabaseTypeName()), "char") {
			return fmt.Errorf("credentials.label must be varchar")
		}
		if length, known := column.Length(); known && length != 255 {
			return fmt.Errorf("credentials.label length must be 255")
		}
		if nullable, known := column.Nullable(); known && nullable {
			return fmt.Errorf("credentials.label must be non-null")
		}
		return nil
	}
	return fmt.Errorf("credentials.label is missing")
}
