package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeWrapper provides Git worktree operations
type WorktreeWrapper struct {
	repoPath string
}

// NewWorktreeWrapper creates a new worktree wrapper
func NewWorktreeWrapper(repoPath string) *WorktreeWrapper {
	return &WorktreeWrapper{
		repoPath: repoPath,
	}
}

// CreateWorktree creates a new Git worktree
func (w *WorktreeWrapper) CreateWorktree(ctx context.Context, worktreePath, baseBranch string) error {
	// Ensure parent directory exists
	parentDir := filepath.Dir(worktreePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create worktree
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", generateBranchName(), worktreePath, baseBranch)
	cmd.Dir = w.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree: %w, output: %s", err, string(output))
	}

	return nil
}

// RemoveWorktree removes a Git worktree
func (w *WorktreeWrapper) RemoveWorktree(ctx context.Context, worktreePath string) error {
	// First, remove the worktree reference
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = w.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// If worktree is already removed, clean up the reference
		if strings.Contains(string(output), "is not a working tree") {
			return w.pruneWorktrees(ctx)
		}
		return fmt.Errorf("failed to remove worktree: %w, output: %s", err, string(output))
	}

	// Ensure directory is removed
	if err := os.RemoveAll(worktreePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove worktree directory: %w", err)
	}

	return nil
}

// ListWorktrees lists all worktrees for the repository
func (w *WorktreeWrapper) ListWorktrees(ctx context.Context) ([]WorktreeInfo, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = w.repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return parseWorktreeList(string(output)), nil
}

// GetCurrentBranch returns the current branch of a worktree
func (w *WorktreeWrapper) GetCurrentBranch(ctx context.Context, worktreePath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = worktreePath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// CheckoutBranch checks out a branch in a worktree
func (w *WorktreeWrapper) CheckoutBranch(ctx context.Context, worktreePath, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", branch)
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to checkout branch: %w, output: %s", err, string(output))
	}

	return nil
}

// PruneWorktrees removes stale worktree references
func (w *WorktreeWrapper) pruneWorktrees(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	cmd.Dir = w.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to prune worktrees: %w, output: %s", err, string(output))
	}

	return nil
}

// WorktreeInfo contains information about a worktree
type WorktreeInfo struct {
	Path   string
	Branch string
	Commit string
}

// parseWorktreeList parses the output of git worktree list --porcelain
func parseWorktreeList(output string) []WorktreeInfo {
	var worktrees []WorktreeInfo
	lines := strings.Split(output, "\n")

	var current WorktreeInfo
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = WorktreeInfo{
				Path: strings.TrimPrefix(line, "worktree "),
			}
		} else if strings.HasPrefix(line, "HEAD ") {
			current.Commit = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") {
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}

	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees
}

// generateBranchName generates a unique branch name for worktree
func generateBranchName() string {
	return fmt.Sprintf("cc-slack-worktree-%d", os.Getpid())
}

// ValidateRepository checks if the path is a valid Git repository
func ValidateRepository(path string) error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("not a git repository: %s", path)
	}

	gitDir := strings.TrimSpace(string(output))
	if gitDir == "" {
		return fmt.Errorf("invalid git repository: %s", path)
	}

	return nil
}

// GetDefaultBranch returns the default branch of a repository
func GetDefaultBranch(ctx context.Context, repoPath string) (string, error) {
	// Try to get the default branch from origin
	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(output))
		branch = strings.TrimPrefix(branch, "refs/remotes/origin/")
		if branch != "" {
			return branch, nil
		}
	}

	// Fallback: try common default branch names
	branches := []string{"main", "master", "develop"}
	for _, branch := range branches {
		cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", fmt.Sprintf("refs/heads/%s", branch))
		cmd.Dir = repoPath

		if err := cmd.Run(); err == nil {
			return branch, nil
		}
	}

	// Last resort: get the current branch
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath

	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to determine default branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
