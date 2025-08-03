package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/yuya-takeyama/cc-slack/internal/config"
	"github.com/yuya-takeyama/cc-slack/internal/db"
	"github.com/yuya-takeyama/cc-slack/internal/slack"
)

var (
	database     *sql.DB
	queries      *db.Queries
	channelCache *slack.ChannelCache
)

// SetDatabase sets the database connection for the web package
func SetDatabase(dbConn *sql.DB) {
	database = dbConn
	queries = db.New(dbConn)
}

// SetChannelCache sets the channel cache for the web package
func SetChannelCache(cache *slack.ChannelCache) {
	channelCache = cache
}

// ThreadResponse represents a thread in the API response
type ThreadResponse struct {
	ThreadTs            string `json:"thread_ts"`
	ThreadTime          string `json:"thread_time"`
	ChannelID           string `json:"channel_id"`
	ChannelName         string `json:"channel_name"`
	WorkspaceSubdomain  string `json:"workspace_subdomain"`
	SessionCount        int    `json:"session_count"`
	LatestSessionStatus string `json:"latest_session_status"`
	InitialPrompt       string `json:"initial_prompt,omitempty"`
}

// ThreadsResponse represents the threads API response
type ThreadsResponse struct {
	Threads []ThreadResponse `json:"threads"`
	HasMore bool             `json:"has_more"`
	Page    int              `json:"page"`
}

// PaginationParams extracts pagination parameters from request
func getPaginationParams(r *http.Request) (limit, offset, page int) {
	// Default pagination values
	const defaultLimit = 50
	limit = defaultLimit
	offset = 0
	page = 1

	// Get page from query params (1-indexed)
	pageStr := r.URL.Query().Get("page")
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
			offset = (page - 1) * limit
		}
	}

	// For checking if there's a next page, we fetch one extra item
	return limit + 1, offset, page
}

