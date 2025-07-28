package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuya-takeyama/cc-slack/internal/database"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	migrationsPath := filepath.Join("..", "..", "migrations")
	err = database.Migrate(db, migrationsPath)
	require.NoError(t, err)

	return db
}

func TestManager_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewManager(db)
	ctx := context.Background()

	params := CreateParams{
		Name:           "test-repo",
		Path:           "/home/test/repo",
		DefaultBranch:  "main",
		ChannelID:      "C123456",
		SlackUsername:  "test-bot",
		SlackIconEmoji: ":robot_face:",
	}

	created, err := manager.Create(ctx, params)
	require.NoError(t, err)
	assert.Equal(t, "test-repo", created.Name)
	assert.Equal(t, "/home/test/repo", created.Path)
	assert.Equal(t, "main", created.DefaultBranch.String)

	got, err := manager.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Name, got.Name)

	gotByName, err := manager.GetByName(ctx, "test-repo")
	require.NoError(t, err)
	assert.Equal(t, created.ID, gotByName.ID)

	gotByChannel, err := manager.GetByChannelID(ctx, "C123456")
	require.NoError(t, err)
	assert.Equal(t, created.ID, gotByChannel.ID)
}

func TestManager_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewManager(db)
	ctx := context.Background()

	_, err := manager.Create(ctx, CreateParams{
		Name:      "repo1",
		Path:      "/path/to/repo1",
		ChannelID: "C123456",
	})
	require.NoError(t, err)

	_, err = manager.Create(ctx, CreateParams{
		Name:      "repo2",
		Path:      "/path/to/repo2",
		ChannelID: "C123456",
	})
	require.NoError(t, err)

	_, err = manager.Create(ctx, CreateParams{
		Name:      "repo3",
		Path:      "/path/to/repo3",
		ChannelID: "C789012",
	})
	require.NoError(t, err)

	allRepos, err := manager.List(ctx)
	require.NoError(t, err)
	assert.Len(t, allRepos, 3)

	channelRepos, err := manager.ListByChannelID(ctx, "C123456")
	require.NoError(t, err)
	assert.Len(t, channelRepos, 2)
}

func TestManager_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewManager(db)
	ctx := context.Background()

	created, err := manager.Create(ctx, CreateParams{
		Name: "original",
		Path: "/original/path",
	})
	require.NoError(t, err)

	updated, err := manager.Update(ctx, UpdateParams{
		ID:            created.ID,
		Name:          "updated",
		Path:          "/updated/path",
		DefaultBranch: "develop",
	})
	require.NoError(t, err)
	assert.Equal(t, "updated", updated.Name)
	assert.Equal(t, "/updated/path", updated.Path)
	assert.Equal(t, "develop", updated.DefaultBranch.String)
}

func TestManager_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewManager(db)
	ctx := context.Background()

	created, err := manager.Create(ctx, CreateParams{
		Name: "to-delete",
		Path: "/path/to/delete",
	})
	require.NoError(t, err)

	err = manager.Delete(ctx, created.ID)
	require.NoError(t, err)

	_, err = manager.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestManager_InferRepositoryFromMessage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	manager := NewManager(db)
	ctx := context.Background()

	repo1, err := manager.Create(ctx, CreateParams{
		Name:      "frontend",
		Path:      "/path/to/frontend",
		ChannelID: "C123456",
	})
	require.NoError(t, err)

	repo2, err := manager.Create(ctx, CreateParams{
		Name:      "backend",
		Path:      "/path/to/backend",
		ChannelID: "C123456",
	})
	require.NoError(t, err)

	tests := []struct {
		name       string
		message    string
		wantRepoID int64
		wantNil    bool
	}{
		{
			name:       "explicit frontend mention",
			message:    "Please fix the bug in frontend",
			wantRepoID: repo1.ID,
		},
		{
			name:       "explicit backend mention",
			message:    "Add new API to backend service",
			wantRepoID: repo2.ID,
		},
		{
			name:       "case insensitive",
			message:    "Update FRONTEND dependencies",
			wantRepoID: repo1.ID,
		},
		{
			name:    "no repository mention",
			message: "General question about the project",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := manager.InferRepositoryFromMessage(ctx, "C123456", tt.message)
			require.NoError(t, err)

			if tt.wantNil {
				assert.Equal(t, int64(0), got.ID)
			} else {
				assert.Equal(t, tt.wantRepoID, got.ID)
			}
		})
	}
}

func TestManager_ValidatePath(t *testing.T) {
	manager := &Manager{}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid absolute path",
			path:    "/home/user/repo",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "relative path",
			path:    "relative/path",
			wantErr: true,
		},
		{
			name:    "current directory",
			path:    "./current",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidatePath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
