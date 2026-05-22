package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 9,
		Up: func(db *gorm.DB) error {
		hasColumn := func(table, column string) (bool, error) {
			return db.Migrator().HasColumn(table, column), nil
		}

		exists, err := hasColumn("groups", "endpoint_provider")
		if err != nil {
			return fmt.Errorf("failed to inspect groups.endpoint_provider: %w", err)
		}
		if exists {
			return nil
		}

		if err := db.Exec("ALTER TABLE groups ADD COLUMN endpoint_provider TEXT NOT NULL DEFAULT ''").Error; err != nil {
			return fmt.Errorf("failed to add groups.endpoint_provider: %w", err)
		}
		return nil
		},
	})
}
