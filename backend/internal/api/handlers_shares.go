package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cloud-ru-tech/ar-drive/backend/internal/arclient"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/audit"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/auth"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/middleware"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/shares"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/validation"
)

type createShareRequest struct {
	RegistryID   string `json:"registry_id"`
	FilePath     string `json:"file_path"`
	Password     string `json:"password,omitempty"`
	TTLSeconds   int64  `json:"ttl_seconds,omitempty"`
	MaxDownloads int64  `json:"max_downloads,omitempty"`
}

type shareResponseItem struct {
	ShortID      string    `json:"short_id"`
	URL          string    `json:"url"`
	ProjectID    string    `json:"project_id"`
	RegistryID   string    `json:"registry_id"`
	FilePath     string    `json:"file_path"`
	HasPassword  bool      `json:"has_password"`
	MaxDownloads int64     `json:"max_downloads,omitempty"`
	Downloads    int64     `json:"downloads"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Revoked      bool      `json:"revoked"`
	CreatedAt    time.Time `json:"created_at"`
}

// POST /api/v1/shares
func (s *Server) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	creds, _, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	var req createShareRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "bad_request")
		return
	}
	if req.RegistryID == "" {
		writeError(w, http.StatusBadRequest, "registry_id required", "validation_failed")
		return
	}
	cleanPath, err := validation.CleanFilePath(req.FilePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_path")
		return
	}

	link, err := s.shares.Create(r.Context(), shares.CreateInput{
		UserKey:      creds.UserKey(),
		ProjectID:    creds.ProjectID,
		RegistryID:   req.RegistryID,
		FilePath:     cleanPath,
		Password:     req.Password,
		TTL:          time.Duration(req.TTLSeconds) * time.Second,
		MaxDownloads: req.MaxDownloads,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "internal")
		return
	}

	_ = s.audit.Record(r.Context(), audit.Event{
		UserKey: creds.UserKey(), ProjectID: creds.ProjectID, RegistryID: req.RegistryID,
		Action: "share.create", Target: cleanPath, Status: "success",
		IP: middleware.RealIP(r), UserAgent: r.UserAgent(),
		Meta: map[string]any{"short_id": link.ShortID, "ttl_sec": req.TTLSeconds},
	})

	writeJSON(w, http.StatusCreated, toShareResponse(link, r))
}

// GET /api/v1/shares
func (s *Server) handleListShares(w http.ResponseWriter, r *http.Request) {
	creds, _, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	list, err := s.shares.ListByUser(r.Context(), creds.UserKey())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "internal")
		return
	}
	out := make([]shareResponseItem, len(list))
	for i := range list {
		out[i] = toShareResponse(&list[i], r)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// DELETE /api/v1/shares/{shortID}
func (s *Server) handleRevokeShare(w http.ResponseWriter, r *http.Request) {
	creds, _, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	shortID := chi.URLParam(r, "shortID")
	if err := s.shares.Revoke(r.Context(), creds.UserKey(), shortID); err != nil {
		if errors.Is(err, shares.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "internal")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /s/{shortID}?password=...
// Публичная ручка: бэкенд сам идёт в Cloud.ru от имени владельца ссылки.
//
// ВНИМАНИЕ: владелец ссылки хранит ключи в localStorage SPA, а сервер не знает их
// при «холодном» переходе по share-ссылке. В MVP мы поддерживаем сценарий
// «публичный реестр» (без аутентификации в AR) и «приватный реестр» — когда
// пользователь активен в браузере и шлёт заголовки. Полноценное хранение
// ключей на стороне сервера для оффлайн-шеринга — задача будущей итерации
// (см. docs/decisions.md, решение «long-lived share token»).
func (s *Server) handleShareDownload(w http.ResponseWriter, r *http.Request) {
	shortID := chi.URLParam(r, "shortID")
	password := r.URL.Query().Get("password")

	link, err := s.shares.Validate(r.Context(), shortID, password)
	if err != nil {
		switch {
		case errors.Is(err, shares.ErrNotFound):
			writeError(w, http.StatusNotFound, "share link not found", "not_found")
		case errors.Is(err, shares.ErrRevoked), errors.Is(err, shares.ErrExpired), errors.Is(err, shares.ErrExhausted):
			writeError(w, http.StatusGone, err.Error(), "share_gone")
		case errors.Is(err, shares.ErrPasswordReq):
			writeError(w, http.StatusUnauthorized, "password required", "password_required")
		case errors.Is(err, shares.ErrPasswordWrong):
			writeError(w, http.StatusUnauthorized, "wrong password", "wrong_password")
		default:
			writeError(w, http.StatusInternalServerError, err.Error(), "internal")
		}
		return
	}

	// Пытаемся достать креды из заголовков (активная сессия владельца) —
	// иначе сообщаем, что нужно открыть ссылку в браузере с активной сессией.
	creds, err := credsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusForbidden,
			"this MVP requires the link owner's browser session for private registries; see docs/decisions.md",
			"owner_session_required")
		return
	}
	token, err := s.tokens.Get(r.Context(), creds)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusForbidden, "owner credentials invalid", "invalid_credentials")
			return
		}
		writeError(w, http.StatusBadGateway, err.Error(), "iam_error")
		return
	}

	resp, err := s.ar.DownloadFile(r.Context(), token, link.RegistryID, link.FilePath)
	if err != nil {
		if errors.Is(err, arclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file no longer exists", "not_found")
			return
		}
		writeError(w, http.StatusBadGateway, err.Error(), "ar_error")
		return
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Content-Disposition",
		"attachment; filename=\""+path.Base(link.FilePath)+"\"")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)

	_ = s.shares.IncrementDownloads(r.Context(), shortID)
}

func toShareResponse(l *shares.Link, r *http.Request) shareResponseItem {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return shareResponseItem{
		ShortID:      l.ShortID,
		URL:          scheme + "://" + r.Host + "/s/" + l.ShortID,
		ProjectID:    l.ProjectID,
		RegistryID:   l.RegistryID,
		FilePath:     l.FilePath,
		HasPassword:  l.HasPassword,
		MaxDownloads: l.MaxDownloads,
		Downloads:    l.Downloads,
		ExpiresAt:    l.ExpiresAt,
		Revoked:      l.Revoked,
		CreatedAt:    l.CreatedAt,
	}
}
