package router

import (
	"context"
	"database/sql"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/db"
)

func TestRepositoryRouter_RouteWithSingleRepo(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.Config{
		WorkingDirectories: config.WorkingDirectoriesConfig{
			Default: "/tmp",
		},
	}

	repos := []db.Repository{
		{
			ID:   1,
			Name: "frontend",
			Path: "/path/to/frontend",
		},
	}

	router := NewRepositoryRouter(logger, cfg, repos)

	result, err := router.Route(context.Background(), "C123", "Fix the bug in login")
	require.NoError(t, err)

	assert.Equal(t, int64(1), result.RepositoryID)
	assert.Equal(t, "frontend", result.RepositoryName)
	assert.Equal(t, "high", result.Confidence)
	assert.Equal(t, "Only one repository configured for this channel", result.Reason)
}

func TestRepositoryRouter_BuildSystemPrompt(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.Config{}

	repos := []db.Repository{
		{
			ID:   1,
			Name: "frontend",
			Path: "/path/to/frontend",
			DefaultBranch: sql.NullString{
				String: "main",
				Valid:  true,
			},
		},
		{
			ID:   2,
			Name: "backend",
			Path: "/path/to/backend",
			DefaultBranch: sql.NullString{
				String: "develop",
				Valid:  true,
			},
		},
	}

	router := NewRepositoryRouter(logger, cfg, repos)
	prompt := router.buildSystemPrompt()

	assert.Contains(t, prompt, "ID: 1, Name: frontend, Path: /path/to/frontend, Default Branch: main")
	assert.Contains(t, prompt, "ID: 2, Name: backend, Path: /path/to/backend, Default Branch: develop")
	assert.Contains(t, prompt, "You are a repository router")
	assert.Contains(t, prompt, "You must respond with valid JSON only")
}

func TestRepositoryRouter_ValidateResult(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.Config{}

	repos := []db.Repository{
		{
			ID:   1,
			Name: "frontend",
			Path: "/path/to/frontend",
		},
		{
			ID:   2,
			Name: "backend",
			Path: "/path/to/backend",
		},
	}

	router := NewRepositoryRouter(logger, cfg, repos)

	tests := []struct {
		name    string
		result  RouteResult
		wantErr bool
	}{
		{
			name: "valid result",
			result: RouteResult{
				RepositoryID:   1,
				RepositoryName: "frontend",
				Confidence:     "high",
				Reason:         "Test reason",
			},
			wantErr: false,
		},
		{
			name: "mismatched name gets corrected",
			result: RouteResult{
				RepositoryID:   2,
				RepositoryName: "wrong-name",
				Confidence:     "medium",
				Reason:         "Test reason",
			},
			wantErr: false,
		},
		{
			name: "invalid repository ID",
			result: RouteResult{
				RepositoryID:   999,
				RepositoryName: "nonexistent",
				Confidence:     "low",
				Reason:         "Test reason",
			},
			wantErr: true,
		},
		{
			name: "invalid confidence gets defaulted",
			result: RouteResult{
				RepositoryID:   1,
				RepositoryName: "frontend",
				Confidence:     "invalid",
				Reason:         "Test reason",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.result
			err := router.validateResult(&result)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check corrections
				if tt.name == "mismatched name gets corrected" {
					assert.Equal(t, "backend", result.RepositoryName)
				}
				if tt.name == "invalid confidence gets defaulted" {
					assert.Equal(t, "medium", result.Confidence)
				}
			}
		})
	}
}
