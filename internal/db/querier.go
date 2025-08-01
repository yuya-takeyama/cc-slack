// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0

package db

import (
	"context"
)

type Querier interface {
	CountActiveSessionsByThread(ctx context.Context, threadID int64) (int64, error)
	CreateSessionWithInitialPrompt(ctx context.Context, arg CreateSessionWithInitialPromptParams) (Session, error)
	CreateThread(ctx context.Context, arg CreateThreadParams) (Thread, error)
	GetActiveSessionByThread(ctx context.Context, threadID int64) (Session, error)
	GetLatestSessionByThread(ctx context.Context, threadID int64) (Session, error)
	GetSession(ctx context.Context, sessionID string) (Session, error)
	GetThread(ctx context.Context, arg GetThreadParams) (Thread, error)
	GetThreadByID(ctx context.Context, id int64) (Thread, error)
	GetThreadByThreadTs(ctx context.Context, threadTs string) (Thread, error)
	ListActiveSessions(ctx context.Context) ([]Session, error)
	ListSessions(ctx context.Context) ([]Session, error)
	ListSessionsByThreadID(ctx context.Context, threadID int64) ([]Session, error)
	ListThreads(ctx context.Context) ([]Thread, error)
	UpdateSessionEndTime(ctx context.Context, arg UpdateSessionEndTimeParams) error
	UpdateSessionID(ctx context.Context, arg UpdateSessionIDParams) error
	UpdateSessionModel(ctx context.Context, arg UpdateSessionModelParams) error
	UpdateSessionOnComplete(ctx context.Context, arg UpdateSessionOnCompleteParams) error
	UpdateSessionStatus(ctx context.Context, arg UpdateSessionStatusParams) error
	UpdateThreadTimestamp(ctx context.Context, id int64) error
}

var _ Querier = (*Queries)(nil)
