package slack

import (
	"context"

	"github.com/yuya-takeyama/cc-slack/internal/db"
)

// RepositoryManager interface for managing repositories
type RepositoryManager interface {
	ListByChannelID(ctx context.Context, channelID string) ([]db.Repository, error)
	GetByID(ctx context.Context, id int64) (db.Repository, error)
}

// WorktreeManager interface for managing worktrees
type WorktreeManager interface {
	CreateWorktree(ctx context.Context, threadID int64, repositoryID int64, baseBranch string) (*db.Worktree, error)
	GetWorktreePath(ctx context.Context, threadID int64) (string, error)
}

// RepositoryRouter interface for routing to repositories
type RepositoryRouter interface {
	Route(ctx context.Context, channelID, message string) (*RouteResult, error)
}

// RouteResult represents a routing result
type RouteResult struct {
	RepositoryID   int64
	RepositoryName string
	Confidence     string
	Reason         string
}
