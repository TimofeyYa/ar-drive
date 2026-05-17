package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/cloud-ru-tech/ar-drive/backend/internal/audit"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/folders"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/middleware"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/validation"
)

type createFolderRequest struct {
	Path string `json:"path"`
}

// POST /api/v1/registries/{registryID}/folders  {path: "dir/sub"}
func (s *Server) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	creds, _, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	registryID := chi.URLParam(r, "registryID")

	var req createFolderRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "bad_request")
		return
	}
	cleaned, err := validation.CleanFolderPath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_path")
		return
	}

	err = s.folders.Create(r.Context(), folders.Marker{
		UserKey:   creds.UserKey(),
		ProjectID: creds.ProjectID,
		RegistryID: registryID,
		FullPath:  cleaned,
		CreatedBy: creds.UserKey(),
	})
	if errors.Is(err, folders.ErrAlreadyExists) {
		writeError(w, http.StatusConflict, "folder already exists", "folder_exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "internal")
		return
	}

	_ = s.audit.Record(r.Context(), audit.Event{
		UserKey: creds.UserKey(), ProjectID: creds.ProjectID, RegistryID: registryID,
		Action: "folder.create", Target: cleaned, Status: "success",
		IP: middleware.RealIP(r), UserAgent: r.UserAgent(),
	})

	writeJSON(w, http.StatusCreated, map[string]any{"path": cleaned, "virtual": true})
}

// DELETE /api/v1/registries/{registryID}/folders?path=dir/sub
// Удаляет все файлы под префиксом + маркер. Возвращает счётчик удалённого.
func (s *Server) handleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	creds, token, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	registryID := chi.URLParam(r, "registryID")
	folderPath, err := validation.CleanFolderPath(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_path")
		return
	}

	// Итеративно вытаскиваем все файлы под префиксом и удаляем по одному.
	var deleted int
	var pageToken string
	for {
		resp, err := s.ar.ListFiles(r.Context(), token, registryID, folderPath, pageToken, 500)
		if err != nil {
			mapARError(w, err)
			return
		}
		for _, f := range resp.Items {
			if err := s.ar.DeleteFile(r.Context(), token, registryID, f.Path); err != nil {
				mapARError(w, err)
				return
			}
			deleted++
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	_ = s.folders.DeletePrefix(r.Context(), creds.ProjectID, registryID, folderPath)

	_ = s.audit.Record(r.Context(), audit.Event{
		UserKey: creds.UserKey(), ProjectID: creds.ProjectID, RegistryID: registryID,
		Action: "folder.delete", Target: folderPath, Status: "success",
		IP: middleware.RealIP(r), UserAgent: r.UserAgent(),
		Meta: map[string]any{"deleted_files": deleted},
	})

	writeJSON(w, http.StatusOK, map[string]any{"deleted_files": deleted, "path": folderPath})
}
