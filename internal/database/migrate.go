package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Migrate runs database migrations
func Migrate(db *sql.DB, migrationsPath string) error {
	// Create migrations table if not exists
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of migration files
	files, err := getMigrationFiles(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	// Run each migration
	for _, file := range files {
		if strings.HasSuffix(file, ".up.sql") {
			version := strings.TrimSuffix(file, ".up.sql")

			// Check if migration already applied
			applied, err := isMigrationApplied(db, version)
			if err != nil {
				return fmt.Errorf("failed to check migration status: %w", err)
			}

			if applied {
				continue
			}

			// Read migration file
			content, err := os.ReadFile(filepath.Join(migrationsPath, file))
			if err != nil {
				return fmt.Errorf("failed to read migration file %s: %w", file, err)
			}

			// Execute migration
			if err := executeMigration(db, version, string(content)); err != nil {
				return fmt.Errorf("failed to execute migration %s: %w", file, err)
			}
		}
	}

	return nil
}

func createMigrationsTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)
	`
	_, err := db.Exec(query)
	return err
}

func getMigrationFiles(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}

	// Sort files to ensure consistent order
	sort.Strings(files)
	return files, nil
}

func isMigrationApplied(db *sql.DB, version string) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM schema_migrations WHERE version = ?"
	err := db.QueryRow(query, version).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func executeMigration(db *sql.DB, version, content string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute migration
	if _, err := tx.Exec(content); err != nil {
		return err
	}

	// Record migration
	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		return err
	}

	return tx.Commit()
}

// MustMigrate runs migrations and panics on error
func MustMigrate(db *sql.DB, migrationsPath string) {
	if err := Migrate(db, migrationsPath); err != nil {
		panic(fmt.Sprintf("failed to run migrations: %v", err))
	}
}
