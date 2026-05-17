package api

import (
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/cloud-ru-tech/ar-drive/backend/internal/arclient"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/audit"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/auth"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/middleware"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/validation"
)

type listFilesResponse struct {
	Files         []arclient.File   `json:"files"`
	Folders       []virtualFolder   `json:"folders"`
	NextPageToken string            `json:"nextPageToken,omitempty"`
}

type virtualFolder struct {
	Path    string `json:"path"`
	Virtual bool   `json:"virtual"` // true = маркер без файлов
}

// GET /api/v1/registries/{registryID}/files?path=&pageSize=&pageToken=
func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	creds, token, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	registryID := chi.URLParam(r, "registryID")

	prefix := r.URL.Query().Get("path")
	if prefix != "" {
		cleaned, err := validation.CleanFolderPath(prefix)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "bad_path")
			return
		}
		prefix = cleaned
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize <= 0 {
		pageSize = 100
	}
	pageToken := r.URL.Query().Get("pageToken")

	arResp, err := s.ar.ListFiles(r.Context(), token, registryID, prefix, pageToken, pageSize)
	if err != nil {
		mapARError(w, err)
		return
	}

	// Маркеры пустых папок
	markers, err := s.folders.List(r.Context(), creds.ProjectID, registryID, prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "internal")
		return
	}

	// Из реальных файлов — извлекаем "непосредственные" подпапки текущей директории.
	subfoldersSet := map[string]bool{}
	files := make([]arclient.File, 0, len(arResp.Items))
	for _, f := range arResp.Items {
		rel := strings.TrimPrefix(f.Path, prefix)
		if i := strings.Index(rel, "/"); i >= 0 {
			subfoldersSet[prefix+rel[:i+1]] = true
			continue
		}
		files = append(files, f)
	}
	// Маркеры тоже подмешиваем (могут быть «пустыми», т.е. без реальных файлов).
	for _, m := range markers {
		rel := strings.TrimPrefix(m.FullPath, prefix)
		if rel == "" {
			continue
		}
		if i := strings.Index(rel, "/"); i >= 0 {
			subfoldersSet[prefix+rel[:i+1]] = true
		}
	}

	folderList := make([]virtualFolder, 0, len(subfoldersSet))
	for p := range subfoldersSet {
		folderList = append(folderList, virtualFolder{Path: p, Virtual: !hasRealFiles(arResp.Items, p)})
	}

	writeJSON(w, http.StatusOK, listFilesResponse{
		Files:         files,
		Folders:       folderList,
		NextPageToken: arResp.NextPageToken,
	})
}

func hasRealFiles(items []arclient.File, prefix string) bool {
	for _, it := range items {
		if strings.HasPrefix(it.Path, prefix) {
			return true
		}
	}
	return false
}

// POST /api/v1/registries/{registryID}/files (multipart: file + path)
func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	creds, token, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	registryID := chi.URLParam(r, "registryID")

	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart: "+err.Error(), "bad_multipart")
		return
	}

	folder := r.FormValue("path") // целевая папка, может быть ""
	if folder != "" {
		cleaned, err := validation.CleanFolderPath(folder)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "bad_path")
			return
		}
		folder = cleaned
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'file' field", "bad_multipart")
		return
	}
	defer file.Close()

	if header.Size > s.cfg.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge,
			"file exceeds limit "+strconv.FormatInt(s.cfg.MaxUploadBytes, 10), "too_large")
		return
	}

	filename := path.Base(header.Filename)
	fullPath := folder + filename
	if _, err := validation.CleanFilePath(fullPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_path")
		return
	}

	contentType := header.Header.Get("Content-Type")

	uploaded, err := s.ar.UploadFile(r.Context(), token, registryID, fullPath, contentType, file, header.Size)
	if err != nil {
		_ = s.audit.Record(r.Context(), audit.Event{
			UserKey: creds.UserKey(), ProjectID: creds.ProjectID, RegistryID: registryID,
			Action: "file.upload", Target: fullPath, Status: "error",
			ErrorMessage: err.Error(), IP: middleware.RealIP(r), UserAgent: r.UserAgent(),
		})
		mapARError(w, err)
		return
	}

	// Папка стала «реальной» — снимаем маркеры выше по дереву.
	if folder != "" {
		_ = s.folders.Delete(r.Context(), creds.ProjectID, registryID, folder)
	}

	_ = s.audit.Record(r.Context(), audit.Event{
		UserKey: creds.UserKey(), ProjectID: creds.ProjectID, RegistryID: registryID,
		Action: "file.upload", Target: fullPath, Status: "success",
		IP: middleware.RealIP(r), UserAgent: r.UserAgent(),
		Meta: map[string]any{"size": header.Size, "content_type": contentType},
	})

	writeJSON(w, http.StatusCreated, uploaded)
}

// GET /api/v1/registries/{registryID}/files/download?path=
func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	creds, token, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	registryID := chi.URLParam(r, "registryID")
	filePath, err := validation.CleanFilePath(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_path")
		return
	}

	resp, err := s.ar.DownloadFile(r.Context(), token, registryID, filePath)
	if err != nil {
		mapARError(w, err)
		return
	}
	defer resp.Body.Close()

	// Прокидываем заголовки
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+path.Base(filePath)+"\"")
	w.WriteHeader(resp.StatusCode)

	_, _ = io.Copy(w, resp.Body)

	_ = s.audit.Record(r.Context(), audit.Event{
		UserKey: creds.UserKey(), ProjectID: creds.ProjectID, RegistryID: registryID,
		Action: "file.download", Target: filePath, Status: "success",
		IP: middleware.RealIP(r), UserAgent: r.UserAgent(),
	})
}

// DELETE /api/v1/registries/{registryID}/files?path=
func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	creds, token, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	registryID := chi.URLParam(r, "registryID")
	filePath, err := validation.CleanFilePath(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_path")
		return
	}

	if err := s.ar.DeleteFile(r.Context(), token, registryID, filePath); err != nil {
		_ = s.audit.Record(r.Context(), audit.Event{
			UserKey: creds.UserKey(), ProjectID: creds.ProjectID, RegistryID: registryID,
			Action: "file.delete", Target: filePath, Status: "error", ErrorMessage: err.Error(),
		})
		mapARError(w, err)
		return
	}

	_ = s.audit.Record(r.Context(), audit.Event{
		UserKey: creds.UserKey(), ProjectID: creds.ProjectID, RegistryID: registryID,
		Action: "file.delete", Target: filePath, Status: "success",
		IP: middleware.RealIP(r), UserAgent: r.UserAgent(),
	})
	w.WriteHeader(http.StatusNoContent)
}

// requireAuth — общий helper: достаёт креды, выдаёт токен.
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) (auth.Credentials, string, bool) {
	creds, err := credsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error(), "missing_credentials")
		return creds, "", false
	}
	token, err := s.tokens.Get(r.Context(), creds)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid Cloud.ru credentials", "invalid_credentials")
		} else {
			writeError(w, http.StatusBadGateway, err.Error(), "iam_error")
		}
		return creds, "", false
	}
	return creds, token, true
}
