package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/yuya-takeyama/cc-slack/internal/db"
)

type Manager struct {
	queries *db.Queries
}

func NewManager(database *sql.DB) *Manager {
	return &Manager{
		queries: db.New(database),
	}
}

func (m *Manager) GetByID(ctx context.Context, id int64) (db.Repository, error) {
	return m.queries.GetRepository(ctx, id)
}

func (m *Manager) GetByName(ctx context.Context, name string) (db.Repository, error) {
	return m.queries.GetRepositoryByName(ctx, name)
}

func (m *Manager) GetByChannelID(ctx context.Context, channelID string) (db.Repository, error) {
	return m.queries.GetRepositoryByChannelID(ctx, sql.NullString{
		String: channelID,
		Valid:  true,
	})
}

func (m *Manager) ListByChannelID(ctx context.Context, channelID string) ([]db.Repository, error) {
	return m.queries.ListRepositoriesByChannelID(ctx, sql.NullString{
		String: channelID,
		Valid:  true,
	})
}

func (m *Manager) List(ctx context.Context) ([]db.Repository, error) {
	return m.queries.ListRepositories(ctx)
}

type CreateParams struct {
	Name           string
	Path           string
	DefaultBranch  string
	ChannelID      string
	SlackUsername  string
	SlackIconEmoji string
	SlackIconURL   string
}

func (m *Manager) Create(ctx context.Context, params CreateParams) (db.Repository, error) {
	if params.DefaultBranch == "" {
		params.DefaultBranch = "main"
	}

	return m.queries.CreateRepository(ctx, db.CreateRepositoryParams{
		Name:           params.Name,
		Path:           params.Path,
		DefaultBranch:  sql.NullString{String: params.DefaultBranch, Valid: params.DefaultBranch != ""},
		SlackChannelID: sql.NullString{String: params.ChannelID, Valid: params.ChannelID != ""},
		SlackUsername:  sql.NullString{String: params.SlackUsername, Valid: params.SlackUsername != ""},
		SlackIconEmoji: sql.NullString{String: params.SlackIconEmoji, Valid: params.SlackIconEmoji != ""},
		SlackIconUrl:   sql.NullString{String: params.SlackIconURL, Valid: params.SlackIconURL != ""},
	})
}

type UpdateParams struct {
	ID             int64
	Name           string
	Path           string
	DefaultBranch  string
	ChannelID      string
	SlackUsername  string
	SlackIconEmoji string
	SlackIconURL   string
}

func (m *Manager) Update(ctx context.Context, params UpdateParams) (db.Repository, error) {
	return m.queries.UpdateRepository(ctx, db.UpdateRepositoryParams{
		ID:             params.ID,
		Name:           params.Name,
		Path:           params.Path,
		DefaultBranch:  sql.NullString{String: params.DefaultBranch, Valid: params.DefaultBranch != ""},
		SlackChannelID: sql.NullString{String: params.ChannelID, Valid: params.ChannelID != ""},
		SlackUsername:  sql.NullString{String: params.SlackUsername, Valid: params.SlackUsername != ""},
		SlackIconEmoji: sql.NullString{String: params.SlackIconEmoji, Valid: params.SlackIconEmoji != ""},
		SlackIconUrl:   sql.NullString{String: params.SlackIconURL, Valid: params.SlackIconURL != ""},
	})
}

func (m *Manager) Delete(ctx context.Context, id int64) error {
	return m.queries.DeleteRepository(ctx, id)
}

func (m *Manager) InferRepositoryFromMessage(ctx context.Context, channelID, message string) (db.Repository, error) {
	repos, err := m.ListByChannelID(ctx, channelID)
	if err != nil {
		return db.Repository{}, fmt.Errorf("failed to list repositories: %w", err)
	}

	if len(repos) == 0 {
		return db.Repository{}, nil
	}

	if len(repos) == 1 {
		return repos[0], nil
	}

	messageLower := strings.ToLower(message)
	for _, repo := range repos {
		if strings.Contains(messageLower, strings.ToLower(repo.Name)) {
			return repo, nil
		}
	}

	return db.Repository{}, nil
}

func (m *Manager) ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("repository path cannot be empty")
	}

	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("repository path must be absolute")
	}

	return nil
}
