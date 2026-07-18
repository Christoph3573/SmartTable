package handler

import (
	"net/http"
)

func (h *Server) GetApiV1Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
