package handler

import (
	"database/sql"
	"net/http"

	appmw "schulapp/internal/middleware"
)

// claims returns the authenticated user. Authentication itself is installed at
// router level; keeping this guard here also makes handlers safe in tests.
func (h *Server) claims(w http.ResponseWriter, r *http.Request) *appmw.Claims {
	c := appmw.GetClaims(r)
	if c == nil {
		writeError(w, http.StatusUnauthorized, "nicht autorisiert")
	}
	return c
}

func requireRole(w http.ResponseWriter, c *appmw.Claims, roles ...string) bool {
	if c == nil {
		return false
	}
	for _, role := range roles {
		if c.Role == role {
			return true
		}
	}
	writeError(w, http.StatusForbidden, "keine Berechtigung")
	return false
}

func (h *Server) canReadClass(r *http.Request, classID int) bool {
	c := appmw.GetClaims(r)
	if c == nil {
		return false
	}
	if c.Role == "admin" {
		return true
	}
	var exists bool
	err := h.DB.QueryRowContext(r.Context(), `SELECT EXISTS (
		SELECT 1 FROM class_members WHERE class_id=$1 AND user_id=$2
		UNION ALL SELECT 1 FROM class_teachers WHERE class_id=$1 AND user_id=$2
	)`, classID, c.UserID).Scan(&exists)
	return err == nil && exists
}

func (h *Server) canManageClass(r *http.Request, classID int) bool {
	c := appmw.GetClaims(r)
	if c == nil {
		return false
	}
	if c.Role == "admin" {
		return true
	}
	if c.Role != "teacher" {
		return false
	}
	var exists bool
	err := h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM class_teachers WHERE class_id=$1 AND user_id=$2)`, classID, c.UserID).Scan(&exists)
	return err == nil && exists
}

func (h *Server) requireClassRead(w http.ResponseWriter, r *http.Request, classID int) bool {
	if h.claims(w, r) == nil {
		return false
	}
	if !h.canReadClass(r, classID) {
		writeError(w, http.StatusForbidden, "kein Zugriff auf diese Klasse")
		return false
	}
	return true
}

func (h *Server) requireClassManage(w http.ResponseWriter, r *http.Request, classID int) bool {
	if h.claims(w, r) == nil {
		return false
	}
	if !h.canManageClass(r, classID) {
		writeError(w, http.StatusForbidden, "keine Verwaltungsrechte für diese Klasse")
		return false
	}
	return true
}

func notFound(w http.ResponseWriter, err error, label string) bool {
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, label+" nicht gefunden")
		return true
	}
	return false
}
