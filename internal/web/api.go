package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/db"
)

var (
	database *sql.DB
	queries  *db.Queries
)

// SetDatabase sets the database connection for the web package
func SetDatabase(dbConn *sql.DB) {
	database = dbConn
	queries = db.New(dbConn)
}

// ThreadResponse represents a thread in the API response
type ThreadResponse struct {
	ThreadTs            string `json:"thread_ts"`
	ChannelID           string `json:"channel_id"`
	WorkspaceSubdomain  string `json:"workspace_subdomain"`
	SessionCount        int    `json:"session_count"`
	LatestSessionStatus string `json:"latest_session_status"`
}

// ThreadsResponse represents the threads API response
type ThreadsResponse struct {
	Threads []ThreadResponse `json:"threads"`
}

// GetThreads handles GET /web/api/threads
func GetThreads(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Get all threads
	threads, err := queries.ListThreads(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list threads")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build response
	response := ThreadsResponse{
		Threads: make([]ThreadResponse, 0, len(threads)),
	}

	for _, thread := range threads {
		// Count sessions for this thread
		sessionCount := 0
		latestStatus := "none"

		// Get latest session for the thread
		latestSession, err := queries.GetLatestSessionByThread(ctx, thread.ID)
		if err == nil {
			latestStatus = latestSession.Status.String
		}

		// Count all sessions for this thread
		sessions, err := queries.ListActiveSessions(ctx)
		if err == nil {
			for _, session := range sessions {
				if session.ThreadID == thread.ID {
					sessionCount++
				}
			}
		}

		response.Threads = append(response.Threads, ThreadResponse{
			ThreadTs:            thread.ThreadTs,
			ChannelID:           thread.ChannelID,
			WorkspaceSubdomain:  config.SLACK_WORKSPACE_SUBDOMAIN,
			SessionCount:        sessionCount,
			LatestSessionStatus: latestStatus,
		})
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
