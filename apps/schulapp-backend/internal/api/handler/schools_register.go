package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"schulapp/internal/api"

	"golang.org/x/crypto/bcrypt"
)

func scanSchool(row interface{ Scan(...any) error }) (api.School, error) {
	var v api.School
	var created time.Time
	err := row.Scan(&v.Id, &v.Name, &created)
	v.CreatedAt = &created
	return v, err
}

func (h *Server) GetApiV1PublicSchools(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,name,created_at FROM schools ORDER BY name`)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.School{}
	for rows.Next() {
		v, err := scanSchool(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}

func (h *Server) GetApiV1PublicClasses(w http.ResponseWriter, r *http.Request, p api.GetApiV1PublicClassesParams) {
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,name,school_year,school_id,created_at FROM classes WHERE school_id=$1 ORDER BY name`, p.SchoolId)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.Class{}
	for rows.Next() {
		v, err := scanClass(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}

// PostApiV1AuthRegister is fully public: any student may register with a
// school (and optionally request to join one class of that school).
// Always returns a generic 201 on success and never leaks which emails exist:
// duplicate emails yield 409 without further detail.
func (h *Server) PostApiV1AuthRegister(w http.ResponseWriter, r *http.Request) {
	if h.RegisterLimiter != nil && !h.RegisterLimiter.Allow() {
		writeError(w, http.StatusTooManyRequests, "zu viele Anfragen")
		return
	}
	var req api.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "ungültiges JSON")
		return
	}
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		writeError(w, 400, "gültige E-Mail ist erforderlich")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, 400, "Passwort muss mindestens 8 Zeichen lang sein")
		return
	}
	if strings.TrimSpace(req.FirstName) == "" || strings.TrimSpace(req.LastName) == "" {
		writeError(w, 400, "Vor- und Nachname sind erforderlich")
		return
	}
	if req.SchoolId < 1 {
		writeError(w, 400, "Schule ist erforderlich")
		return
	}
	var schoolExists bool
	if err := h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM schools WHERE id=$1)`, req.SchoolId).Scan(&schoolExists); err != nil || !schoolExists {
		writeError(w, 400, "Schule nicht gefunden")
		return
	}
	classID := 0
	if req.RequestedClassId != nil {
		classID = *req.RequestedClassId
		var classSchool int
		err := h.DB.QueryRowContext(r.Context(), `SELECT school_id FROM classes WHERE id=$1`, classID).Scan(&classSchool)
		if err == sql.ErrNoRows {
			writeError(w, 400, "Klasse nicht gefunden")
			return
		}
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		if classSchool != req.SchoolId {
			writeError(w, 400, "Klasse gehört nicht zu dieser Schule")
			return
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "Registrierung fehlgeschlagen")
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer tx.Rollback()
	u, err := scanUser(tx.QueryRowContext(r.Context(), `INSERT INTO users(email,password_hash,first_name,last_name,role,school_id) VALUES($1,$2,$3,$4,'student',$5) RETURNING id,email,first_name,last_name,role,school_id`, email, string(hash), strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName), req.SchoolId))
	if err != nil {
		if isDuplicate(err) {
			writeError(w, 409, "E-Mail bereits vergeben")
			return
		}
		log.Printf("register: user insert failed (email=%s school_id=%d): %v", email, req.SchoolId, err)
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if classID > 0 {
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO class_join_requests(class_id,student_id) VALUES($1,$2) ON CONFLICT(class_id,student_id) DO NOTHING`, classID, u.Id); err != nil {
			log.Printf("register: join_request insert failed (user_id=%d class_id=%d): %v", u.Id, classID, err)
			writeError(w, 500, "Datenbankfehler")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("register: tx commit failed (email=%s): %v", email, err)
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, u)
}

func (h *Server) GetApiV1Schools(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	query := `SELECT id,name,created_at FROM schools`
	args := []any{}
	// Staff only see their own school; superadmins see everything.
	if !isSuperadmin(c.Role) {
		own, ok := h.ownSchoolID(r)
		if !ok {
			writeJSON(w, 200, []api.School{})
			return
		}
		query += ` WHERE id=$1`
		args = append(args, own)
	}
	query += ` ORDER BY name`
	rows, err := h.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.School{}
	for rows.Next() {
		v, err := scanSchool(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}

func (h *Server) PostApiV1Schools(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, h.claims(w, r), RoleSuperadmin) {
		return
	}
	var req api.CreateSchoolRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
		writeError(w, 400, "Schulname ist erforderlich")
		return
	}
	v, err := scanSchool(h.DB.QueryRowContext(r.Context(), `INSERT INTO schools(name) VALUES($1) RETURNING id,name,created_at`, strings.TrimSpace(req.Name)))
	if err != nil {
		if isDuplicate(err) {
			writeError(w, 409, "Schule existiert bereits")
			return
		}
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, v)
}

// GetApiV1MeMembership returns the caller's school, their classes and their
// own pending join requests - used for the "join a class" banner.
func (h *Server) GetApiV1MeMembership(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	var out api.Membership
	out.Classes = []api.Class{}
	out.Pending = []api.JoinRequest{}

	var schoolID sql.NullInt64
	_ = h.DB.QueryRowContext(r.Context(), `SELECT school_id FROM users WHERE id=$1`, c.UserID).Scan(&schoolID)
	if schoolID.Valid {
		s, err := scanSchool(h.DB.QueryRowContext(r.Context(), `SELECT id,name,created_at FROM schools WHERE id=$1`, int(schoolID.Int64)))
		if err == nil {
			out.School = &s
		}
	}

	rows, err := h.DB.QueryContext(r.Context(), `SELECT c.id,c.name,c.school_year,c.school_id,c.created_at FROM classes c WHERE EXISTS(SELECT 1 FROM class_members cm WHERE cm.class_id=c.id AND cm.user_id=$1) OR EXISTS(SELECT 1 FROM class_teachers ct WHERE ct.class_id=c.id AND ct.user_id=$1) ORDER BY c.name`, c.UserID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			v, err := scanClass(rows)
			if err != nil {
				break
			}
			out.Classes = append(out.Classes, v)
		}
		rows.Close()
	}

	if c.Role == RoleStudent {
		reqs, err := h.DB.QueryContext(r.Context(), `SELECT jr.id,jr.class_id,jr.student_id,jr.status,jr.decided_by,jr.created_at,jr.decided_at FROM class_join_requests jr WHERE jr.student_id=$1 AND jr.status='pending' ORDER BY jr.created_at DESC`, c.UserID)
		if err == nil {
			defer reqs.Close()
			for reqs.Next() {
				v, err := scanJoinRequest(reqs)
				if err != nil {
					break
				}
				out.Pending = append(out.Pending, v)
			}
			reqs.Close()
		}
	}
	writeJSON(w, 200, out)
}
