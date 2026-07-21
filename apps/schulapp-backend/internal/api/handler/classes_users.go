package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"schulapp/internal/api"
	appmw "schulapp/internal/middleware"

	"golang.org/x/crypto/bcrypt"
)

func scanClass(row interface{ Scan(...any) error }) (api.Class, error) {
	var v api.Class
	var created time.Time
	err := row.Scan(&v.Id, &v.Name, &v.SchoolYear, &created)
	v.CreatedAt = &created
	return v, err
}

func (h *Server) GetApiV1Classes(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	query := `SELECT id,name,school_year,created_at FROM classes`
	args := []any{}
	if c.Role != "admin" {
		query += ` WHERE EXISTS(SELECT 1 FROM class_members cm WHERE cm.class_id=classes.id AND cm.user_id=$1) OR EXISTS(SELECT 1 FROM class_teachers ct WHERE ct.class_id=classes.id AND ct.user_id=$1)`
		args = append(args, c.UserID)
	}
	query += ` ORDER BY name`
	rows, err := h.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	result := []api.Class{}
	for rows.Next() {
		v, err := scanClass(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		result = append(result, v)
	}
	if err = rows.Err(); err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 200, result)
}

func (h *Server) PostApiV1Classes(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, h.claims(w, r), "admin") {
		return
	}
	var req api.CreateClassRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.SchoolYear) == "" {
		writeError(w, 400, "name und school_year sind erforderlich")
		return
	}
	v, err := scanClass(h.DB.QueryRowContext(r.Context(), `INSERT INTO classes(name,school_year) VALUES($1,$2) RETURNING id,name,school_year,created_at`, strings.TrimSpace(req.Name), strings.TrimSpace(req.SchoolYear)))
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, v)
}

func (h *Server) GetApiV1ClassesId(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireClassRead(w, r, id) {
		return
	}
	v, err := scanClass(h.DB.QueryRowContext(r.Context(), `SELECT id,name,school_year,created_at FROM classes WHERE id=$1`, id))
	if notFound(w, err, "Klasse") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 200, v)
}

func (h *Server) PatchApiV1ClassesId(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireClassManage(w, r, id) {
		return
	}
	var req api.UpdateClassRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, 400, "ungültiges JSON")
		return
	}
	v, err := scanClass(h.DB.QueryRowContext(r.Context(), `UPDATE classes SET name=COALESCE($1,name),school_year=COALESCE($2,school_year) WHERE id=$3 RETURNING id,name,school_year,created_at`, req.Name, req.SchoolYear, id))
	if notFound(w, err, "Klasse") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 200, v)
}

func (h *Server) DeleteApiV1ClassesId(w http.ResponseWriter, r *http.Request, id int) {
	if !requireRole(w, h.claims(w, r), "admin") {
		return
	}
	res, err := h.DB.ExecContext(r.Context(), `DELETE FROM classes WHERE id=$1`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, 404, "Klasse nicht gefunden")
		return
	}
	w.WriteHeader(204)
}

