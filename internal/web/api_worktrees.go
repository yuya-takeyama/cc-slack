package web

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

// WorktreeResponse represents a worktree in the API response
type WorktreeResponse struct {
	ID             int64  `json:"id"`
	RepositoryPath string `json:"repository_path"`
	RepositoryName string `json:"repository_name"`
	ThreadID       int64  `json:"thread_id"`
	Path           string `json:"path"`
	BaseBranch     string `json:"base_branch"`
	CurrentBranch  string `json:"current_branch,omitempty"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
	DeletedAt      string `json:"deleted_at,omitempty"`
}

// WorktreesResponse represents the worktrees API response
type WorktreesResponse struct {
	Worktrees []WorktreeResponse `json:"worktrees"`
}

// GetWorktrees handles GET /api/worktrees
func GetWorktrees(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Get all worktrees
	worktrees, err := queries.ListWorktrees(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list worktrees")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build response
	response := WorktreesResponse{
		Worktrees: make([]WorktreeResponse, 0, len(worktrees)),
	}

	for _, worktree := range worktrees {
		worktreeResp := WorktreeResponse{
			ID:             worktree.ID,
			RepositoryPath: worktree.RepositoryPath,
			RepositoryName: worktree.RepositoryName,
			ThreadID:       worktree.ThreadID,
			Path:           worktree.Path,
			BaseBranch:     worktree.BaseBranch,
			Status:         worktree.Status,
			CreatedAt:      worktree.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}

		if worktree.CurrentBranch.Valid {
			worktreeResp.CurrentBranch = worktree.CurrentBranch.String
		}

		if worktree.DeletedAt.Valid {
			worktreeResp.DeletedAt = worktree.DeletedAt.Time.Format("2006-01-02T15:04:05Z")
		}

		response.Worktrees = append(response.Worktrees, worktreeResp)
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// GetActiveWorktrees handles GET /api/worktrees/active
func GetActiveWorktrees(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Get all active worktrees
	worktrees, err := queries.ListActiveWorktrees(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list active worktrees")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build response
	response := WorktreesResponse{
		Worktrees: make([]WorktreeResponse, 0, len(worktrees)),
	}

	for _, worktree := range worktrees {
		worktreeResp := WorktreeResponse{
			ID:             worktree.ID,
			RepositoryPath: worktree.RepositoryPath,
			RepositoryName: worktree.RepositoryName,
			ThreadID:       worktree.ThreadID,
			Path:           worktree.Path,
			BaseBranch:     worktree.BaseBranch,
			Status:         worktree.Status,
			CreatedAt:      worktree.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}

		if worktree.CurrentBranch.Valid {
			worktreeResp.CurrentBranch = worktree.CurrentBranch.String
		}

		response.Worktrees = append(response.Worktrees, worktreeResp)
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
