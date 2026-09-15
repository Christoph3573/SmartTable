package handler

import (
	"database/sql"
	"net/http"
	"time"

	"schulapp/internal/api"
)

func scanJoinRequest(row interface{ Scan(...any) error }) (api.JoinRequest, error) {
	var v api.JoinRequest
	var status string
	var decidedBy sql.NullInt64
	var created time.Time
	var decidedAt sql.NullTime
	var email, first, last sql.NullString
	err := row.Scan(&v.Id, &v.ClassId, &v.StudentId, &status, &decidedBy, &created, &decidedAt, &email, &first, &last)
	if err != nil {
		return v, err
	}
	_ = created
	_ = decidedAt
	v.Status = api.JoinRequestStatus(status)
	if decidedBy.Valid {
		x := int(decidedBy.Int64)
		v.DecidedBy = &x
	}
	v.CreatedAt = &created
	if decidedAt.Valid {
		x := decidedAt.Time
		v.DecidedAt = &x
	}
	if email.Valid {
		x := email.String
		v.StudentEmail = &x
	}
	if first.Valid {
		x := first.String
		v.StudentFirstName = &x
	}
	if last.Valid {
		x := last.String
		v.StudentLastName = &x
	}
	return v, nil
}

func joinRequestSelect() string {
	return `SELECT jr.id,jr.class_id,jr.student_id,jr.status,jr.decided_by,jr.created_at,jr.decided_at,u.email,u.first_name,u.last_name
		FROM class_join_requests jr JOIN users u ON u.id=jr.student_id`
}

// PostApiV1ClassesIdJoinRequests lets a student of the class' school request
// to join. Idempotent: an existing pending request is returned with 201.
func (h *Server) PostApiV1ClassesIdJoinRequests(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if c.Role != RoleStudent {
		writeError(w, 403, "nur Schüler können Beitrittsanfragen stellen")
		return
	}
	if _, ok := h.requireClassSchool(w, r, id); !ok {
		return
	}
	var alreadyMember bool
	if err := h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM class_members WHERE class_id=$1 AND user_id=$2)`, id, c.UserID).Scan(&alreadyMember); err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if alreadyMember {
		writeError(w, 409, "bereits Mitglied dieser Klasse")
		return
	}
	var existing api.JoinRequest
	err := scanJoinRequestRow(h.DB.QueryRowContext(r.Context(), joinRequestSelect()+` WHERE jr.class_id=$1 AND jr.student_id=$2`, id, c.UserID), &existing)
	if err == nil {
		if existing.Status == "pending" {
			writeJSON(w, 201, existing)
			return
		}
		// Rejected/approved requests can be re-requested: reset to pending.
		v, err := h.resetJoinRequest(r, existing.Id)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		writeJSON(w, 201, v)
		return
	}
	if err != sql.ErrNoRows {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	var newID int
	err = h.DB.QueryRowContext(r.Context(), `INSERT INTO class_join_requests(class_id,student_id) VALUES($1,$2) RETURNING id`, id, c.UserID).Scan(&newID)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	var v api.JoinRequest
	if err := scanJoinRequestRow(h.DB.QueryRowContext(r.Context(), joinRequestSelect()+` WHERE jr.id=$1`, newID), &v); err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, v)
}

func scanJoinRequestRow(row *sql.Row, v *api.JoinRequest) error {
	scanned, err := scanJoinRequest(row)
	if err != nil {
		return err
	}
	*v = scanned
	return nil
}

func (h *Server) resetJoinRequest(r *http.Request, id int) (api.JoinRequest, error) {
	var newID int
	err := h.DB.QueryRowContext(r.Context(), `UPDATE class_join_requests SET status='pending',decided_by=NULL,decided_at=NULL WHERE id=$1 RETURNING id`, id).Scan(&newID)
	if err != nil {
		return api.JoinRequest{}, err
	}
	var v api.JoinRequest
	err = scanJoinRequestRow(h.DB.QueryRowContext(r.Context(), joinRequestSelect()+` WHERE jr.id=$1`, id), &v)
	return v, err
}

// GetApiV1ClassesIdJoinRequests lists pending requests of a class.
// Teachers see their own classes, school_admins all of their school.
func (h *Server) GetApiV1ClassesIdJoinRequests(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if !requireRole(w, c, RoleSuperadmin, RoleSchoolAdmin, RoleTeacher) {
		return
	}
	if c.Role == RoleTeacher && !h.canManageClass(r, id) {
		writeError(w, 403, "keine Verwaltungsrechte für diese Klasse")
		return
	}
	if _, ok := h.requireClassSchool(w, r, id); !ok {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), joinRequestSelect()+` WHERE jr.class_id=$1 AND jr.status='pending' ORDER BY jr.created_at`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.JoinRequest{}
	for rows.Next() {
		v, err := scanJoinRequest(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}

func (h *Server) decideJoinRequest(w http.ResponseWriter, r *http.Request, id int, approve bool) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if !requireRole(w, c, RoleSuperadmin, RoleSchoolAdmin, RoleTeacher) {
		return
	}
	var classID, studentID int
	var status string
	err := h.DB.QueryRowContext(r.Context(), `SELECT class_id,student_id,status FROM class_join_requests WHERE id=$1`, id).Scan(&classID, &studentID, &status)
	if err == sql.ErrNoRows {
		writeError(w, 404, "Anfrage nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if status != "pending" {
		writeError(w, 409, "Anfrage wurde bereits entschieden")
		return
	}
	if c.Role == RoleTeacher && !h.canManageClass(r, classID) {
		writeError(w, 403, "keine Verwaltungsrechte für diese Klasse")
		return
	}
	if _, ok := h.requireClassSchool(w, r, classID); !ok {
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer tx.Rollback()
	newStatus := "rejected"
	if approve {
		newStatus = "approved"
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE class_join_requests SET status=$1,decided_by=$2,decided_at=NOW() WHERE id=$3 AND status='pending'`, newStatus, c.UserID, id); err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if approve {
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO class_members(class_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, classID, studentID); err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	var v api.JoinRequest
	if err := scanJoinRequestRow(h.DB.QueryRowContext(r.Context(), joinRequestSelect()+` WHERE jr.id=$1`, id), &v); err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 200, v)
}

func (h *Server) PostApiV1JoinRequestsIdApprove(w http.ResponseWriter, r *http.Request, id int) {
	h.decideJoinRequest(w, r, id, true)
}

func (h *Server) PostApiV1JoinRequestsIdReject(w http.ResponseWriter, r *http.Request, id int) {
	h.decideJoinRequest(w, r, id, false)
}