func (h *Server) GetApiV1ClassesIdMembers(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireClassRead(w, r, id) {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT u.id,u.email,u.first_name,u.last_name,u.role FROM class_members cm JOIN users u ON u.id=cm.user_id WHERE cm.class_id=$1 ORDER BY u.last_name,u.first_name`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.ClassMember{}
	for rows.Next() {
		var v api.ClassMember
		if err = rows.Scan(&v.UserId, &v.Email, &v.FirstName, &v.LastName, &v.Role); err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}
func (h *Server) PostApiV1ClassesIdMembers(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireClassManage(w, r, id) {
		return
	}
	var req api.AddMemberRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.UserId < 1 {
		writeError(w, 400, "user_id ist erforderlich")
		return
	}
	var role string
	err := h.DB.QueryRowContext(r.Context(), `SELECT role FROM users WHERE id=$1 AND active`, req.UserId).Scan(&role)
	if notFound(w, err, "Benutzer") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if role != "student" {
		writeError(w, 400, "nur Schüler können Klassenmitglieder sein")
		return
	}
	_, err = h.DB.ExecContext(r.Context(), `INSERT INTO class_members(class_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, req.UserId)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	w.WriteHeader(201)
}
func (h *Server) DeleteApiV1ClassesIdMembersUserId(w http.ResponseWriter, r *http.Request, id, userID int) {
	if !h.requireClassManage(w, r, id) {
		return
	}
	_, err := h.DB.ExecContext(r.Context(), `DELETE FROM class_members WHERE class_id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	w.WriteHeader(204)
}
func (h *Server) GetApiV1ClassesIdTeachers(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireClassRead(w, r, id) {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT u.id,u.email,u.first_name,u.last_name,ct.is_home_teacher FROM class_teachers ct JOIN users u ON u.id=ct.user_id WHERE ct.class_id=$1 ORDER BY u.last_name,u.first_name`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.ClassTeacher{}
	for rows.Next() {
		var v api.ClassTeacher
		if err = rows.Scan(&v.UserId, &v.Email, &v.FirstName, &v.LastName, &v.IsHomeTeacher); err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}
func (h *Server) PostApiV1ClassesIdTeachers(w http.ResponseWriter, r *http.Request, id int) {
	if !requireRole(w, h.claims(w, r), "admin") {
		return
	}
	var req api.AddTeacherRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.UserId < 1 {
		writeError(w, 400, "user_id ist erforderlich")
		return
	}
	var role string
	err := h.DB.QueryRowContext(r.Context(), `SELECT role FROM users WHERE id=$1 AND active`, req.UserId).Scan(&role)
	if notFound(w, err, "Benutzer") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if role != "teacher" {
		writeError(w, 400, "nur Lehrkräfte können zugewiesen werden")
		return
	}
	home := false
	if req.IsHomeTeacher != nil {
		home = *req.IsHomeTeacher
	}
	_, err = h.DB.ExecContext(r.Context(), `INSERT INTO class_teachers(class_id,user_id,is_home_teacher) VALUES($1,$2,$3) ON CONFLICT(class_id,user_id) DO UPDATE SET is_home_teacher=EXCLUDED.is_home_teacher`, id, req.UserId, home)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	w.WriteHeader(201)
}
func (h *Server) DeleteApiV1ClassesIdTeachersUserId(w http.ResponseWriter, r *http.Request, id, userID int) {
	if !requireRole(w, h.claims(w, r), "admin") {
		return
	}
	_, err := h.DB.ExecContext(r.Context(), `DELETE FROM class_teachers WHERE class_id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	w.WriteHeader(204)
}

func scanUser(row interface{ Scan(...any) error }) (api.User, error) {
	var u api.User
	err := row.Scan(&u.Id, &u.Email, &u.FirstName, &u.LastName, &u.Role)
	return u, err
}
func (h *Server) GetApiV1Users(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, h.claims(w, r), "admin") {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,email,first_name,last_name,role FROM users WHERE active ORDER BY last_name,first_name`)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, u)
	}
	writeJSON(w, 200, out)
}
func (h *Server) PostApiV1Users(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, h.claims(w, r), "admin") {
		return
	}
	var req api.LoginRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(string(req.Email)) == "" || len(req.Password) < 8 {
		writeError(w, 400, "E-Mail und Passwort (mindestens 8 Zeichen) sind erforderlich")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "Passwort konnte nicht verarbeitet werden")
		return
	}
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	name := strings.Split(email, "@")[0]
	u, err := scanUser(h.DB.QueryRowContext(r.Context(), `INSERT INTO users(email,password_hash,first_name,last_name,role) VALUES($1,$2,$3,$4,'student') RETURNING id,email,first_name,last_name,role`, email, string(hash), name, ""))
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			writeError(w, 409, "E-Mail bereits vergeben")
			return
		}
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, u)
}
func (h *Server) GetApiV1UsersId(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if c.Role != "admin" && c.UserID != id {
		writeError(w, 403, "keine Berechtigung")
		return
	}
	u, err := scanUser(h.DB.QueryRowContext(r.Context(), `SELECT id,email,first_name,last_name,role FROM users WHERE id=$1 AND active`, id))
	if notFound(w, err, "Benutzer") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 200, u)
}
func (h *Server) PatchApiV1UsersId(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if c.Role != "admin" && c.UserID != id {
		writeError(w, 403, "keine Berechtigung")
		return
	}
	var req api.UpdateUserRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, 400, "ungültiges JSON")
		return
	}
	u, err := scanUser(h.DB.QueryRowContext(r.Context(), `UPDATE users SET first_name=COALESCE($1,first_name),last_name=COALESCE($2,last_name),updated_at=NOW() WHERE id=$3 AND active RETURNING id,email,first_name,last_name,role`, req.FirstName, req.LastName, id))
	if notFound(w, err, "Benutzer") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 200, u)
}
func (h *Server) DeleteApiV1UsersId(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if !requireRole(w, c, "admin") {
		return
	}
	if c.UserID == id {
		writeError(w, 400, "eigenes Konto kann nicht gelöscht werden")
		return
	}
	res, err := h.DB.ExecContext(r.Context(), `UPDATE users SET active=false,updated_at=NOW() WHERE id=$1 AND active`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, 404, "Benutzer nicht gefunden")
		return
	}
	w.WriteHeader(204)
}

var _ = sql.ErrNoRows
var _ = appmw.Claims{}
