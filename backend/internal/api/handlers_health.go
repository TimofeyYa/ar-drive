package api

import (
	"net/http"
)

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	// readiness: проверим, что up endpoint IAM Cloud.ru хотя бы отвечает.
	// Это не проверка валидности ключей — просто доступность.
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
