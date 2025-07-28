package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

//go:embed all:dist
var distFS embed.FS

// Handler handles web console requests
type Handler struct {
	staticFS fs.FS
}

// NewHandler creates a new web handler
func NewHandler() (*Handler, error) {
	staticFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}

	return &Handler{
		staticFS: staticFS,
	}, nil
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "" || path == "/" {
		path = "/index.html"
	}

	// API routes
	if strings.HasPrefix(path, "/api/") {
		h.handleAPI(w, r, path)
		return
	}

	// Try to serve the exact file first
	file, err := h.staticFS.Open(strings.TrimPrefix(path, "/"))
	if err == nil {
		file.Close()
		log.Debug().
			Str("component", "web").
			Str("path", path).
			Msg("Serving static file")
		http.FileServer(http.FS(h.staticFS)).ServeHTTP(w, r)
		return
	}

	// For SPA routing, serve index.html for any unmatched paths
	log.Debug().
		Str("component", "web").
		Str("requested_path", path).
		Str("served_path", "/index.html").
		Msg("Serving index.html for SPA route")

	// Create a new request with index.html path
	r.URL.Path = "/index.html"
	http.FileServer(http.FS(h.staticFS)).ServeHTTP(w, r)
}

func (h *Handler) handleAPI(w http.ResponseWriter, r *http.Request, path string) {
	// Handle exact matches first
	switch path {
	case "/api/threads":
		if r.Method == http.MethodGet {
			GetThreads(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case "/api/sessions":
		if r.Method == http.MethodGet {
			GetSessions(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Handle pattern matches
	if strings.HasPrefix(path, "/api/threads/") && strings.HasSuffix(path, "/sessions") {
		if r.Method == http.MethodGet {
			// Pass request directly to API handler
			GetThreadSessions(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	http.NotFound(w, r)
}