// GetThreads handles GET /api/threads
func GetThreads(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Get pagination parameters
	limit, offset, page := getPaginationParams(r)

	// Get threads with pagination
	threads, err := queries.ListThreadsPaginated(ctx, db.ListThreadsPaginatedParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to list threads")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if there are more threads (for next page)
	hasMore := len(threads) > limit-1
	if hasMore {
		// Remove the extra thread we fetched
		threads = threads[:limit-1]
	}

	// Build response
	response := ThreadsResponse{
		Threads: make([]ThreadResponse, 0, len(threads)),
		HasMore: hasMore,
		Page:    page,
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
		sessions, err := queries.ListSessionsByThreadID(ctx, thread.ID)
		if err == nil {
			sessionCount = len(sessions)
		}

		// Convert thread timestamp to human-readable format
		threadTime, err := ConvertThreadTsToTime(thread.ThreadTs)
		if err != nil {
			log.Error().Err(err).Str("thread_ts", thread.ThreadTs).Msg("Failed to convert thread timestamp")
			// Use original timestamp as fallback
			threadTime = time.Now()
		}

		// Get channel name
		channelName := thread.ChannelID
		if channelCache != nil {
			channelName = channelCache.GetChannelName(ctx, thread.ChannelID)
		}

		threadResp := ThreadResponse{
			ThreadTs:            thread.ThreadTs,
			ThreadTime:          FormatThreadTime(threadTime),
			ChannelID:           thread.ChannelID,
			ChannelName:         channelName,
			WorkspaceSubdomain:  config.SLACK_WORKSPACE_SUBDOMAIN,
			SessionCount:        sessionCount,
			LatestSessionStatus: latestStatus,
		}

		// Add initial prompt if available
		if thread.FirstSessionPrompt.Valid {
			threadResp.InitialPrompt = thread.FirstSessionPrompt.String
		}

		response.Threads = append(response.Threads, threadResp)
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// SessionResponse represents a session in the API response
type SessionResponse struct {
	SessionID     string `json:"session_id"`
	ThreadTs      string `json:"thread_ts"`
	Status        string `json:"status"`
	StartedAt     string `json:"started_at"`
	EndedAt       string `json:"ended_at,omitempty"`
	InitialPrompt string `json:"initial_prompt,omitempty"`
}

// SessionsResponse represents the sessions API response
type SessionsResponse struct {
	Sessions []SessionResponse `json:"sessions"`
	HasMore  bool              `json:"has_more"`
	Page     int               `json:"page"`
}

// GetSessions handles GET /api/sessions
func GetSessions(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Get pagination parameters
	limit, offset, page := getPaginationParams(r)

	// Get sessions with pagination
	sessions, err := queries.ListSessionsPaginated(ctx, db.ListSessionsPaginatedParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to list sessions")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if there are more sessions (for next page)
	hasMore := len(sessions) > limit-1
	if hasMore {
		// Remove the extra session we fetched
		sessions = sessions[:limit-1]
	}

	// Build response
	response := SessionsResponse{
		Sessions: make([]SessionResponse, 0, len(sessions)),
		HasMore:  hasMore,
		Page:     page,
	}

	for _, session := range sessions {
		// Get thread info
		thread, err := queries.GetThreadByID(ctx, session.ThreadID)
		if err != nil {
			log.Error().Err(err).Int64("thread_id", session.ThreadID).Msg("Failed to get thread")
			continue
		}

		sessionResp := SessionResponse{
			SessionID: session.SessionID,
			ThreadTs:  thread.ThreadTs,
			Status:    session.Status.String,
			StartedAt: session.StartedAt.Time.Format("2006-01-02T15:04:05Z"),
		}

		if session.EndedAt.Valid {
			sessionResp.EndedAt = session.EndedAt.Time.Format("2006-01-02T15:04:05Z")
		}

		if session.InitialPrompt.Valid {
			sessionResp.InitialPrompt = session.InitialPrompt.String
		}

		response.Sessions = append(response.Sessions, sessionResp)
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// ThreadSessionsResponse represents the thread sessions API response
type ThreadSessionsResponse struct {
	Thread   *ThreadResponse   `json:"thread"`
	Sessions []SessionResponse `json:"sessions"`
}

// GetThreadSessions handles GET /api/threads/:thread_id/sessions
func GetThreadSessions(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Extract thread_id from URL path
	// Expected path: /api/threads/{thread_id}/sessions
	threadTs := r.URL.Path[len("/api/threads/"):]
	if idx := len(threadTs) - len("/sessions"); idx > 0 {
		threadTs = threadTs[:idx]
	}

	if threadTs == "" {
		http.Error(w, "Thread ID is required", http.StatusBadRequest)
		return
	}

	// Get thread by thread_ts
	thread, err := queries.GetThreadByThreadTs(ctx, threadTs)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Thread not found", http.StatusNotFound)
			return
		}
		log.Error().Err(err).Str("thread_ts", threadTs).Msg("Failed to get thread")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get all sessions for this thread
	sessions, err := queries.ListSessionsByThreadID(ctx, thread.ID)
	if err != nil {
		log.Error().Err(err).Int64("thread_id", thread.ID).Msg("Failed to list sessions")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert thread timestamp to human-readable format
	threadTime, err := ConvertThreadTsToTime(thread.ThreadTs)
	if err != nil {
		log.Error().Err(err).Str("thread_ts", thread.ThreadTs).Msg("Failed to convert thread timestamp")
		// Use original timestamp as fallback
		threadTime = time.Now()
	}

	// Get channel name
	channelName := thread.ChannelID
	if channelCache != nil {
		channelName = channelCache.GetChannelName(ctx, thread.ChannelID)
	}

	// Build thread response
	threadResp := &ThreadResponse{
		ThreadTs:            thread.ThreadTs,
		ThreadTime:          FormatThreadTime(threadTime),
		ChannelID:           thread.ChannelID,
		ChannelName:         channelName,
		WorkspaceSubdomain:  config.SLACK_WORKSPACE_SUBDOMAIN,
		SessionCount:        len(sessions),
		LatestSessionStatus: "none",
	}

	// Get latest session status
	if len(sessions) > 0 {
		latestSession := sessions[0] // Assuming sessions are ordered by created_at DESC
		threadResp.LatestSessionStatus = latestSession.Status.String
	}

	// Build response
	response := ThreadSessionsResponse{
		Thread:   threadResp,
		Sessions: make([]SessionResponse, 0, len(sessions)),
	}

	for _, session := range sessions {
		sessionResp := SessionResponse{
			SessionID: session.SessionID,
			ThreadTs:  thread.ThreadTs,
			Status:    session.Status.String,
			StartedAt: session.StartedAt.Time.Format("2006-01-02T15:04:05Z"),
		}

		if session.EndedAt.Valid {
			sessionResp.EndedAt = session.EndedAt.Time.Format("2006-01-02T15:04:05Z")
		}

		if session.InitialPrompt.Valid {
			sessionResp.InitialPrompt = session.InitialPrompt.String
		}

		response.Sessions = append(response.Sessions, sessionResp)
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
