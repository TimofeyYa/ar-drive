package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/cloud-ru-tech/ar-drive/backend/internal/arclient"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/auth"
)

// GET /api/v1/registries?pageSize=&pageToken=
func (s *Server) handleListRegistries(w http.ResponseWriter, r *http.Request) {
	creds, err := credsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error(), "missing_credentials")
		return
	}
	token, err := s.tokens.Get(r.Context(), creds)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid credentials", "invalid_credentials")
			return
		}
		writeError(w, http.StatusBadGateway, err.Error(), "iam_error")
		return
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize <= 0 {
		pageSize = 50
	}
	pageToken := r.URL.Query().Get("pageToken")

	resp, err := s.ar.ListRegistries(r.Context(), token, creds.ProjectID, pageToken, pageSize)
	if err != nil {
		mapARError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func mapARError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, arclient.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "Cloud.ru AR rejected credentials", "ar_unauthorized")
	case errors.Is(err, arclient.ErrNotFound):
		writeError(w, http.StatusNotFound, "resource not found in Cloud.ru AR", "ar_not_found")
	case errors.Is(err, arclient.ErrConflict):
		writeError(w, http.StatusConflict, "conflict in Cloud.ru AR", "ar_conflict")
	default:
		writeError(w, http.StatusBadGateway, err.Error(), "ar_error")
	}
}
