package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Before SQLite rebuilds a table referenced by foreign keys, disable foreign-key enforcement outside the transaction to prevent DROP cascading child-table data.
// The full migration chain still runs serially in BEGIN IMMEDIATE and verifies all foreign keys before commit.
func applySQLiteMigrationRegistry(db *gorm.DB, entries []migration) error {
	return db.Connection(func(connection *gorm.DB) (resultErr error) {
		conn, ok := connection.Statement.ConnPool.(*sql.Conn)
		if !ok {
			return fmt.Errorf("SQLite migration connection is not pinned")
		}
		ctx := context.Background()
		var foreignKeys int
		if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			return err
		}
		active := false
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var cleanupErr error
			if active {
				_, cleanupErr = conn.ExecContext(cleanup, "ROLLBACK")
			}
			restore := "PRAGMA foreign_keys = OFF"
			if foreignKeys != 0 {
				restore = "PRAGMA foreign_keys = ON"
			}
			_, err := conn.ExecContext(cleanup, restore)
			cleanupErr = errors.Join(cleanupErr, err)
			var restored int
			if err := conn.QueryRowContext(cleanup, "PRAGMA foreign_keys").Scan(&restored); err != nil {
				cleanupErr = errors.Join(cleanupErr, err)
			} else if restored != foreignKeys {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("restore SQLite foreign key enforcement failed"))
			}
			if cleanupErr != nil {
				// Discard a connection whose cleanup failed; do not return unknown transaction or foreign-key state to the pool.
				if err := conn.Raw(func(any) error { return driver.ErrBadConn }); err != nil && !errors.Is(err, driver.ErrBadConn) {
					cleanupErr = errors.Join(cleanupErr, err)
				}
			}
			resultErr = errors.Join(resultErr, cleanupErr)
		}()
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			return err
		}
		active = true
		tx := connection.Session(&gorm.Session{NewDB: true, SkipDefaultTransaction: true, Context: ctx})
		if err := applyMigrationsLocked(tx, entries, false); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return err
		}
		active = false
		return nil
	})
}
