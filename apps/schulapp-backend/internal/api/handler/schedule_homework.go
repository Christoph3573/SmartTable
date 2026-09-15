package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"schulapp/internal/api"
	appmw "schulapp/internal/middleware"
)

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}
func nullableString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}
func nullableBool(v *bool) any {
	if v == nil {
		return nil
	}
	return *v
}
func nullableTime(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}
func nullableDate(v *openapi_types.Date) any {
	if v == nil {
		return nil
	}
	return v.Time
}

func scanSub(row interface{ Scan(...any) error }) (api.Substitution, error) {
	var v api.Substitution
	var date time.Time
	var classID, subjectID, orig, sub sql.NullInt64
	var room, note sql.NullString
	var created time.Time
	var typ string
	err := row.Scan(&v.Id, &date, &v.Period, &classID, &subjectID, &orig, &sub, &room, &typ, &note, &created)
	if err == nil {
		v.Date = openapi_types.Date{Time: date}
		if classID.Valid {
			x := int(classID.Int64)
			v.ClassId = &x
		}
		if subjectID.Valid {
			x := int(subjectID.Int64)
			v.SubjectId = &x
		}
		if orig.Valid {
			x := int(orig.Int64)
			v.OriginalTeacherId = &x
		}
		if sub.Valid {
			x := int(sub.Int64)
			v.SubTeacherId = &x
		}
		if room.Valid {
			x := room.String
			v.Room = &x
		}
		if note.Valid {
			x := note.String
			v.Note = &x
		}
		v.Type = api.SubstitutionType(typ)
		v.CreatedAt = &created
	}
	return v, err
}
func (h *Server) GetApiV1Substitutions(w http.ResponseWriter, r *http.Request, p api.GetApiV1SubstitutionsParams) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	var from, to any
	if p.DateFrom != nil {
		from = p.DateFrom.Time
	}
	if p.DateTo != nil {
		to = p.DateTo.Time
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,date,period,class_id,subject_id,original_teacher_id,sub_teacher_id,room,type,note,created_at FROM substitutions WHERE (date >= $1::date OR $1::date IS NULL) AND (date <= $2::date OR $2::date IS NULL) AND (class_id=$3 OR $3 IS NULL) AND ($4='superadmin' OR $4='admin' OR class_id IS NULL OR EXISTS(SELECT 1 FROM class_members cm WHERE cm.class_id=substitutions.class_id AND cm.user_id=$5) OR EXISTS(SELECT 1 FROM class_teachers ct WHERE ct.class_id=substitutions.class_id AND ct.user_id=$5)) ORDER BY date,period`, from, to, nullableInt(p.ClassId), c.Role, c.UserID)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.Substitution{}
	for rows.Next() {
		v, err := scanSub(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}
func (h *Server) PostApiV1Substitutions(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, h.claims(w, r), "teacher", "superadmin") {
		return
	}
	var req api.CreateSubstitutionRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Period < 1 || !req.Type.Valid() {
		writeError(w, 400, "ungültige Vertretung")
		return
	}
	if req.ClassId != nil && !h.canManageClass(r, *req.ClassId) {
		writeError(w, 403, "keine Verwaltungsrechte für diese Klasse")
		return
	}
	v, err := scanSub(h.DB.QueryRowContext(r.Context(), `INSERT INTO substitutions(date,period,class_id,subject_id,original_teacher_id,sub_teacher_id,room,type,note) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,date,period,class_id,subject_id,original_teacher_id,sub_teacher_id,room,type,note,created_at`, req.Date.Time, req.Period, nullableInt(req.ClassId), nullableInt(req.SubjectId), nullableInt(req.OriginalTeacherId), nullableInt(req.SubTeacherId), nullableString(req.Room), req.Type, nullableString(req.Note)))
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, v)
}
func (h *Server) substitutionClass(r *http.Request, id int) (int, error) {
	var classID sql.NullInt64
	err := h.DB.QueryRowContext(r.Context(), `SELECT class_id FROM substitutions WHERE id=$1`, id).Scan(&classID)
	if err != nil {
		return 0, err
	}
	if !classID.Valid {
		return 0, nil
	}
	return int(classID.Int64), nil
}
func (h *Server) PatchApiV1SubstitutionsId(w http.ResponseWriter, r *http.Request, id int) {
	if !requireRole(w, h.claims(w, r), "teacher", "superadmin") {
		return
	}
	old, err := h.substitutionClass(r, id)
	if notFound(w, err, "Vertretung") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if old != 0 && !h.canManageClass(r, old) {
		writeError(w, 403, "keine Verwaltungsrechte")
		return
	}
	var req api.UpdateSubstitutionRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || (req.Type != nil && !req.Type.Valid()) {
		writeError(w, 400, "ungültige Vertretung")
		return
	}
	if req.ClassId != nil && !h.canManageClass(r, *req.ClassId) {
		writeError(w, 403, "keine Verwaltungsrechte")
		return
	}
	var typ any
	if req.Type != nil {
		typ = string(*req.Type)
	}
	v, err := scanSub(h.DB.QueryRowContext(r.Context(), `UPDATE substitutions SET date=COALESCE($1,date),period=COALESCE($2,period),class_id=COALESCE($3,class_id),subject_id=COALESCE($4,subject_id),original_teacher_id=COALESCE($5,original_teacher_id),sub_teacher_id=COALESCE($6,sub_teacher_id),room=COALESCE($7,room),type=COALESCE($8,type),note=COALESCE($9,note) WHERE id=$10 RETURNING id,date,period,class_id,subject_id,original_teacher_id,sub_teacher_id,room,type,note,created_at`, nullableDate(req.Date), nullableInt(req.Period), nullableInt(req.ClassId), nullableInt(req.SubjectId), nullableInt(req.OriginalTeacherId), nullableInt(req.SubTeacherId), nullableString(req.Room), typ, nullableString(req.Note), id))
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 200, v)
}
func (h *Server) DeleteApiV1SubstitutionsId(w http.ResponseWriter, r *http.Request, id int) {
	if !requireRole(w, h.claims(w, r), "teacher", "superadmin") {
		return
	}
	classID, err := h.substitutionClass(r, id)
	if notFound(w, err, "Vertretung") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if classID != 0 && !h.canManageClass(r, classID) {
		writeError(w, 403, "keine Verwaltungsrechte")
		return
	}
	_, err = h.DB.ExecContext(r.Context(), `DELETE FROM substitutions WHERE id=$1`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	w.WriteHeader(204)
}

func scanEvent(row interface{ Scan(...any) error }) (api.Event, error) {
	var v api.Event
	var all bool
	var classID, creator sql.NullInt64
	var created time.Time
	var typ string
	err := row.Scan(&v.Id, &v.Title, &v.StartTime, &v.EndTime, &all, &typ, &classID, &creator, &created)
	if err == nil {
		v.AllDay = &all
		v.Type = api.EventType(typ)
		if classID.Valid {
			x := int(classID.Int64)
			v.ClassId = &x
		}
		if creator.Valid {
			x := int(creator.Int64)
			v.CreatorId = &x
		}
		v.CreatedAt = &created
	}
	return v, err
}
func (h *Server) GetApiV1Events(w http.ResponseWriter, r *http.Request, p api.GetApiV1EventsParams) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	var start, end any
	if p.StartDate != nil {
		start = p.StartDate.Time
	}
	if p.EndDate != nil {
		end = p.EndDate.Time.AddDate(0, 0, 1)
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,title,start_time,end_time,all_day,type,class_id,creator_id,created_at FROM events WHERE (start_time >= $1 OR $1 IS NULL) AND (end_time <= $2 OR $2 IS NULL) AND (class_id=$3 OR $3 IS NULL) AND ($4='superadmin' OR $4='admin' OR class_id IS NULL OR EXISTS(SELECT 1 FROM class_members cm WHERE cm.class_id=events.class_id AND cm.user_id=$5) OR EXISTS(SELECT 1 FROM class_teachers ct WHERE ct.class_id=events.class_id AND ct.user_id=$5)) ORDER BY start_time`, start, end, nullableInt(p.ClassId), c.Role, c.UserID)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.Event{}
	for rows.Next() {
		v, err := scanEvent(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}
func (h *Server) PostApiV1Events(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if !requireRole(w, c, "teacher", "superadmin") {
		return
	}
	var req api.CreateEventRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Title == "" || !req.EndTime.After(req.StartTime) || !req.Type.Valid() {
		writeError(w, 400, "ungültiges Event")
		return
	}
	if req.ClassId != nil && !h.canManageClass(r, *req.ClassId) {
		writeError(w, 403, "keine Verwaltungsrechte")
		return
	}
	all := false
	if req.AllDay != nil {
		all = *req.AllDay
	}
	v, err := scanEvent(h.DB.QueryRowContext(r.Context(), `INSERT INTO events(title,start_time,end_time,all_day,type,class_id,creator_id) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,title,start_time,end_time,all_day,type,class_id,creator_id,created_at`, req.Title, req.StartTime, req.EndTime, all, req.Type, nullableInt(req.ClassId), c.UserID))
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, v)
}
func (h *Server) eventAccess(r *http.Request, id int) (bool, error) {
	c := appmw.GetClaims(r)
	var classID, creator sql.NullInt64
	err := h.DB.QueryRowContext(r.Context(), `SELECT class_id,creator_id FROM events WHERE id=$1`, id).Scan(&classID, &creator)
	if err != nil {
		return false, err
	}
	if isSuperadmin(c.Role) || (creator.Valid && int(creator.Int64) == c.UserID) {
		return true, nil
	}
	return classID.Valid && h.canManageClass(r, int(classID.Int64)), nil
}
func (h *Server) PatchApiV1EventsId(w http.ResponseWriter, r *http.Request, id int) {
	if h.claims(w, r) == nil {
		return
	}
	ok, err := h.eventAccess(r, id)
	if notFound(w, err, "Event") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if !ok {
		writeError(w, 403, "keine Berechtigung")
		return
	}
	var req api.UpdateEventRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || (req.Type != nil && !req.Type.Valid()) {
		writeError(w, 400, "ungültiges Event")
		return
	}
	if req.ClassId != nil && !h.canManageClass(r, *req.ClassId) {
		writeError(w, 403, "keine Verwaltungsrechte")
		return
	}
	var typ any
	if req.Type != nil {
		typ = string(*req.Type)
	}
	v, err := scanEvent(h.DB.QueryRowContext(r.Context(), `UPDATE events SET title=COALESCE($1,title),start_time=COALESCE($2,start_time),end_time=COALESCE($3,end_time),all_day=COALESCE($4,all_day),type=COALESCE($5,type),class_id=COALESCE($6,class_id) WHERE id=$7 AND COALESCE($3,end_time)>COALESCE($2,start_time) RETURNING id,title,start_time,end_time,all_day,type,class_id,creator_id,created_at`, nullableString(req.Title), nullableTime(req.StartTime), nullableTime(req.EndTime), nullableBool(req.AllDay), typ, nullableInt(req.ClassId), id))
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, 400, "Endzeit muss nach Startzeit liegen")
			return
		}
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 200, v)
}
func (h *Server) DeleteApiV1EventsId(w http.ResponseWriter, r *http.Request, id int) {
	if h.claims(w, r) == nil {
		return
	}
	ok, err := h.eventAccess(r, id)
	if notFound(w, err, "Event") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if !ok {
		writeError(w, 403, "keine Berechtigung")
		return
	}
	_, err = h.DB.ExecContext(r.Context(), `DELETE FROM events WHERE id=$1`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	w.WriteHeader(204)
}

func scanHomework(row interface{ Scan(...any) error }) (api.Homework, error) {
	var v api.Homework
	var due time.Time
	var desc sql.NullString
	var created time.Time
	err := row.Scan(&v.Id, &v.Title, &desc, &due, &v.ClassId, &v.SubjectId, &v.TeacherId, &created)
	if err == nil {
		v.DueDate = openapi_types.Date{Time: due}
		if desc.Valid {
			x := desc.String
			v.Description = &x
		}
		v.CreatedAt = &created
	}
	return v, err
}
func (h *Server) GetApiV1ClassesIdHomework(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireClassRead(w, r, id) {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,title,description,due_date,class_id,subject_id,teacher_id,created_at FROM homework WHERE class_id=$1 ORDER BY due_date`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.Homework{}
	for rows.Next() {
		v, err := scanHomework(rows)
		if err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}
func (h *Server) PostApiV1ClassesIdHomework(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if !h.canManageClass(r, id) {
		writeError(w, 403, "keine Verwaltungsrechte")
		return
	}
	var req api.CreateHomeworkRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Title == "" || req.SubjectId < 1 {
		writeError(w, 400, "Titel und subject_id sind erforderlich")
		return
	}
	v, err := scanHomework(h.DB.QueryRowContext(r.Context(), `INSERT INTO homework(title,description,due_date,class_id,subject_id,teacher_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,title,description,due_date,class_id,subject_id,teacher_id,created_at`, req.Title, nullableString(req.Description), req.DueDate.Time, id, req.SubjectId, c.UserID))
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, v)
}
func (h *Server) homeworkAccess(r *http.Request, id int) (api.Homework, bool, error) {
	v, err := scanHomework(h.DB.QueryRowContext(r.Context(), `SELECT id,title,description,due_date,class_id,subject_id,teacher_id,created_at FROM homework WHERE id=$1`, id))
	if err != nil {
		return v, false, err
	}
	c := appmw.GetClaims(r)
	return v, isSuperadmin(c.Role) || v.TeacherId == c.UserID || h.canManageClass(r, v.ClassId), nil
}
func (h *Server) PatchApiV1HomeworkId(w http.ResponseWriter, r *http.Request, id int) {
	if h.claims(w, r) == nil {
		return
	}
	_, ok, err := h.homeworkAccess(r, id)
	if notFound(w, err, "Hausaufgabe") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if !ok {
		writeError(w, 403, "keine Berechtigung")
		return
	}
	var req api.UpdateHomeworkRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, 400, "ungültiges JSON")
		return
	}
	v, err := scanHomework(h.DB.QueryRowContext(r.Context(), `UPDATE homework SET title=COALESCE($1,title),description=COALESCE($2,description),due_date=COALESCE($3,due_date),subject_id=COALESCE($4,subject_id) WHERE id=$5 RETURNING id,title,description,due_date,class_id,subject_id,teacher_id,created_at`, nullableString(req.Title), nullableString(req.Description), nullableDate(req.DueDate), nullableInt(req.SubjectId), id))
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 200, v)
}
func (h *Server) DeleteApiV1HomeworkId(w http.ResponseWriter, r *http.Request, id int) {
	if h.claims(w, r) == nil {
		return
	}
	_, ok, err := h.homeworkAccess(r, id)
	if notFound(w, err, "Hausaufgabe") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if !ok {
		writeError(w, 403, "keine Berechtigung")
		return
	}
	_, err = h.DB.ExecContext(r.Context(), `DELETE FROM homework WHERE id=$1`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	w.WriteHeader(204)
}
func (h *Server) GetApiV1HomeworkIdSubmissions(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	hw, manage, err := h.homeworkAccess(r, id)
	if notFound(w, err, "Hausaufgabe") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if !manage && c.Role != "student" {
		writeError(w, 403, "keine Berechtigung")
		return
	}
	if !manage && !h.canReadClass(r, hw.ClassId) {
		writeError(w, 403, "keine Berechtigung")
		return
	}
	q := `SELECT id,homework_id,student_id,status,grade,submitted_at,graded_at,file_id FROM homework_submissions WHERE homework_id=$1`
	args := []any{id}
	if !manage {
		q += ` AND student_id=$2`
		args = append(args, c.UserID)
	}
	rows, err := h.DB.QueryContext(r.Context(), q, args...)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.HomeworkSubmission{}
	for rows.Next() {
		var v api.HomeworkSubmission
		var grade sql.NullFloat64
		var submitted, graded sql.NullTime
		var status string
		var fileID sql.NullInt64
		if err = rows.Scan(&v.Id, &v.HomeworkId, &v.StudentId, &status, &grade, &submitted, &graded, &fileID); err != nil {
			writeError(w, 500, "Datenbankfehler")
			return
		}
		v.Status = api.HomeworkSubmissionStatus(status)
		if grade.Valid {
			x := float32(grade.Float64)
			v.Grade = &x
		}
		if submitted.Valid {
			x := submitted.Time
			v.SubmittedAt = &x
		}
		if graded.Valid {
			x := graded.Time
			v.GradedAt = &x
		}
		if fileID.Valid {
			x := int(fileID.Int64)
			v.FileId = &x
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}
func (h *Server) PostApiV1HomeworkIdSubmissions(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if c.Role != "student" {
		writeError(w, 403, "nur Schüler können abgeben")
		return
	}
	hw, _, err := h.homeworkAccess(r, id)
	if notFound(w, err, "Hausaufgabe") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if !h.canReadClass(r, hw.ClassId) {
		writeError(w, 403, "kein Zugriff auf diese Klasse")
		return
	}
	var existingStatus string
	var existingFileID sql.NullInt64
	err = h.DB.QueryRowContext(r.Context(), `SELECT status,file_id FROM homework_submissions WHERE homework_id=$1 AND student_id=$2`, id, c.UserID).Scan(&existingStatus, &existingFileID)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if existingStatus == "graded" {
		writeError(w, 400, "bereits benotete Abgaben können nicht mehr bearbeitet werden")
		return
	}
	var fileID any
	newFileID, err := h.uploadSubmissionFile(w, r, hw.ClassId, c.UserID)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if newFileID > 0 {
		fileID = newFileID
	}
	var v api.HomeworkSubmission
	var grade sql.NullFloat64
	var submitted, graded sql.NullTime
	var status string
	var scannedFileID sql.NullInt64
	err = h.DB.QueryRowContext(r.Context(), `INSERT INTO homework_submissions(homework_id,student_id,status,submitted_at,file_id) VALUES($1,$2,'submitted',NOW(),$3)
		ON CONFLICT(homework_id,student_id) DO UPDATE SET status='submitted',submitted_at=NOW(),file_id=COALESCE(EXCLUDED.file_id,homework_submissions.file_id)
		RETURNING id,homework_id,student_id,status,grade,submitted_at,graded_at,file_id`, id, c.UserID, fileID).
		Scan(&v.Id, &v.HomeworkId, &v.StudentId, &status, &grade, &submitted, &graded, &scannedFileID)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	// A replaced attachment leaves the previous upload orphaned - clean it up
	// now that the new file_id is committed.
	if newFileID > 0 && existingFileID.Valid && int(existingFileID.Int64) != newFileID {
		h.deleteFileByID(r, int(existingFileID.Int64))
	}
	v.Status = api.HomeworkSubmissionStatus(status)
	if grade.Valid {
		x := float32(grade.Float64)
		v.Grade = &x
	}
	if submitted.Valid {
		x := submitted.Time
		v.SubmittedAt = &x
	}
	if graded.Valid {
		x := graded.Time
		v.GradedAt = &x
	}
	if scannedFileID.Valid {
		x := int(scannedFileID.Int64)
		v.FileId = &x
	}
	writeJSON(w, 201, v)
}

// uploadSubmissionFile reads an optional multipart "file" field and stores it like a
// regular class file, returning its new id (0 if no file was attached).
func (h *Server) uploadSubmissionFile(w http.ResponseWriter, r *http.Request, classID int, uploaderID int) (int, error) {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/form-data") {
		return 0, nil
	}
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		return 0, fmt.Errorf("ungültiger Upload (maximal 20 MB)")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return 0, nil
	}
	defer file.Close()
	if err = os.MkdirAll(h.UploadDir, 0750); err != nil {
		return 0, fmt.Errorf("Speicher nicht verfügbar")
	}
	safe := filepath.Base(header.Filename)
	if safe == "." || safe == "" {
		return 0, fmt.Errorf("ungültiger Dateiname")
	}
	tmp, err := os.CreateTemp(h.UploadDir, "upload-*")
	if err != nil {
		return 0, fmt.Errorf("Upload konnte nicht gespeichert werden")
	}
	size, copyErr := io.Copy(tmp, file)
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(tmp.Name())
		return 0, fmt.Errorf("Upload konnte nicht gespeichert werden")
	}
	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	var fileID int
	err = h.DB.QueryRowContext(r.Context(), `INSERT INTO files(name,path,size,mime_type,uploader_id,class_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, safe, tmp.Name(), size, mime, uploaderID, classID).Scan(&fileID)
	if err != nil {
		os.Remove(tmp.Name())
		return 0, fmt.Errorf("Datenbankfehler")
	}
	return fileID, nil
}
func (h *Server) DeleteApiV1SubmissionsId(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	var studentID int
	var status string
	var fileID sql.NullInt64
	err := h.DB.QueryRowContext(r.Context(), `SELECT student_id,status,file_id FROM homework_submissions WHERE id=$1`, id).Scan(&studentID, &status, &fileID)
	if notFound(w, err, "Abgabe") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if !isSuperadmin(c.Role) && (c.Role != "student" || c.UserID != studentID) {
		writeError(w, 403, "keine Berechtigung")
		return
	}
	if status == "graded" {
		writeError(w, 400, "bereits benotete Abgaben können nicht zurückgezogen werden")
		return
	}
	_, err = h.DB.ExecContext(r.Context(), `UPDATE homework_submissions SET status='open',submitted_at=NULL,file_id=NULL WHERE id=$1`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if fileID.Valid {
		h.deleteFileByID(r, int(fileID.Int64))
	}
	w.WriteHeader(204)
}
func (h *Server) PatchApiV1SubmissionsId(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if !requireRole(w, c, "teacher", "superadmin") {
		return
	}
	var classID int
	err := h.DB.QueryRowContext(r.Context(), `SELECT h.class_id FROM homework_submissions hs JOIN homework h ON h.id=hs.homework_id WHERE hs.id=$1`, id).Scan(&classID)
	if notFound(w, err, "Abgabe") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if !h.canManageClass(r, classID) {
		writeError(w, 403, "keine Verwaltungsrechte")
		return
	}
	var req api.GradeSubmissionRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Grade < 1 || req.Grade > 6 {
		writeError(w, 400, "Note muss zwischen 1 und 6 liegen")
		return
	}
	var v api.HomeworkSubmission
	var grade sql.NullFloat64
	var submitted, graded sql.NullTime
	var status string
	var fileID sql.NullInt64
	err = h.DB.QueryRowContext(r.Context(), `UPDATE homework_submissions SET grade=$1,status='graded',graded_at=NOW() WHERE id=$2 RETURNING id,homework_id,student_id,status,grade,submitted_at,graded_at,file_id`, req.Grade, id).Scan(&v.Id, &v.HomeworkId, &v.StudentId, &status, &grade, &submitted, &graded, &fileID)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	v.Status = api.HomeworkSubmissionStatus(status)
	if grade.Valid {
		x := float32(grade.Float64)
		v.Grade = &x
	}
	if submitted.Valid {
		x := submitted.Time
		v.SubmittedAt = &x
	}
	if graded.Valid {
		x := graded.Time
		v.GradedAt = &x
	}
	if fileID.Valid {
		x := int(fileID.Int64)
		v.FileId = &x
	}
	writeJSON(w, 200, v)
}
