package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"schulapp/internal/api"
	appmw "schulapp/internal/middleware"
)

func (h *Server) GetApiV1Subjects(w http.ResponseWriter, r *http.Request) {
	claims := appmw.GetClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht autorisiert")
		return
	}

	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, name, short FROM subjects ORDER BY name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	defer rows.Close()

	subjects := []api.Subject{}
	for rows.Next() {
		var s api.Subject
		if err := rows.Scan(&s.Id, &s.Name, &s.Short); err != nil {
			writeError(w, http.StatusInternalServerError, "Fehler beim Lesen")
			return
		}
		subjects = append(subjects, s)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Fehler beim Iterieren")
		return
	}

	writeJSON(w, http.StatusOK, subjects)
}

func (h *Server) PostApiV1Subjects(w http.ResponseWriter, r *http.Request) {
	claims := appmw.GetClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht autorisiert")
		return
	}
	if !isSuperadmin(claims.Role) {
		writeError(w, http.StatusForbidden, "keine Berechtigung")
		return
	}

	var req api.CreateSubjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	if req.Name == "" || req.Short == "" {
		writeError(w, http.StatusBadRequest, "name und short dürfen nicht leer sein")
		return
	}

	var subject api.Subject
	err := h.DB.QueryRowContext(r.Context(),
		`INSERT INTO subjects (name, short) VALUES ($1, $2)
		 RETURNING id, name, short`,
		req.Name, req.Short,
	).Scan(&subject.Id, &subject.Name, &subject.Short)

	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "Fach existiert bereits")
			return
		}
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}

	writeJSON(w, http.StatusCreated, subject)
}
