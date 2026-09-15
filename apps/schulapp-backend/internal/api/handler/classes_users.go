package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"schulapp/internal/api"
	appmw "schulapp/internal/middleware"

	"golang.org/x/crypto/bcrypt"
)

func scanClass(row interface{ Scan(...any) error }) (api.Class, error) {
	var v api.Class
	var created time.Time
	var schoolID sql.NullInt64
	err := row.Scan(&v.Id, &v.Name, &v.SchoolYear, &schoolID, &created)
	if schoolID.Valid {
		x := int(schoolID.Int64)
		v.SchoolId = &x
	}
	v.CreatedAt = &created
	return v, err
}

func (h *Server) GetApiV1Classes(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	// superadmin/school_admin see all classes in scope (school), everyone else
	// only their member/assigned classes.
	query := `SELECT id,name,school_year,school_id,created_at FROM classes`
	args := []any{}
	var where []string
	if isSuperadmin(c.Role) {
		// no filter
	} else if c.Role == RoleSchoolAdmin {
		own, ok := h.ownSchoolID(r)
		if !ok {
			writeError(w, 403, "keine Schule zugeordnet")
			return
		}
		where = append(where, `school_id=$1`)
		args = append(args, own)
	} else {
		query += ` WHERE EXISTS(SELECT 1 FROM class_members cm WHERE cm.class_id=classes.id AND cm.user_id=$1) OR EXISTS(SELECT 1 FROM class_teachers ct WHERE ct.class_id=classes.id AND ct.user_id=$1)`
		args = append(args, c.UserID)
		h.queryClasses(w, r, query+` ORDER BY name`, args...)
		return
	}
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, " AND ")
	}
	query += ` ORDER BY name`
	h.queryClasses(w, r, query, args...)
}

