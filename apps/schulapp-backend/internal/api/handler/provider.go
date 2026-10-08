package handler

import (
	"encoding/json"
	"net/http"
)

// Daten-Provider (Schuldaten-Quelle): Der Nutzer wählt im Frontend zwischen
// SmartTable (lokal) und SchoolConnect (Schülerportal). Der Wert wird hier
// serverseitig gespeichert, damit ihn auch der OpenCode-MCP-Server
// respektieren kann (nicht mehr nur localStorage im Browser).

type dataProvider struct {
	Provider string `json:"provider"`
}

// GetApiV1MeProvider liefert den gespeicherten Daten-Provider des Nutzers.
func (h *Server) GetApiV1MeProvider(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	p := h.userDataProvider(r, c.UserID)
	writeJSON(w, http.StatusOK, dataProvider{Provider: p})
}

// PatchApiV1MeProvider speichert den Daten-Provider des Nutzers.
func (h *Server) PatchApiV1MeProvider(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	var req dataProvider
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	if req.Provider != "smarttable" && req.Provider != "schoolconnect" {
		writeError(w, http.StatusBadRequest, "provider muss smarttable oder schoolconnect sein")
		return
	}
	if _, err := h.DB.ExecContext(r.Context(),
		`UPDATE users SET data_provider=$1 WHERE id=$2`, req.Provider, c.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	writeJSON(w, http.StatusOK, dataProvider{Provider: req.Provider})
}

// userDataProvider liest den Daten-Provider des Nutzers; bei Fehler oder
// fehlendem Wert fällt es auf den bisherigen Default 'smarttable' zurück.
func (h *Server) userDataProvider(r *http.Request, userID int) string {
	var p string
	if err := h.DB.QueryRowContext(r.Context(),
		`SELECT data_provider FROM users WHERE id=$1`, userID).Scan(&p); err != nil || p == "" {
		return "smarttable"
	}
	return p
}
