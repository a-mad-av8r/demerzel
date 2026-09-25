package migrations

import (
	"fmt"
	"strings"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const ID0010 = "0010_model_cooldown"
const modelCooldownTable0010 = "request_log_attempts"
const modelCooldownEffect0010 = "chk_request_log_attempt_effect"
const modelCooldownDeadline0010 = "chk_request_log_attempt_cooldown"
const modelCooldownEffectExpression0010 = "effect IN ('','none','cooldown_credential','cooldown_model','record_credential_failure','skip_group') AND (effect <> 'cooldown_model' OR cooldown_until_ms IS NOT NULL)"

// Up0010 extends only the log contract; runtime model cooldown remains managed by Registry and checkpoints.
func Up0010(db *gorm.DB) error {
	if err := ValidateRecoverable0010(db); err != nil {
		return err
	}
	if Validate0010(db) == nil {
		return nil
	}
	if !db.Migrator().HasColumn(modelCooldownTable0010, "cooldown_until_ms") {
		if err := db.Exec("ALTER TABLE request_log_attempts ADD COLUMN cooldown_until_ms BIGINT NULL CONSTRAINT chk_request_log_attempt_cooldown CHECK (cooldown_until_ms IS NULL OR cooldown_until_ms >= 0)").Error; err != nil {
			return err
		}
	}
	if strings.EqualFold(db.Dialector.Name(), "sqlite") {
		// Changing a SQLite CHECK requires rebuilding the table, restoring every independent index from the original definition.
		var indexes []string
		if err := db.Raw("SELECT sql FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL", modelCooldownTable0010).Scan(&indexes).Error; err != nil {
			return err
		}
		if err := rebuildModelCooldownSQLite0010(db); err != nil {
			return err
		}
		for _, index := range indexes {
			if err := db.Exec(index).Error; err != nil {
				return err
			}
		}
	} else {
		drop := "DROP CONSTRAINT"
		if dialector, ok := db.Dialector.(*gormmysql.Dialector); ok && dialector.Config != nil &&
			mysqlRequiresCheckDropSyntax0003(dialector.ServerVersion) {
			drop = "DROP CHECK"
		}
		// Atomically replace within one DDL so a MySQL interruption cannot prevent recovery because the old migration constraint is absent.
		if err := db.Exec("ALTER TABLE request_log_attempts " + drop + " chk_request_log_attempt_effect, ADD CONSTRAINT chk_request_log_attempt_effect CHECK (" + modelCooldownEffectExpression0010 + ")").Error; err != nil {
			return err
		}
	}
	return Validate0010(db)
}

func rebuildModelCooldownSQLite0010(db *gorm.DB) error {
	var ddl string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?", modelCooldownTable0010).Scan(&ddl).Error; err != nil {
		return err
	}
	const old = "effect IN ('','none','cooldown_credential','record_credential_failure','skip_group')"
	if strings.Count(ddl, old) != 1 {
		return fmt.Errorf("unexpected prior model cooldown effect constraint")
	}
	start := strings.Index(ddl, "(")
	if start < 0 {
		return fmt.Errorf("invalid prior request attempts DDL")
	}
	ddl = "CREATE TABLE request_log_attempts__0010 " + strings.Replace(ddl[start:], old, modelCooldownEffectExpression0010, 1)
	columns, err := db.Migrator().ColumnTypes(modelCooldownTable0010)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		if strings.ContainsAny(column.Name(), "\"`\x00") {
			return fmt.Errorf("invalid attempt column")
		}
		names = append(names, `"`+column.Name()+`"`)
	}
	projection := strings.Join(names, ",")
	// The migration executor owns the write transaction, so do not BEGIN again through Migrator here.
	for _, statement := range []string{ddl,
		"INSERT INTO request_log_attempts__0010 (" + projection + ") SELECT " + projection + " FROM request_log_attempts",
		"DROP TABLE request_log_attempts", "ALTER TABLE request_log_attempts__0010 RENAME TO request_log_attempts"} {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

// ValidateRecoverable0010 accepts MySQL states where the column was added or constraints were atomically replaced.
func ValidateRecoverable0010(db *gorm.DB) error {
	if !db.Migrator().HasTable(modelCooldownTable0010) {
		return fmt.Errorf("model cooldown attempts table is missing")
	}
	columns, err := db.Migrator().ColumnTypes(modelCooldownTable0010)
	if err != nil {
		return err
	}
	hasColumn := false
	for _, column := range columns {
		if column.Name() != "cooldown_until_ms" {
			continue
		}
		hasColumn = true
		if !strings.Contains(strings.ToLower(column.DatabaseTypeName()), "int") {
			return fmt.Errorf("model cooldown deadline must be integer")
		}
		if nullable, known := column.Nullable(); !known || !nullable {
			return fmt.Errorf("model cooldown deadline must be nullable")
		}
	}
	if hasColumn && !db.Migrator().HasConstraint(modelCooldownTable0010, modelCooldownDeadline0010) {
		return fmt.Errorf("model cooldown deadline constraint is missing")
	}
	if !db.Migrator().HasConstraint(modelCooldownTable0010, modelCooldownEffect0010) {
		return fmt.Errorf("attempt effect constraint is missing")
	}
	return nil
}

func Validate0010(db *gorm.DB) error {
	if err := ValidateRecoverable0010(db); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(modelCooldownTable0010, "cooldown_until_ms") {
		return fmt.Errorf("model cooldown deadline column is missing")
	}
	definition, err := modelCooldownConstraint0010(db, modelCooldownEffect0010)
	if err != nil {
		return err
	}
	if !strings.Contains(definition, "cooldown_model") || !strings.Contains(strings.ToLower(definition), "is not null") {
		return fmt.Errorf("model cooldown effect constraint is invalid")
	}
	deadline, err := modelCooldownConstraint0010(db, modelCooldownDeadline0010)
	if err != nil {
		return err
	}
	normalized := strings.NewReplacer(" ", "", "\n", "", "`", "", `"`, "", "(", "", ")", "", "::bigint", "").Replace(strings.ToLower(deadline))
	if !strings.Contains(normalized, "cooldown_until_ms>=0") {
		return fmt.Errorf("model cooldown deadline bounds are invalid")
	}
	return Validate0006(db)
}

func modelCooldownConstraint0010(db *gorm.DB, name string) (string, error) {
	var definition string
	var err error
	switch strings.ToLower(db.Dialector.Name()) {
	case "sqlite":
		err = db.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?", modelCooldownTable0010).Scan(&definition).Error
	case "mysql":
		err = db.Raw("SELECT CHECK_CLAUSE FROM information_schema.check_constraints WHERE constraint_schema = DATABASE() AND constraint_name = ?", name).Scan(&definition).Error
	case "postgres", "postgresql":
		err = db.Raw("SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = ? AND conrelid = ?::regclass", name, modelCooldownTable0010).Scan(&definition).Error
	default:
		return "", fmt.Errorf("unsupported model cooldown migration driver %q", db.Dialector.Name())
	}
	if err != nil {
		return "", err
	}
	if definition == "" {
		return "", fmt.Errorf("model cooldown constraint %q is missing", name)
	}
	return definition, nil
}
