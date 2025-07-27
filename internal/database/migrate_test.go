package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateMigrationsTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Test table creation
	err := createMigrationsTable(db)
	if err != nil {
		t.Fatalf("Failed to create migrations table: %v", err)
	}

	// Verify table exists
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&tableName)
	if err != nil {
		t.Fatalf("Failed to verify table existence: %v", err)
	}
	if tableName != "schema_migrations" {
		t.Errorf("Expected table name 'schema_migrations', got '%s'", tableName)
	}

	// Test idempotency - should not error when called again
	err = createMigrationsTable(db)
	if err != nil {
		t.Errorf("createMigrationsTable should be idempotent, got error: %v", err)
	}
}

func TestGetMigrationFiles(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) string
		wantFiles []string
		wantErr   bool
	}{
		{
			name: "directory with migration files",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				files := []string{
					"000001_init.up.sql",
					"000001_init.down.sql",
					"000002_add_users.up.sql",
					"000002_add_users.down.sql",
				}
				for _, f := range files {
					if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("-- test migration"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				return tmpDir
			},
			wantFiles: []string{
				"000001_init.down.sql",
				"000001_init.up.sql",
				"000002_add_users.down.sql",
				"000002_add_users.up.sql",
			},
			wantErr: false,
		},
		{
			name: "empty directory",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantFiles: []string{},
			wantErr:   false,
		},
		{
			name: "directory with non-sql files",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				files := []string{
					"000001_init.up.sql",
					"README.md",
					"test.txt",
				}
				for _, f := range files {
					if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("test"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				return tmpDir
			},
			wantFiles: []string{"000001_init.up.sql"},
			wantErr:   false,
		},
		{
			name: "non-existent directory",
			setup: func(t *testing.T) string {
				return "/non/existent/path"
			},
			wantFiles: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)

			files, err := getMigrationFiles(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("getMigrationFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(files) != len(tt.wantFiles) {
				t.Errorf("getMigrationFiles() got %d files, want %d", len(files), len(tt.wantFiles))
				return
			}

			for i, f := range files {
				if f != tt.wantFiles[i] {
					t.Errorf("getMigrationFiles()[%d] = %v, want %v", i, f, tt.wantFiles[i])
				}
			}
		})
	}
}

func TestIsMigrationApplied(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create migrations table
	if err := createMigrationsTable(db); err != nil {
		t.Fatal(err)
	}

	// Test non-applied migration
	applied, err := isMigrationApplied(db, "000001_init")
	if err != nil {
		t.Fatalf("Failed to check migration: %v", err)
	}
	if applied {
		t.Error("Expected migration to not be applied")
	}

	// Apply migration
	_, err = db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", "000001_init")
	if err != nil {
		t.Fatal(err)
	}

	// Test applied migration
	applied, err = isMigrationApplied(db, "000001_init")
	if err != nil {
		t.Fatalf("Failed to check migration: %v", err)
	}
	if !applied {
		t.Error("Expected migration to be applied")
	}
}

func TestExecuteMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create migrations table
	if err := createMigrationsTable(db); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		version string
		content string
		wantErr bool
	}{
		{
			name:    "valid migration",
			version: "000001_create_test_table",
			content: `CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT);`,
			wantErr: false,
		},
		{
			name:    "invalid SQL",
			version: "000002_invalid",
			content: `INVALID SQL STATEMENT;`,
			wantErr: true,
		},
		{
			name:    "empty migration",
			version: "000003_empty",
			content: ``,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executeMigration(db, tt.version, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeMigration() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				// Verify migration was recorded
				applied, err := isMigrationApplied(db, tt.version)
				if err != nil {
					t.Fatalf("Failed to check migration: %v", err)
				}
				if !applied {
					t.Error("Migration was not recorded")
				}
			}
		})
	}
}

func TestMigrate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test migrations directory
	migrationsDir := t.TempDir()

	// Create test migration files
	migrations := []struct {
		filename string
		content  string
	}{
		{
			filename: "000001_init.up.sql",
			content: `
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);`,
		},
		{
			filename: "000001_init.down.sql",
			content:  `DROP TABLE users;`,
		},
		{
			filename: "000002_add_email.up.sql",
			content:  `ALTER TABLE users ADD COLUMN email TEXT;`,
		},
		{
			filename: "000002_add_email.down.sql",
			content:  `ALTER TABLE users DROP COLUMN email;`,
		},
	}

	for _, m := range migrations {
		path := filepath.Join(migrationsDir, m.filename)
		if err := os.WriteFile(path, []byte(m.content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Run migrations
	err := Migrate(db, migrationsDir)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify tables were created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Error("Expected users table to be created")
	}

	// Verify column was added
	rows, err := db.Query("PRAGMA table_info(users)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, dtype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &dtype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatal(err)
		}
		columns[name] = true
	}

	if !columns["email"] {
		t.Error("Expected email column to be added")
	}

	// Test idempotency - running again should not error
	err = Migrate(db, migrationsDir)
	if err != nil {
		t.Errorf("Migrate should be idempotent, got error: %v", err)
	}
}

func TestMustMigrate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Test with valid migrations
	migrationsDir := t.TempDir()

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustMigrate panicked unexpectedly: %v", r)
		}
	}()

	MustMigrate(db, migrationsDir)

	// Test with invalid path - should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustMigrate should panic with invalid path")
		}
	}()

	MustMigrate(db, "/non/existent/path")
}

// Helper function to setup test database
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	return db
}
