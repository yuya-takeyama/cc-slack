package slack

import (
	"context"

	"github.com/yuya-takeyama/cc-slack/internal/db"
)

// WorktreeManager interface for managing worktrees
type WorktreeManager interface {
	CreateWorktree(ctx context.Context, threadID int64, repositoryPath, repositoryName, baseBranch string) (*db.Worktree, error)
	GetWorktreePath(ctx context.Context, threadID int64) (string, error)
}

// RepositoryRouter interface for routing to repositories
type RepositoryRouter interface {
	Route(ctx context.Context, channelID, message string) (*RouteResult, error)
}

// RouteResult represents a routing result
type RouteResult struct {
	RepositoryPath string `json:"repository_path"`
	RepositoryName string `json:"repository_name"`
	Confidence     string `json:"confidence"`
	Reason         string `json:"reason"`
}
