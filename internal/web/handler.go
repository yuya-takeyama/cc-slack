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
	path := strings.TrimPrefix(r.URL.Path, "/web")
	if path == "" || path == "/" {
		path = "/index.html"
	}

	// API routes
	if strings.HasPrefix(path, "/api/") {
		h.handleAPI(w, r, path)
		return
	}

	// Static files
	log.Debug().
		Str("component", "web").
		Str("path", path).
		Msg("Serving static file")

	http.FileServer(http.FS(h.staticFS)).ServeHTTP(w, r)
}

func (h *Handler) handleAPI(w http.ResponseWriter, r *http.Request, path string) {
	switch path {
	case "/api/threads":
		if r.Method == http.MethodGet {
			GetThreads(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}
