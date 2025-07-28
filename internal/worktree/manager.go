package worktree

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/db"
	"github.com/yuya-takeyama/cc-slack/internal/git"
)

// Manager manages Git worktrees for threads
type Manager struct {
	logger      zerolog.Logger
	config      *config.Config
	queries     *db.Queries
	database    *sql.DB
	mu          sync.RWMutex
	wrappers    map[int64]*git.WorktreeWrapper // repository ID -> wrapper
	stopCleanup chan struct{}
	cleanupDone chan struct{}
}

// NewManager creates a new worktree manager
func NewManager(logger zerolog.Logger, cfg *config.Config, database *sql.DB) *Manager {
	return &Manager{
		logger:      logger,
		config:      cfg,
		queries:     db.New(database),
		database:    database,
		wrappers:    make(map[int64]*git.WorktreeWrapper),
		stopCleanup: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}
}

// Start begins the cleanup goroutine
func (m *Manager) Start(ctx context.Context) {
	go m.cleanupLoop(ctx)
}

// Stop stops the cleanup goroutine
func (m *Manager) Stop() {
	close(m.stopCleanup)
	<-m.cleanupDone
}

// CreateWorktree creates a new worktree for a thread
func (m *Manager) CreateWorktree(ctx context.Context, threadID, repositoryID int64, baseBranch string) (*db.Worktree, error) {
	// Get repository information
	repo, err := m.queries.GetRepository(ctx, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	// Get or create wrapper for repository
	wrapper := m.getOrCreateWrapper(repositoryID, repo.Path)

	// Determine base branch
	if baseBranch == "" {
		if repo.DefaultBranch.Valid && repo.DefaultBranch.String != "" {
			baseBranch = repo.DefaultBranch.String
		} else {
			// Try to determine default branch
			defaultBranch, err := git.GetDefaultBranch(ctx, repo.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to determine default branch: %w", err)
			}
			baseBranch = defaultBranch
		}
	}

	// Generate worktree path
	worktreePath := m.generateWorktreePath(repo.Name, threadID)

	// Make absolute if relative
	if !filepath.IsAbs(worktreePath) {
		worktreePath = filepath.Join(repo.Path, worktreePath)
	}

	// Create physical worktree
	if err := wrapper.CreateWorktree(ctx, worktreePath, baseBranch); err != nil {
		return nil, fmt.Errorf("failed to create git worktree: %w", err)
	}

	// Get current branch name
	currentBranch, err := wrapper.GetCurrentBranch(ctx, worktreePath)
	if err != nil {
		// Cleanup on error
		_ = wrapper.RemoveWorktree(ctx, worktreePath)
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Create database record
	worktree, err := m.queries.CreateWorktree(ctx, db.CreateWorktreeParams{
		RepositoryID:  repositoryID,
		ThreadID:      threadID,
		Path:          worktreePath,
		BaseBranch:    baseBranch,
		CurrentBranch: sql.NullString{String: currentBranch, Valid: true},
		Status:        "active",
	})
	if err != nil {
		// Cleanup on error
		_ = wrapper.RemoveWorktree(ctx, worktreePath)
		return nil, fmt.Errorf("failed to create worktree record: %w", err)
	}

	m.logger.Info().
		Int64("thread_id", threadID).
		Int64("repository_id", repositoryID).
		Str("path", worktreePath).
		Str("base_branch", baseBranch).
		Msg("Created worktree")

	return &worktree, nil
}

// GetWorktreeByThreadID gets the worktree for a thread
func (m *Manager) GetWorktreeByThreadID(ctx context.Context, threadID int64) (*db.Worktree, error) {
	worktree, err := m.queries.GetWorktreeByThreadID(ctx, threadID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}
	return &worktree, nil
}

// RemoveWorktree removes a worktree
func (m *Manager) RemoveWorktree(ctx context.Context, worktreeID int64) error {
	// Get worktree info
	worktree, err := m.queries.GetWorktree(ctx, worktreeID)
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get repository info
	repo, err := m.queries.GetRepository(ctx, worktree.RepositoryID)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Get wrapper
	wrapper := m.getOrCreateWrapper(worktree.RepositoryID, repo.Path)

	// Remove physical worktree
	if err := wrapper.RemoveWorktree(ctx, worktree.Path); err != nil {
		m.logger.Error().Err(err).
			Str("path", worktree.Path).
			Msg("Failed to remove physical worktree")
	}

	// Update status in database
	if err := m.queries.UpdateWorktreeStatus(ctx, db.UpdateWorktreeStatusParams{
		Status:  "deleted",
		Column2: "deleted",
		ID:      worktreeID,
	}); err != nil {
		return fmt.Errorf("failed to update worktree status: %w", err)
	}

	m.logger.Info().
		Int64("worktree_id", worktreeID).
		Str("path", worktree.Path).
		Msg("Removed worktree")

	return nil
}

// CleanupOldWorktrees removes worktrees older than retention period
func (m *Manager) CleanupOldWorktrees(ctx context.Context) error {
	retentionPeriod := m.config.WorkingDirectories.WorktreeRetentionPeriod
	if retentionPeriod == "" {
		retentionPeriod = "24h"
	}

	duration, err := time.ParseDuration(retentionPeriod)
	if err != nil {
		return fmt.Errorf("invalid retention period: %w", err)
	}

	// SQLite datetime modifier format
	modifier := fmt.Sprintf("-%d seconds", int(duration.Seconds()))

	// Find old worktrees
	oldWorktrees, err := m.queries.ListOldWorktrees(ctx, modifier)
	if err != nil {
		return fmt.Errorf("failed to list old worktrees: %w", err)
	}

	var errors []error
	for _, worktree := range oldWorktrees {
		if err := m.RemoveWorktree(ctx, worktree.ID); err != nil {
			errors = append(errors, fmt.Errorf("failed to remove worktree %d: %w", worktree.ID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup completed with %d errors: %v", len(errors), errors)
	}

	return nil
}

// generateWorktreePath generates a path for a new worktree
func (m *Manager) generateWorktreePath(repoName string, threadID int64) string {
	baseDir := m.config.WorkingDirectories.WorktreeDirectory
	if baseDir == "" {
		baseDir = ".cc-slack-worktrees"
	}

	timestamp := time.Now().Format("20060102-150405")
	dirname := fmt.Sprintf("%s-thread-%d-%s", repoName, threadID, timestamp)

	return filepath.Join(baseDir, dirname)
}

// getOrCreateWrapper gets or creates a wrapper for a repository
func (m *Manager) getOrCreateWrapper(repositoryID int64, repoPath string) *git.WorktreeWrapper {
	m.mu.Lock()
	defer m.mu.Unlock()

	if wrapper, exists := m.wrappers[repositoryID]; exists {
		return wrapper
	}

	wrapper := git.NewWorktreeWrapper(repoPath)
	m.wrappers[repositoryID] = wrapper
	return wrapper
}

// cleanupLoop runs periodic cleanup
func (m *Manager) cleanupLoop(ctx context.Context) {
	defer close(m.cleanupDone)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCleanup:
			return
		case <-ticker.C:
			if err := m.CleanupOldWorktrees(ctx); err != nil {
				m.logger.Error().Err(err).Msg("Failed to cleanup old worktrees")
			}
		}
	}
}

// ValidateRepository validates that a repository path is valid
func (m *Manager) ValidateRepository(path string) error {
	return git.ValidateRepository(path)
}

// GetWorktreePath returns the working directory path for a thread
func (m *Manager) GetWorktreePath(ctx context.Context, threadID int64) (string, error) {
	worktree, err := m.GetWorktreeByThreadID(ctx, threadID)
	if err != nil {
		return "", err
	}
	if worktree == nil {
		return "", nil
	}
	return worktree.Path, nil
}
