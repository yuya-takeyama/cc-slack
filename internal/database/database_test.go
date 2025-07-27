package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		cleanup func(string)
		wantErr bool
	}{
		{
			name: "successful connection with new database",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "test.db")
			},
			cleanup: func(dbPath string) {},
			wantErr: false,
		},
		{
			name: "creates directory if not exists",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "subdir", "test.db")
			},
			cleanup: func(dbPath string) {},
			wantErr: false,
		},
		{
			name: "connects to existing database",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.db")
				// Create database first
				db, err := Open(dbPath)
				if err != nil {
					t.Fatalf("failed to create initial database: %v", err)
				}
				db.Close()
				return dbPath
			},
			cleanup: func(dbPath string) {},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := tt.setup(t)
			defer tt.cleanup(dbPath)

			db, err := Open(dbPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Open() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				defer db.Close()

				// Verify database is actually working
				if err := db.Ping(); err != nil {
					t.Errorf("Failed to ping database: %v", err)
				}

				// Verify foreign keys are enabled
				var foreignKeys int
				err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
				if err != nil {
					t.Errorf("Failed to check foreign keys: %v", err)
				}
				if foreignKeys != 1 {
					t.Errorf("Foreign keys not enabled, got %d", foreignKeys)
				}

				// Verify WAL mode is enabled
				var journalMode string
				err = db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
				if err != nil {
					t.Errorf("Failed to check journal mode: %v", err)
				}
				if journalMode != "wal" {
					t.Errorf("WAL mode not enabled, got %s", journalMode)
				}
			}
		})
	}
}

func TestOpenPermissions(t *testing.T) {
	// Test that database directory is created with correct permissions
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Check directory permissions
	dirInfo, err := os.Stat(filepath.Dir(dbPath))
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	mode := dirInfo.Mode()
	if mode.Perm() != 0755 {
		t.Errorf("Directory permissions = %v, want 0755", mode.Perm())
	}
}
