package handler

import (
	"database/sql"
	"net/http"

	appmw "schulapp/internal/middleware"
)

// Role constants for the school hierarchy:
// superadmin (platform) -> school_admin (school) -> teacher -> student.
const (
	RoleStudent     = "student"
	RoleTeacher     = "teacher"
	RoleSchoolAdmin = "school_admin"
	RoleSuperadmin  = "superadmin"
)

// isSuperadmin treats legacy "admin" tokens/rows like superadmin, so old
// deployments keep working until new tokens are issued.
func isSuperadmin(role string) bool {
	return role == RoleSuperadmin || role == "admin"
}

func isStaff(c *appmw.Claims) bool {
	return c != nil && (c.Role == RoleTeacher || c.Role == RoleSchoolAdmin || isSuperadmin(c.Role))
}

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
	// Legacy "admin" acts as superadmin everywhere.
	if isSuperadmin(c.Role) {
		for _, role := range roles {
			if role == RoleSuperadmin || role == "admin" {
				return true
			}
		}
	}
	for _, role := range roles {
		if c.Role == role {
			return true
		}
	}
	writeError(w, http.StatusForbidden, "keine Berechtigung")
	return false
}

// ownSchoolID returns the school of the authenticated user (NULL for superadmin).
func (h *Server) ownSchoolID(r *http.Request) (int, bool) {
	c := appmw.GetClaims(r)
	if c == nil {
		return 0, false
	}
	if isSuperadmin(c.Role) {
		return 0, true
	}
	var id sql.NullInt64
	if err := h.DB.QueryRowContext(r.Context(), `SELECT school_id FROM users WHERE id=$1 AND active`, c.UserID).Scan(&id); err != nil || !id.Valid {
		return 0, false
	}
	return int(id.Int64), true
}

// requireOwnSchool ensures the caller's school matches the given school.
// Superadmins pass for any school.
func (h *Server) requireOwnSchool(w http.ResponseWriter, r *http.Request, schoolID int) bool {
	c := h.claims(w, r)
	if c == nil {
		return false
	}
	if isSuperadmin(c.Role) {
		return true
	}
	own, ok := h.ownSchoolID(r)
	if !ok || own != schoolID {
		writeError(w, http.StatusForbidden, "kein Zugriff auf diese Schule")
		return false
	}
	return true
}

// classSchoolID returns the school a class belongs to.
func (h *Server) classSchoolID(r *http.Request, classID int) (int, error) {
	var schoolID int
	err := h.DB.QueryRowContext(r.Context(), `SELECT school_id FROM classes WHERE id=$1`, classID).Scan(&schoolID)
	return schoolID, err
}

// requireClassSchool ensures the caller may act within the class' school:
// superadmin everywhere, school_admin/teacher/student only in their own school.
func (h *Server) requireClassSchool(w http.ResponseWriter, r *http.Request, classID int) (int, bool) {
	c := h.claims(w, r)
	if c == nil {
		return 0, false
	}
	schoolID, err := h.classSchoolID(r, classID)
	if notFound(w, err, "Klasse") {
		return 0, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return 0, false
	}
	if isSuperadmin(c.Role) {
		return schoolID, true
	}
	own, ok := h.ownSchoolID(r)
	if !ok || own != schoolID {
		writeError(w, http.StatusForbidden, "kein Zugriff auf diese Schule")
		return 0, false
	}
	return schoolID, true
}

func (h *Server) canReadClass(r *http.Request, classID int) bool {
	c := appmw.GetClaims(r)
	if c == nil {
		return false
	}
	if isSuperadmin(c.Role) {
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
	if isSuperadmin(c.Role) {
		return true
	}
	// school_admins manage every class of their own school.
	if c.Role == RoleSchoolAdmin {
		schoolID, err := h.classSchoolID(r, classID)
		if err != nil {
			return false
		}
		own, ok := h.ownSchoolID(r)
		return ok && own == schoolID
	}
	if c.Role != RoleTeacher {
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
