package worktree

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/database"
	"github.com/yuya-takeyama/cc-slack/internal/db"
	"github.com/yuya-takeyama/cc-slack/internal/repository"
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

func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	// Create temporary directory
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	// Initialize git repo
	require.NoError(t, os.MkdirAll(repoPath, 0755))

	cmd := exec.Command("git", "init", "--initial-branch=main")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	// Create initial commit
	testFile := filepath.Join(repoPath, "README.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Test Repo"), 0644))

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "-c", "user.email=test@example.com", "-c", "user.name=Test User", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cleanup := func() {
		// Cleanup is handled by t.TempDir()
	}

	return repoPath, cleanup
}

func TestManager_CreateAndGetWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping git integration test in short mode")
	}

	sqlDB := setupTestDB(t)
	defer sqlDB.Close()

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	logger := zerolog.Nop()
	cfg := &config.Config{
		WorkingDirectories: config.WorkingDirectoriesConfig{
			WorktreeDirectory: ".test-worktrees",
		},
	}

	// Create repository and thread
	repoManager := repository.NewManager(sqlDB)
	ctx := context.Background()

	repo, err := repoManager.Create(ctx, repository.CreateParams{
		Name:          "test-repo",
		Path:          repoPath,
		DefaultBranch: "main",
	})
	require.NoError(t, err)

	// Create thread
	queries := db.New(sqlDB)
	thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
		ChannelID:        "C123",
		ThreadTs:         "123.456",
		WorkingDirectory: "",
		RepositoryID:     sql.NullInt64{Int64: repo.ID, Valid: true},
	})
	require.NoError(t, err)

	// Create worktree manager
	manager := NewManager(logger, cfg, sqlDB)

	// Create worktree
	worktree, err := manager.CreateWorktree(ctx, thread.ID, repo.ID, "main")
	require.NoError(t, err)
	assert.Equal(t, repo.ID, worktree.RepositoryID)
	assert.Equal(t, thread.ID, worktree.ThreadID)
	assert.Equal(t, "main", worktree.BaseBranch)
	assert.Equal(t, "active", worktree.Status)
	assert.Contains(t, worktree.Path, "test-worktrees")
	assert.DirExists(t, worktree.Path)

	// Get worktree by thread ID
	gotWorktree, err := manager.GetWorktreeByThreadID(ctx, thread.ID)
	require.NoError(t, err)
	require.NotNil(t, gotWorktree)
	assert.Equal(t, worktree.ID, gotWorktree.ID)

	// Get worktree path
	path, err := manager.GetWorktreePath(ctx, thread.ID)
	require.NoError(t, err)
	assert.Equal(t, worktree.Path, path)

	// Remove worktree
	err = manager.RemoveWorktree(ctx, worktree.ID)
	require.NoError(t, err)

	// Verify worktree is marked as deleted
	updatedWorktree, err := queries.GetWorktree(ctx, worktree.ID)
	require.NoError(t, err)
	assert.Equal(t, "deleted", updatedWorktree.Status)
}

func TestManager_GenerateWorktreePath(t *testing.T) {
	manager := &Manager{
		config: &config.Config{
			WorkingDirectories: config.WorkingDirectoriesConfig{
				WorktreeDirectory: "/tmp/worktrees",
			},
		},
	}

	path := manager.generateWorktreePath("myrepo", 123)
	assert.Contains(t, path, "/tmp/worktrees/myrepo-thread-123-")
	assert.Contains(t, path, time.Now().Format("20060102"))
}

func TestManager_CleanupOldWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	sqlDB := setupTestDB(t)
	defer sqlDB.Close()

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	logger := zerolog.Nop()
	cfg := &config.Config{
		WorkingDirectories: config.WorkingDirectoriesConfig{
			WorktreeDirectory:       ".test-worktrees",
			WorktreeRetentionPeriod: "1s", // Very short for testing
		},
	}

	// Create repository
	repoManager := repository.NewManager(sqlDB)
	ctx := context.Background()

	repo, err := repoManager.Create(ctx, repository.CreateParams{
		Name: "test-repo",
		Path: repoPath,
	})
	require.NoError(t, err)

	// Create old worktree
	queries := db.New(sqlDB)

	// Create thread
	thread, err := queries.CreateThread(ctx, db.CreateThreadParams{
		ChannelID:        "C123",
		ThreadTs:         "123.456",
		WorkingDirectory: "",
		RepositoryID:     sql.NullInt64{Int64: repo.ID, Valid: true},
	})
	require.NoError(t, err)

	// Create worktree manager
	manager := NewManager(logger, cfg, sqlDB)

	// Create worktree
	worktree, err := manager.CreateWorktree(ctx, thread.ID, repo.ID, "main")
	require.NoError(t, err)

	// Wait for retention period
	time.Sleep(2 * time.Second)

	// Run cleanup
	err = manager.CleanupOldWorktrees(ctx)
	require.NoError(t, err)

	// Verify worktree is deleted
	updatedWorktree, err := queries.GetWorktree(ctx, worktree.ID)
	require.NoError(t, err)
	assert.Equal(t, "deleted", updatedWorktree.Status)
}
