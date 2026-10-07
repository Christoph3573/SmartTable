package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Kurse (Schule): Der Nutzer hakt ab, welche Kurse er belegt. Abgewählte
// Kurse (selected=false) werden im Stunden-/Vertretungsplan ausgeblendet.
// Eigene Kurse (provider=custom) bringen zusätzliche Kursstunden mit.
//
// Die verfügbaren Kurse werden clientseitig aus der jeweiligen Datenquelle
// abgeleitet (SmartTable-Fächer bzw. SchoolConnect-Kurskürzel) und hier als
// Auswahl-Zeilen gespeichert. Fehlt eine Zeile, gilt der Kurs als sichtbar.

type courseLesson struct {
	DayOfWeek int    `json:"day_of_week"`
	Period    int    `json:"period"`
	Room      string `json:"room,omitempty"`
}

type course struct {
	Id          int            `json:"id"`
	Provider    string         `json:"provider"`
	ExternalKey string         `json:"external_key"`
	Name        string         `json:"name"`
	Short       string         `json:"short,omitempty"`
	Color       string         `json:"color,omitempty"`
	Selected    bool           `json:"selected"`
	Lessons     []courseLesson `json:"lessons"`
}

type createCourseRequest struct {
	Provider    string         `json:"provider"`
	ExternalKey string         `json:"external_key"`
	Name        string         `json:"name"`
	Short       string         `json:"short"`
	Color       string         `json:"color"`
	Selected    *bool          `json:"selected"`
	Lessons     []courseLesson `json:"lessons"`
}

type updateCourseRequest struct {
	Name     *string         `json:"name"`
	Short    *string         `json:"short"`
	Color    *string         `json:"color"`
	Selected *bool           `json:"selected"`
	Lessons  *[]courseLesson `json:"lessons"`
}

func validCourseProvider(p string) bool {
	return p == "smarttable" || p == "schoolconnect" || p == "custom"
}

