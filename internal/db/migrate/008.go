package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 8,
		Up:      migrateStatsHourlyCompositePrimaryKey,
	})
}

func migrateStatsHourlyCompositePrimaryKey(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable("stats_hourlies") {
		return nil
	}

	switch db.Dialector.Name() {
	case "sqlite":
		return migrateStatsHourlyCompositePrimaryKeySQLite(db)
	case "mysql":
		return migrateStatsHourlyCompositePrimaryKeyMySQL(db)
	case "postgres", "postgresql":
		return migrateStatsHourlyCompositePrimaryKeyPostgres(db)
	default:
		return nil
	}
}

func migrateStatsHourlyCompositePrimaryKeySQLite(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
CREATE TABLE IF NOT EXISTS stats_hourlies_new (
    hour INTEGER NOT NULL,
    date TEXT NOT NULL,
    input_token BIGINT,
    output_token BIGINT,
    input_cost REAL,
    output_cost REAL,
    wait_time BIGINT,
    request_success BIGINT,
    request_failed BIGINT,
    PRIMARY KEY (hour, date)
)`).Error; err != nil {
			return fmt.Errorf("create stats_hourlies_new: %w", err)
		}

		if err := tx.Exec(`
INSERT OR REPLACE INTO stats_hourlies_new (hour, date, input_token, output_token, input_cost, output_cost, wait_time, request_success, request_failed)
SELECT hour, date, input_token, output_token, input_cost, output_cost, wait_time, request_success, request_failed
FROM stats_hourlies
WHERE hour BETWEEN 0 AND 23 AND date IS NOT NULL AND TRIM(date) != ''`).Error; err != nil {
			return fmt.Errorf("copy stats_hourlies: %w", err)
		}

		if err := tx.Exec(`DROP TABLE stats_hourlies`).Error; err != nil {
			return fmt.Errorf("drop old stats_hourlies: %w", err)
		}
		if err := tx.Exec(`ALTER TABLE stats_hourlies_new RENAME TO stats_hourlies`).Error; err != nil {
			return fmt.Errorf("rename stats_hourlies_new: %w", err)
		}
		return nil
	})
}

func migrateStatsHourlyCompositePrimaryKeyMySQL(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		_ = tx.Exec("ALTER TABLE stats_hourlies DROP PRIMARY KEY").Error
		if err := tx.Exec("ALTER TABLE stats_hourlies ADD PRIMARY KEY (hour, date)").Error; err != nil {
			return fmt.Errorf("alter stats_hourlies primary key: %w", err)
		}
		return nil
	})
}

func migrateStatsHourlyCompositePrimaryKeyPostgres(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		_ = tx.Exec("ALTER TABLE stats_hourlies DROP CONSTRAINT IF EXISTS stats_hourlies_pkey").Error
		if err := tx.Exec("ALTER TABLE stats_hourlies ADD PRIMARY KEY (hour, date)").Error; err != nil {
			return fmt.Errorf("alter stats_hourlies primary key: %w", err)
		}
		return nil
	})
}
