package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cloud-ru-tech/ar-drive/backend/internal/audit"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/auth"
)

type verifyRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	ProjectID    string `json:"project_id"`
}

type verifyResponse struct {
	OK       bool   `json:"ok"`
	UserKey  string `json:"user_key"`
	Message  string `json:"message,omitempty"`
}

// POST /api/v1/auth/verify — проверяет ключи Cloud.ru через IAM и кеширует токен.
func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "bad_request")
		return
	}
	creds := auth.Credentials{
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		ProjectID:    req.ProjectID,
	}
	if creds.ClientID == "" || creds.ClientSecret == "" || creds.ProjectID == "" {
		writeError(w, http.StatusBadRequest, "client_id, client_secret и project_id обязательны", "validation_failed")
		return
	}

	if _, err := s.tokens.Get(r.Context(), creds); err != nil {
		_ = s.audit.Record(r.Context(), audit.Event{
			UserKey:   creds.UserKey(),
			ProjectID: creds.ProjectID,
			Action:    "auth.verify",
			Status:    "error",
			ErrorMessage: err.Error(),
		})
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "Неверные ключи Cloud.ru — проверьте Key ID и Key Secret", "invalid_credentials")
			return
		}
		writeError(w, http.StatusBadGateway, "Не удалось обратиться к IAM Cloud.ru: "+err.Error(), "iam_unavailable")
		return
	}

	_ = s.audit.Record(r.Context(), audit.Event{
		UserKey:   creds.UserKey(),
		ProjectID: creds.ProjectID,
		Action:    "auth.verify",
		Status:    "success",
	})
	writeJSON(w, http.StatusOK, verifyResponse{OK: true, UserKey: creds.UserKey()})
}