func (h *Server) queryClasses(w http.ResponseWriter, r *http.Request, query string, args ...any) {
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
	c := h.claims(w, r)
	if !requireRole(w, c, RoleSuperadmin, RoleSchoolAdmin, RoleTeacher) {
		return
	}
	var req api.CreateClassRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.SchoolYear) == "" {
		writeError(w, 400, "name und school_year sind erforderlich")
		return
	}
	// school_id comes from the caller's profile, never from the body.
	var schoolID int
	if isSuperadmin(c.Role) {
		// Superadmins without a school fall back to the first school so
		// classes always belong somewhere.
		err := h.DB.QueryRowContext(r.Context(), `SELECT id FROM schools ORDER BY id LIMIT 1`).Scan(&schoolID)
		if err != nil {
			writeError(w, 500, "keine Schule vorhanden")
			return
		}
	} else {
		own, ok := h.ownSchoolID(r)
		if !ok {
			writeError(w, 403, "keine Schule zugeordnet")
			return
		}
		schoolID = own
	}
	v, err := scanClass(h.DB.QueryRowContext(r.Context(), `INSERT INTO classes(name,school_year,school_id) VALUES($1,$2,$3) RETURNING id,name,school_year,school_id,created_at`, strings.TrimSpace(req.Name), strings.TrimSpace(req.SchoolYear), schoolID))
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
	v, err := scanClass(h.DB.QueryRowContext(r.Context(), `SELECT id,name,school_year,school_id,created_at FROM classes WHERE id=$1`, id))
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
	v, err := scanClass(h.DB.QueryRowContext(r.Context(), `UPDATE classes SET name=COALESCE($1,name),school_year=COALESCE($2,school_year) WHERE id=$3 RETURNING id,name,school_year,school_id,created_at`, req.Name, req.SchoolYear, id))
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
	c := h.claims(w, r)
	// Delete stays privileged: superadmin everywhere, school_admin in own school.
	if !requireRole(w, c, RoleSuperadmin, RoleSchoolAdmin) {
		return
	}
	if !isSuperadmin(c.Role) {
		if _, ok := h.requireClassSchool(w, r, id); !ok {
			return
		}
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
	var targetSchool sql.NullInt64
	err := h.DB.QueryRowContext(r.Context(), `SELECT role, school_id FROM users WHERE id=$1 AND active`, req.UserId).Scan(&role, &targetSchool)
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
	if _, ok := h.requireClassSchool(w, r, id); !ok {
		return
	}
	// Members must belong to the class' school.
	if targetSchool.Valid {
		schoolID, _ := h.classSchoolID(r, id)
		if int(targetSchool.Int64) != schoolID {
			writeError(w, 400, "Schüler gehört zu einer anderen Schule")
			return
		}
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
	c := h.claims(w, r)
	if !requireRole(w, c, RoleSuperadmin, RoleSchoolAdmin) {
		return
	}
	if !isSuperadmin(c.Role) {
		if _, ok := h.requireClassSchool(w, r, id); !ok {
			return
		}
	}
	var req api.AddTeacherRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.UserId < 1 {
		writeError(w, 400, "user_id ist erforderlich")
		return
	}
	var role string
	var targetSchool sql.NullInt64
	err := h.DB.QueryRowContext(r.Context(), `SELECT role, school_id FROM users WHERE id=$1 AND active`, req.UserId).Scan(&role, &targetSchool)
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
	if !isSuperadmin(c.Role) && targetSchool.Valid {
		schoolID, _ := h.classSchoolID(r, id)
		if int(targetSchool.Int64) != schoolID {
			writeError(w, 400, "Lehrkraft gehört zu einer anderen Schule")
			return
		}
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
	c := h.claims(w, r)
	if !requireRole(w, c, RoleSuperadmin, RoleSchoolAdmin) {
		return
	}
	if !isSuperadmin(c.Role) {
		if _, ok := h.requireClassSchool(w, r, id); !ok {
			return
		}
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
	var schoolID sql.NullInt64
	err := row.Scan(&u.Id, &u.Email, &u.FirstName, &u.LastName, &u.Role, &schoolID)
	if schoolID.Valid {
		x := int(schoolID.Int64)
		u.SchoolId = &x
	}
	return u, err
}

const userSelect = `SELECT id,email,first_name,last_name,role,school_id FROM users`

func (h *Server) GetApiV1Users(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	// superadmin: all; school_admin: teachers (+students read-only scope) of own school.
	if !requireRole(w, c, RoleSuperadmin, RoleSchoolAdmin) {
		return
	}
	query := userSelect + ` WHERE active`
	args := []any{}
	if !isSuperadmin(c.Role) {
		own, ok := h.ownSchoolID(r)
		if !ok {
			writeError(w, 403, "keine Schule zugeordnet")
			return
		}
		query += ` AND school_id=$1`
		args = append(args, own)
	}
	query += ` ORDER BY last_name,first_name`
	rows, err := h.DB.QueryContext(r.Context(), query, args...)
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
	c := h.claims(w, r)
	if !requireRole(w, c, RoleSuperadmin, RoleSchoolAdmin) {
		return
	}
	var req api.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "ungültiges JSON")
		return
	}
	if err := validateCreateUser(req); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	// Role/school policy: superadmin may create any role (school_id optional,
	// NULL = platform admin). school_admin may only create teachers of the own
	// school - school_id always comes from the caller's profile.
	targetRole := string(req.Role)
	if isSuperadmin(c.Role) {
		if targetRole == RoleSuperadmin && req.SchoolId != nil {
			writeError(w, 400, "Superadmins gehören zu keiner Schule")
			return
		}
	} else {
		if targetRole != RoleTeacher {
			writeError(w, 403, "Schul-Admins dürfen nur Lehrkräfte anlegen")
			return
		}
		own, ok := h.ownSchoolID(r)
		if !ok {
			writeError(w, 403, "keine Schule zugeordnet")
			return
		}
		req.SchoolId = &own
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer tx.Rollback()
	u, err := createUser(r.Context(), tx, req)
	if err != nil {
		if isDuplicate(err) {
			writeError(w, 409, "E-Mail bereits vergeben")
			return
		}
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, u)
}

func (h *Server) PostApiV1UsersBulk(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if !requireRole(w, c, RoleSuperadmin, RoleSchoolAdmin) {
		return
	}
	var req api.BulkCreateUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "ungültiges JSON")
		return
	}
	if len(req.Users) == 0 || len(req.Users) > 200 {
		writeError(w, 400, "zwischen 1 und 200 Benutzern angeben")
		return
	}
	for _, user := range req.Users {
		if err := validateCreateUser(user); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if !isSuperadmin(c.Role) && string(user.Role) != RoleTeacher {
			writeError(w, 403, "Schul-Admins dürfen nur Lehrkräfte anlegen")
			return
		}
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer tx.Rollback()
	var ownSchool int
	var hasOwn bool
	if !isSuperadmin(c.Role) {
		ownSchool, hasOwn = h.ownSchoolID(r)
		if !hasOwn {
			writeError(w, 403, "keine Schule zugeordnet")
			return
		}
	}
	users := make([]api.User, 0, len(req.Users))
	for _, user := range req.Users {
		if hasOwn {
			s := ownSchool
			user.SchoolId = &s
		}
		created, err := createUser(r.Context(), tx, user)
		if err != nil {
			if isDuplicate(err) {
				writeError(w, 409, "mindestens eine E-Mail ist bereits vergeben")
				return
			}
			writeError(w, 500, "Datenbankfehler")
			return
		}
		users = append(users, created)
	}
	if err = tx.Commit(); err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, api.BulkCreateUsersResponse{Users: users})
}

