package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestWorktreeWrapper_CreateAndRemove(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping git integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	wrapper := NewWorktreeWrapper(repoPath)

	// Test create worktree
	worktreePath := filepath.Join(filepath.Dir(repoPath), "test-worktree")
	err := wrapper.CreateWorktree(ctx, worktreePath, "HEAD")
	require.NoError(t, err)

	// Verify worktree exists
	assert.DirExists(t, worktreePath)

	// List worktrees
	worktrees, err := wrapper.ListWorktrees(ctx)
	require.NoError(t, err)
	assert.Len(t, worktrees, 2) // Main repo + new worktree

	// Get current branch
	branch, err := wrapper.GetCurrentBranch(ctx, worktreePath)
	require.NoError(t, err)
	assert.Contains(t, branch, "cc-slack-worktree-")

	// Remove worktree
	err = wrapper.RemoveWorktree(ctx, worktreePath)
	require.NoError(t, err)

	// Verify worktree is removed
	assert.NoDirExists(t, worktreePath)

	// List worktrees again
	worktrees, err = wrapper.ListWorktrees(ctx)
	require.NoError(t, err)
	assert.Len(t, worktrees, 1) // Only main repo
}

func TestParseWorktreeList(t *testing.T) {
	output := `worktree /path/to/main
HEAD abc123def456
branch refs/heads/main

worktree /path/to/feature
HEAD 789xyz012345
branch refs/heads/feature-branch
`

	worktrees := parseWorktreeList(output)

	assert.Len(t, worktrees, 2)

	assert.Equal(t, "/path/to/main", worktrees[0].Path)
	assert.Equal(t, "abc123def456", worktrees[0].Commit)
	assert.Equal(t, "main", worktrees[0].Branch)

	assert.Equal(t, "/path/to/feature", worktrees[1].Path)
	assert.Equal(t, "789xyz012345", worktrees[1].Commit)
	assert.Equal(t, "feature-branch", worktrees[1].Branch)
}

func TestValidateRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping git integration test in short mode")
	}

	// Test valid repository
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	err := ValidateRepository(repoPath)
	assert.NoError(t, err)

	// Test invalid path
	err = ValidateRepository("/tmp/nonexistent-repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestGetDefaultBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping git integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// The repo is already on main branch from init
	branch, err := GetDefaultBranch(ctx, repoPath)
	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}
