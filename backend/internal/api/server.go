// Package api — HTTP-ручки приложения. Использует chi.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"github.com/cloud-ru-tech/ar-drive/backend/internal/arclient"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/audit"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/auth"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/config"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/folders"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/middleware"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/shares"
)

type Server struct {
	cfg    *config.Config
	logger zerolog.Logger

	tokens  *auth.TokenCache
	ar      *arclient.Client
	folders *folders.Store
	shares  *shares.Store
	audit   *audit.Logger
}

func New(
	cfg *config.Config,
	logger zerolog.Logger,
	tokens *auth.TokenCache,
	ar *arclient.Client,
	folderStore *folders.Store,
	shareStore *shares.Store,
	auditLogger *audit.Logger,
) *Server {
	return &Server{
		cfg:     cfg,
		logger:  logger,
		tokens:  tokens,
		ar:      ar,
		folders: folderStore,
		shares:  shareStore,
		audit:   auditLogger,
	}
}

// Router строит chi.Mux со всеми ручками.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Compress(5))
	r.Use(middleware.AccessLog(s.logger))
	r.Use(middleware.SecureHeaders())
	r.Use(middleware.CORS(s.cfg.CORSOrigins))
	r.Use(middleware.RateLimit(s.cfg.RateLimitGlobalRPM))

	// Health & metrics
	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.With(middleware.RateLimit(s.cfg.RateLimitAuthRPM)).
			Post("/auth/verify", s.handleAuthVerify)

		r.Get("/registries", s.handleListRegistries)

		r.Route("/registries/{registryID}", func(r chi.Router) {
			r.Get("/files", s.handleListFiles)
			r.With(middleware.RateLimit(s.cfg.RateLimitUploadRPM)).
				Post("/files", s.handleUploadFile)
			r.Get("/files/download", s.handleDownloadFile) // ?path=...
			r.Delete("/files", s.handleDeleteFile)         // ?path=...

			r.Post("/folders", s.handleCreateFolder)
			r.Delete("/folders", s.handleDeleteFolder) // ?path=...
		})

		r.Route("/shares", func(r chi.Router) {
			r.Get("/", s.handleListShares)
			r.Post("/", s.handleCreateShare)
			r.Delete("/{shortID}", s.handleRevokeShare)
		})
	})

	// Публичная ручка скачивания по short-link
	r.Get("/s/{shortID}", s.handleShareDownload)

	return r
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

func writeError(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, errorBody{Error: msg, Code: code})
}