func validateCreateUser(req api.CreateUserRequest) error {
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return errors.New("gültige E-Mail ist erforderlich")
	}
	if len(req.Password) < 8 {
		return errors.New("Passwort muss mindestens 8 Zeichen lang sein")
	}
	if strings.TrimSpace(req.FirstName) == "" || strings.TrimSpace(req.LastName) == "" {
		return errors.New("Vor- und Nachname sind erforderlich")
	}
	if !validRole(string(req.Role)) {
		return errors.New("ungültige Rolle")
	}
	return nil
}

func validRole(role string) bool {
	return role == RoleStudent || role == RoleTeacher || role == RoleSchoolAdmin || role == RoleSuperadmin || role == "admin"
}

func nullableSchoolID(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func createUser(ctx context.Context, tx *sql.Tx, req api.CreateUserRequest) (api.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return api.User{}, err
	}
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	return scanUser(tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,first_name,last_name,role,school_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,email,first_name,last_name,role,school_id`, email, string(hash), strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName), string(req.Role), nullableSchoolID(req.SchoolId)))
}

func isDuplicate(err error) bool {
	return strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique")
}
func (h *Server) GetApiV1UsersId(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if !isSuperadmin(c.Role) && c.UserID != id {
		writeError(w, 403, "keine Berechtigung")
		return
	}
	u, err := scanUser(h.DB.QueryRowContext(r.Context(), userSelect+` WHERE id=$1 AND active`, id))
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
	if !isSuperadmin(c.Role) && c.UserID != id {
		writeError(w, 403, "keine Berechtigung")
		return
	}
	var req api.UpdateUserRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, 400, "ungültiges JSON")
		return
	}
	if !isSuperadmin(c.Role) && (req.Role != nil || req.Password != nil) {
		writeError(w, 403, "Rolle und Passwort dürfen nur vom Superadmin geändert werden")
		return
	}
	if req.Role != nil && !validRole(string(*req.Role)) {
		writeError(w, 400, "ungültige Rolle")
		return
	}
	var passwordHash *string
	if req.Password != nil {
		if len(*req.Password) < 8 {
			writeError(w, 400, "Passwort muss mindestens 8 Zeichen lang sein")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, 500, "Passwort konnte nicht verarbeitet werden")
			return
		}
		value := string(hash)
		passwordHash = &value
	}
	u, err := scanUser(h.DB.QueryRowContext(r.Context(), `UPDATE users SET first_name=COALESCE($1,first_name),last_name=COALESCE($2,last_name),role=COALESCE($3,role),password_hash=COALESCE($4,password_hash),updated_at=NOW() WHERE id=$5 AND active RETURNING id,email,first_name,last_name,role,school_id`, req.FirstName, req.LastName, req.Role, passwordHash, id))
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
	if !requireRole(w, c, RoleSuperadmin, RoleSchoolAdmin) {
		return
	}
	if c.UserID == id {
		writeError(w, 400, "eigenes Konto kann nicht gelöscht werden")
		return
	}
	if !isSuperadmin(c.Role) {
		// school_admin may only deactivate teachers of the own school.
		var targetRole string
		var targetSchool sql.NullInt64
		if err := h.DB.QueryRowContext(r.Context(), `SELECT role, school_id FROM users WHERE id=$1 AND active`, id).Scan(&targetRole, &targetSchool); err != nil {
			if err == sql.ErrNoRows {
				writeError(w, 404, "Benutzer nicht gefunden")
				return
			}
			writeError(w, 500, "Datenbankfehler")
			return
		}
		if targetRole != RoleTeacher {
			writeError(w, 403, "Schul-Admins dürfen nur Lehrkräfte deaktivieren")
			return
		}
		own, ok := h.ownSchoolID(r)
		if !ok || !targetSchool.Valid || int(targetSchool.Int64) != own {
			writeError(w, 403, "kein Zugriff auf diese Schule")
			return
		}
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