// courseLessons lädt die eigenen Kursstunden eines Kurses.
func (h *Server) courseLessons(r *http.Request, courseID int) []courseLesson {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT day_of_week, period, room FROM course_lessons WHERE course_id=$1 ORDER BY day_of_week, period`, courseID)
	if err != nil {
		return []courseLesson{}
	}
	defer rows.Close()
	out := []courseLesson{}
	for rows.Next() {
		var l courseLesson
		var room sql.NullString
		if err := rows.Scan(&l.DayOfWeek, &l.Period, &room); err != nil {
			continue
		}
		if room.Valid {
			l.Room = room.String
		}
		out = append(out, l)
	}
	return out
}

// scanCourse liest eine courses-Zeile inkl. eigener Kursstunden.
func (h *Server) scanCourse(r *http.Request, row interface{ Scan(...any) error }) (course, error) {
	var c course
	var short, color, key sql.NullString
	var created sql.NullTime
	if err := row.Scan(&c.Id, &c.Provider, &key, &c.Name, &short, &color, &c.Selected, &created); err != nil {
		return c, err
	}
	if key.Valid {
		c.ExternalKey = key.String
	}
	if short.Valid {
		c.Short = short.String
	}
	if color.Valid {
		c.Color = color.String
	}
	c.Lessons = h.courseLessons(r, c.Id)
	return c, nil
}

const courseSelect = `SELECT id, provider, external_key, name, short, color, selected, created_at FROM courses`

// GetApiV1Courses listet die gespeicherten Kurs-Auswahlen des Benutzers.
func (h *Server) GetApiV1Courses(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), courseSelect+` WHERE owner_id=$1 ORDER BY provider, name`, c.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []course{}
	for rows.Next() {
		item, err := h.scanCourse(r, rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Datenbankfehler")
			return
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

// PostApiV1Courses legt einen eigenen Kurs (provider=custom) inkl. Kursstunden an.
func (h *Server) PostApiV1Courses(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	var req createCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name darf nicht leer sein")
		return
	}
	provider := req.Provider
	if provider == "" {
		provider = "custom"
	}
	if !validCourseProvider(provider) {
		writeError(w, http.StatusBadRequest, "ungültiger provider")
		return
	}
	selected := true
	if req.Selected != nil {
		selected = *req.Selected
	}
	// Eigene Kurse haben keine externe Kennung: NULL statt '' (sonst
	// kollidieren mehrere eigene Kurse am UNIQUE-Index).
	var externalKey any
	if key := strings.TrimSpace(req.ExternalKey); key != "" {
		externalKey = key
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	defer tx.Rollback()
	var id int
	if err := tx.QueryRowContext(r.Context(),
		`INSERT INTO courses (owner_id, provider, external_key, name, short, color, selected)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		c.UserID, provider, externalKey, strings.TrimSpace(req.Name),
		nullableString(&req.Short), nullableString(&req.Color), selected,
	).Scan(&id); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	if err := h.replaceCourseLessonsTx(r, tx, id, req.Lessons); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	row := h.DB.QueryRowContext(r.Context(), courseSelect+` WHERE id=$1`, id)
	item, err := h.scanCourse(r, row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// PutApiV1CoursesSelection speichert/aktualisiert eine Kurs-Auswahl
// (z. B. eine Checkbox im Kurse-Reiter). Upsert über (owner, provider, key).
func (h *Server) PutApiV1CoursesSelection(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	var req createCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	if !validCourseProvider(req.Provider) || req.Provider == "custom" {
		writeError(w, http.StatusBadRequest, "provider muss smarttable oder schoolconnect sein")
		return
	}
	key := strings.TrimSpace(req.ExternalKey)
	if key == "" || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "external_key und name sind erforderlich")
		return
	}
	selected := true
	if req.Selected != nil {
		selected = *req.Selected
	}
	row := h.DB.QueryRowContext(r.Context(),
		`INSERT INTO courses (owner_id, provider, external_key, name, short, color, selected)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (owner_id, provider, external_key) DO UPDATE SET
		   name=EXCLUDED.name, short=EXCLUDED.short, color=EXCLUDED.color, selected=EXCLUDED.selected
		 RETURNING id, provider, external_key, name, short, color, selected, created_at`,
		c.UserID, req.Provider, key, strings.TrimSpace(req.Name),
		nullableString(&req.Short), nullableString(&req.Color), selected)
	item, err := h.scanCourse(r, row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// PatchApiV1CoursesId aktualisiert einen Kurs (Name/Kürzel/Farbe/Auswahl und
// optional die Kursstunden eines eigenen Kurses).
func (h *Server) PatchApiV1CoursesId(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ungültige id")
		return
	}
	var req updateCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	var ownerID int
	err = h.DB.QueryRowContext(r.Context(), `SELECT owner_id FROM courses WHERE id=$1`, id).Scan(&ownerID)
	if notFound(w, err, "Kurs") {
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	if ownerID != c.UserID && !isSuperadmin(c.Role) {
		writeError(w, http.StatusForbidden, "kein Zugriff auf diesen Kurs")
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(),
		`UPDATE courses SET
		   name     = COALESCE($1, name),
		   short    = COALESCE($2, short),
		   color    = COALESCE($3, color),
		   selected = COALESCE($4, selected)
		 WHERE id=$5`,
		nullableString(req.Name), nullableString(req.Short), nullableString(req.Color), req.Selected, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	if req.Lessons != nil {
		if err := h.replaceCourseLessonsTx(r, tx, id, *req.Lessons); err != nil {
			writeError(w, http.StatusInternalServerError, "Datenbankfehler")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	row := h.DB.QueryRowContext(r.Context(), courseSelect+` WHERE id=$1`, id)
	item, err := h.scanCourse(r, row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// DeleteApiV1CoursesId löscht einen Kurs (samt Kursstunden).
func (h *Server) DeleteApiV1CoursesId(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ungültige id")
		return
	}
	res, err := h.DB.ExecContext(r.Context(),
		`DELETE FROM courses WHERE id=$1 AND (owner_id=$2 OR $3)`, id, c.UserID, isSuperadmin(c.Role))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "Kurs nicht gefunden")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// replaceCourseLessonsTx ersetzt die Kursstunden eines Kurses (Duplikate
// Tag/Stunde werden verworfen, damit der UNIQUE-Index nicht greift).
func (h *Server) replaceCourseLessonsTx(r *http.Request, tx *sql.Tx, courseID int, lessons []courseLesson) error {
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM course_lessons WHERE course_id=$1`, courseID); err != nil {
		return err
	}
	seen := map[int]bool{}
	for _, l := range lessons {
		if l.DayOfWeek < 1 || l.DayOfWeek > 5 || l.Period < 1 {
			continue
		}
		slot := l.DayOfWeek*100 + l.Period
		if seen[slot] {
			continue
		}
		seen[slot] = true
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO course_lessons (course_id, day_of_week, period, room) VALUES ($1,$2,$3,$4)`,
			courseID, l.DayOfWeek, l.Period, nullableString(&l.Room)); err != nil {
			return err
		}
	}
	return nil
}

// parseIDParam liest {id} aus der Route.
func parseIDParam(r *http.Request) (int, error) {
	return strconv.Atoi(chi.URLParam(r, "id"))
}
