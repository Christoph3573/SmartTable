package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"schulapp/internal/api"
)

func scanLesson(row interface{ Scan(...any) error }) (api.Lesson, error) {
	var v api.Lesson
	var room sql.NullString
	var created time.Time
	err := row.Scan(&v.Id, &v.ClassId, &v.SubjectId, &v.TeacherId, &v.DayOfWeek, &v.Period, &room, &created)
	if err == nil {
		if room.Valid {
			x := room.String
			v.Room = &x
		}
		v.CreatedAt = &created
	}
	return v, err
}

// mondayOf returns the Monday (start) of the ISO week containing t.
func mondayOf(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	// time.Weekday: Sunday=0..Saturday=6; we want Monday=0..Sunday=6 offset.
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset)
}

// GetApiV1ClassesIdLessons returns the weekly timetable for a class. Readable
// by class members/teachers, school_admin and superadmin (same access rule
// as homework via requireClassRead). Lessons recur by day_of_week; this
// projects them onto the requested calendar week and overlays any
// substitution for the same class_id+date+period, so the frontend gets one
// flat list per week without having to reconcile recurring vs. absolute
// scheduling itself.
func (h *Server) GetApiV1ClassesIdLessons(w http.ResponseWriter, r *http.Request, id int, p api.GetApiV1ClassesIdLessonsParams) {
	if !h.requireClassRead(w, r, id) {
		return
	}
	week := time.Now()
	if p.WeekOf != nil {
		week = p.WeekOf.Time
	}
	monday := mondayOf(week)
	friday := monday.AddDate(0, 0, 4)

	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,class_id,subject_id,teacher_id,day_of_week,period,room,created_at FROM lessons WHERE class_id=$1 ORDER BY day_of_week,period`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	lessons := []api.Lesson{}
	for rows.Next() {
		v, err := scanLesson(rows)
		if err != nil {
			rows.Close()
			writeError(w, 500, "Datenbankfehler")
			return
		}
		lessons = append(lessons, v)
	}
	rows.Close()

	subRows, err := h.DB.QueryContext(r.Context(), `SELECT id,date,period,class_id,subject_id,original_teacher_id,sub_teacher_id,room,type,note,created_at FROM substitutions WHERE class_id=$1 AND date BETWEEN $2::date AND $3::date`, id, monday, friday)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	type key struct {
		date   string
		period int
	}
	subs := map[key]api.Substitution{}
	for subRows.Next() {
		v, err := scanSub(subRows)
		if err != nil {
			subRows.Close()
			writeError(w, 500, "Datenbankfehler")
			return
		}
		subs[key{v.Date.Format(openapi_types.DateFormat), v.Period}] = v
	}
	subRows.Close()

	out := make([]api.TimetableEntry, 0, len(lessons))
	for _, l := range lessons {
		date := monday.AddDate(0, 0, l.DayOfWeek-1)
		entry := api.TimetableEntry{
			Lesson: l,
			Date:   openapi_types.Date{Time: date},
			Type:   "regular",
		}
		if sub, ok := subs[key{date.Format(openapi_types.DateFormat), l.Period}]; ok {
			subCopy := sub
			entry.Substitution = &subCopy
			if sub.Type == "cancellation" {
				entry.Type = "cancelled"
			} else {
				entry.Type = "substituted"
			}
		}
		out = append(out, entry)
	}
	writeJSON(w, 200, out)
}

// PostApiV1ClassesIdLessons creates a recurring lesson slot. Restricted to
// teachers of the class, school_admin of the class' school and superadmin -
// the same rule canManageClass already enforces for homework/substitutions.
func (h *Server) PostApiV1ClassesIdLessons(w http.ResponseWriter, r *http.Request, id int) {
	if !h.requireClassManage(w, r, id) {
		return
	}
	var req api.CreateLessonRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.SubjectId < 1 || req.TeacherId < 1 || req.DayOfWeek < 1 || req.DayOfWeek > 5 || req.Period < 1 {
		writeError(w, 400, "ungültiger Stundenplan-Slot")
		return
	}
	v, err := scanLesson(h.DB.QueryRowContext(r.Context(),
		`INSERT INTO lessons(class_id,subject_id,teacher_id,day_of_week,period,room) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,class_id,subject_id,teacher_id,day_of_week,period,room,created_at`,
		id, req.SubjectId, req.TeacherId, req.DayOfWeek, req.Period, nullableString(req.Room)))
	if err != nil {
		if isDuplicate(err) {
			writeError(w, 409, "Klasse hat an diesem Tag/dieser Stunde bereits einen Stundenplan-Slot")
			return
		}
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 201, v)
}

func (h *Server) lessonClass(r *http.Request, id int) (int, error) {
	var classID int
	err := h.DB.QueryRowContext(r.Context(), `SELECT class_id FROM lessons WHERE id=$1`, id).Scan(&classID)
	return classID, err
}

func (h *Server) PatchApiV1LessonsId(w http.ResponseWriter, r *http.Request, id int) {
	classID, err := h.lessonClass(r, id)
	if notFound(w, err, "Stundenplan-Slot") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if !h.requireClassManage(w, r, classID) {
		return
	}
	var req api.UpdateLessonRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, 400, "ungültiges JSON")
		return
	}
	if req.DayOfWeek != nil && (*req.DayOfWeek < 1 || *req.DayOfWeek > 5) {
		writeError(w, 400, "day_of_week muss zwischen 1 und 5 liegen")
		return
	}
	v, err := scanLesson(h.DB.QueryRowContext(r.Context(),
		`UPDATE lessons SET
			subject_id  = COALESCE($1,subject_id),
			teacher_id  = COALESCE($2,teacher_id),
			day_of_week = COALESCE($3,day_of_week),
			period      = COALESCE($4,period),
			room        = COALESCE($5,room)
		WHERE id=$6 RETURNING id,class_id,subject_id,teacher_id,day_of_week,period,room,created_at`,
		nullableInt(req.SubjectId), nullableInt(req.TeacherId), nullableInt(req.DayOfWeek), nullableInt(req.Period), nullableString(req.Room), id))
	if notFound(w, err, "Stundenplan-Slot") {
		return
	}
	if err != nil {
		if isDuplicate(err) {
			writeError(w, 409, "Klasse hat an diesem Tag/dieser Stunde bereits einen Stundenplan-Slot")
			return
		}
		writeError(w, 500, "Datenbankfehler")
		return
	}
	writeJSON(w, 200, v)
}

func (h *Server) DeleteApiV1LessonsId(w http.ResponseWriter, r *http.Request, id int) {
	classID, err := h.lessonClass(r, id)
	if notFound(w, err, "Stundenplan-Slot") {
		return
	}
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	if !h.requireClassManage(w, r, classID) {
		return
	}
	res, err := h.DB.ExecContext(r.Context(), `DELETE FROM lessons WHERE id=$1`, id)
	if err != nil {
		writeError(w, 500, "Datenbankfehler")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, 404, "Stundenplan-Slot nicht gefunden")
		return
	}
	w.WriteHeader(204)
}
