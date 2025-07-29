package web

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/yuya-takeyama/cc-slack/internal/config"
)

// RepositoryResponse represents a repository in the API response
type RepositoryResponse struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	DefaultBranch string `json:"default_branch"`
	ChannelID     string `json:"channel_id,omitempty"`
	Username      string `json:"username,omitempty"`
	IconEmoji     string `json:"icon_emoji,omitempty"`
	IconURL       string `json:"icon_url,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// RepositoriesResponse represents the repositories API response
type RepositoriesResponse struct {
	Repositories []RepositoryResponse `json:"repositories"`
}

// GetRepositories handles GET /api/repositories
func GetRepositories(w http.ResponseWriter, r *http.Request) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load configuration")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build response from config
	response := RepositoriesResponse{
		Repositories: make([]RepositoryResponse, 0, len(cfg.Repositories)),
	}

	for i, repo := range cfg.Repositories {
		repoResp := RepositoryResponse{
			ID:            int64(i + 1), // Generate ID based on index
			Name:          repo.Name,
			Path:          repo.Path,
			DefaultBranch: repo.DefaultBranch,
			CreatedAt:     "2024-01-01T00:00:00Z", // Static for now
			UpdatedAt:     "2024-01-01T00:00:00Z", // Static for now
		}

		// Handle channels
		if len(repo.Channels) > 0 {
			repoResp.ChannelID = repo.Channels[0] // Use first channel for display
		}

		// Handle Slack overrides
		if repo.SlackOverride != nil {
			repoResp.Username = repo.SlackOverride.Username
			repoResp.IconEmoji = repo.SlackOverride.IconEmoji
			repoResp.IconURL = repo.SlackOverride.IconURL
		}

		response.Repositories = append(response.Repositories, repoResp)
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
