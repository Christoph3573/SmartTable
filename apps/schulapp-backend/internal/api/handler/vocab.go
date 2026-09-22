package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"schulapp/internal/api"
	appmw "schulapp/internal/middleware"
)

// Vokabeln (Lern-Bereich): Sets + Karteikarten mit Leitner-Boxen.
// Sichtbarkeit: eigene Sets + Sets geteilter Klassen (Mitglied/Lehrer).
// Schreiben (Set/Karten anlegen, bearbeiten, löschen): nur der Owner.

func scanVocabSet(row interface{ Scan(...any) error }) (api.VocabSet, error) {
	var v api.VocabSet
	var classID sql.NullInt64
	var description sql.NullString
	var created time.Time
	err := row.Scan(&v.Id, &v.OwnerId, &classID, &v.Title, &description, &v.SourceLang, &v.TargetLang, &v.CardCount, &created)
	if err != nil {
		return v, err
	}
	if classID.Valid {
		x := int(classID.Int64)
		v.ClassId = &x
	}
	if description.Valid {
		x := description.String
		v.Description = &x
	}
	v.CreatedAt = &created
	return v, nil
}

func scanVocabCard(row interface{ Scan(...any) error }) (api.VocabCard, error) {
	var v api.VocabCard
	var hint sql.NullString
	var due time.Time
	err := row.Scan(&v.Id, &v.SetId, &v.Front, &v.Back, &hint, &v.Box, &due)
	if err != nil {
		return v, err
	}
	if hint.Valid {
		x := hint.String
		v.Hint = &x
	}
	v.DueAt = due
	return v, nil
}

const vocabSetSelect = `SELECT s.id, s.owner_id, s.class_id, s.title, s.description,
	s.source_lang, s.target_lang,
	(SELECT COUNT(*) FROM vocab_cards c WHERE c.set_id = s.id),
	s.created_at FROM vocab_sets s`

func vocabSetByID(r *http.Request, db *sql.DB, id int) (api.VocabSet, error) {
	row := db.QueryRowContext(r.Context(), vocabSetSelect+` WHERE s.id=$1`, id)
	return scanVocabSet(row)
}

// vocabSetVisible prüft, ob der Aufrufer das Set sehen darf: Owner oder
// Mitglied/Lehrer der geteilten Klasse (bzw. Superadmin überall).
func (h *Server) vocabSetVisible(r *http.Request, set api.VocabSet) bool {
	c := appmw.GetClaims(r)
	if c == nil {
		return false
	}
	if c.UserID == set.OwnerId || isSuperadmin(c.Role) {
		return true
	}
	if set.ClassId == nil {
		return false
	}
	return h.canReadClass(r, *set.ClassId)
}

func (h *Server) GetApiV1VocabSets(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), vocabSetSelect+`
		WHERE s.owner_id=$1
		   OR s.class_id IS NULL AND s.owner_id=$1
		   OR (s.class_id IS NOT NULL AND (
		         EXISTS(SELECT 1 FROM class_members cm WHERE cm.class_id=s.class_id AND cm.user_id=$1)
		      OR EXISTS(SELECT 1 FROM class_teachers ct WHERE ct.class_id=s.class_id AND ct.user_id=$1)))
		ORDER BY s.created_at DESC`, c.UserID)
	if isSuperadmin(c.Role) {
		rows, err = h.DB.QueryContext(r.Context(), vocabSetSelect+` ORDER BY s.created_at DESC`)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.VocabSet{}
	for rows.Next() {
		v, err := scanVocabSet(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Datenbankfehler")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Server) PostApiV1VocabSets(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	var req api.CreateVocabSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	if req.Title == "" || req.SourceLang == "" || req.TargetLang == "" {
		writeError(w, http.StatusBadRequest, "title, source_lang und target_lang sind Pflicht")
		return
	}
	var classID any
	if req.ClassId != nil {
		if _, ok := h.requireClassSchool(w, r, *req.ClassId); !ok {
			return
		}
		classID = *req.ClassId
	}
	var v api.VocabSet
	row := h.DB.QueryRowContext(r.Context(),
		`INSERT INTO vocab_sets (owner_id, class_id, title, description, source_lang, target_lang)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		c.UserID, classID, req.Title, nullableString(req.Description), req.SourceLang, req.TargetLang)
	var id int
	if err := row.Scan(&id); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	v, err := vocabSetByID(r, h.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (h *Server) vocabSetForWrite(w http.ResponseWriter, r *http.Request) (api.VocabSet, bool) {
	c := h.claims(w, r)
	if c == nil {
		return api.VocabSet{}, false
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ungültige ID")
		return api.VocabSet{}, false
	}
	v, err := vocabSetByID(r, h.DB, id)
	if notFound(w, err, "Vokabelset") {
		return api.VocabSet{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return api.VocabSet{}, false
	}
	if v.OwnerId != c.UserID && !isSuperadmin(c.Role) {
		writeError(w, http.StatusForbidden, "nur der Besitzer darf dieses Set bearbeiten")
		return api.VocabSet{}, false
	}
	return v, true
}

func (h *Server) PatchApiV1VocabSetsId(w http.ResponseWriter, r *http.Request, id int) {
	v, ok := h.vocabSetForWrite(w, r)
	if !ok {
		return
	}
	var req api.UpdateVocabSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	// class_id-Update nur wenn mitgeschickt; Titel/Beschreibung via COALESCE.
	if req.ClassId != nil {
		if _, ok := h.requireClassSchool(w, r, *req.ClassId); !ok {
			return
		}
		row := h.DB.QueryRowContext(r.Context(),
			`UPDATE vocab_sets SET title=COALESCE($1::text,title), description=COALESCE($2::text,description), class_id=$4 WHERE id=$3 RETURNING id`,
			nullableString(req.Title), nullableString(req.Description), v.Id, *req.ClassId)
		var newID int
		if err := row.Scan(&newID); err != nil {
			writeError(w, http.StatusInternalServerError, "Datenbankfehler")
			return
		}
		updated, err := vocabSetByID(r, h.DB, newID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Datenbankfehler")
			return
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}
	row := h.DB.QueryRowContext(r.Context(),
		`UPDATE vocab_sets SET title=COALESCE($1::text,title), description=COALESCE($2::text,description) WHERE id=$3 RETURNING id`,
		nullableString(req.Title), nullableString(req.Description), v.Id)
	var newID int
	if err := row.Scan(&newID); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	updated, err := vocabSetByID(r, h.DB, newID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Server) DeleteApiV1VocabSetsId(w http.ResponseWriter, r *http.Request, id int) {
	v, ok := h.vocabSetForWrite(w, r)
	if !ok {
		return
	}
	if _, err := h.DB.ExecContext(r.Context(), `DELETE FROM vocab_sets WHERE id=$1`, v.Id); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Server) GetApiV1VocabSetsIdCards(w http.ResponseWriter, r *http.Request, id int) {
	if h.claims(w, r) == nil {
		return
	}
	v, err := vocabSetByID(r, h.DB, id)
	if notFound(w, err, "Vokabelset") {
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	if !h.vocabSetVisible(r, v) {
		writeError(w, http.StatusForbidden, "kein Zugriff auf dieses Set")
		return
	}
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, set_id, front, back, hint, box, due_at FROM vocab_cards WHERE set_id=$1 ORDER BY id`, v.Id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.VocabCard{}
	for rows.Next() {
		card, err := scanVocabCard(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Datenbankfehler")
			return
		}
		out = append(out, card)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Server) PostApiV1VocabSetsIdCards(w http.ResponseWriter, r *http.Request, id int) {
	v, ok := h.vocabSetForWrite(w, r)
	if !ok {
		return
	}
	var req api.CreateVocabCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	if req.Front == "" || req.Back == "" {
		writeError(w, http.StatusBadRequest, "front und back sind Pflicht")
		return
	}
	var newID int
	err := h.DB.QueryRowContext(r.Context(),
		`INSERT INTO vocab_cards (set_id, front, back, hint) VALUES ($1,$2,$3,$4) RETURNING id`,
		v.Id, req.Front, req.Back, nullableString(req.Hint),
	).Scan(&newID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	row := h.DB.QueryRowContext(r.Context(),
		`SELECT id, set_id, front, back, hint, box, due_at FROM vocab_cards WHERE id=$1`, newID)
	card, err := scanVocabCard(row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	writeJSON(w, http.StatusCreated, card)
}

func (h *Server) vocabCardSet(w http.ResponseWriter, r *http.Request, cardID int) (api.VocabCard, api.VocabSet, bool) {
	if h.claims(w, r) == nil {
		return api.VocabCard{}, api.VocabSet{}, false
	}
	row := h.DB.QueryRowContext(r.Context(),
		`SELECT id, set_id, front, back, hint, box, due_at FROM vocab_cards WHERE id=$1`, cardID)
	card, err := scanVocabCard(row)
	if notFound(w, err, "Karte") {
		return api.VocabCard{}, api.VocabSet{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return api.VocabCard{}, api.VocabSet{}, false
	}
	set, err := vocabSetByID(r, h.DB, card.SetId)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return api.VocabCard{}, api.VocabSet{}, false
	}
	return card, set, true
}

func (h *Server) PatchApiV1VocabCardsId(w http.ResponseWriter, r *http.Request, id int) {
	card, set, ok := h.vocabCardSet(w, r, id)
	if !ok {
		return
	}
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if set.OwnerId != c.UserID && !isSuperadmin(c.Role) {
		writeError(w, http.StatusForbidden, "nur der Besitzer darf diese Karte bearbeiten")
		return
	}
	var req api.UpdateVocabCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	row := h.DB.QueryRowContext(r.Context(),
		`UPDATE vocab_cards SET front=COALESCE($1::text,front), back=COALESCE($2::text,back), hint=COALESCE($3::text,hint)
		 WHERE id=$4 RETURNING id`,
		nullableString(req.Front), nullableString(req.Back), nullableString(req.Hint), card.Id)
	var newID int
	if err := row.Scan(&newID); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	row = h.DB.QueryRowContext(r.Context(),
		`SELECT id, set_id, front, back, hint, box, due_at FROM vocab_cards WHERE id=$1`, newID)
	updated, err := scanVocabCard(row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Server) DeleteApiV1VocabCardsId(w http.ResponseWriter, r *http.Request, id int) {
	card, set, ok := h.vocabCardSet(w, r, id)
	if !ok {
		return
	}
	c := h.claims(w, r)
	if c == nil {
		return
	}
	if set.OwnerId != c.UserID && !isSuperadmin(c.Role) {
		writeError(w, http.StatusForbidden, "nur der Besitzer darf diese Karte löschen")
		return
	}
	if _, err := h.DB.ExecContext(r.Context(), `DELETE FROM vocab_cards WHERE id=$1`, card.Id); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PostApiV1VocabCardsIdGrade bewertet eine Karte nach Leitner:
// gewusst → Box+1 (max 5), nicht gewusst → Box 1. Fälligkeit:
// Box 1 sofort, 2 +1 Tag, 3 +3 Tage, 4 +7 Tage, 5 +14 Tage.
// Bewerten darf jeder, der das Set sehen darf (Lernfortschritt ist
// pro Karte global — bewusst simpel, kein User-Tracking).
func (h *Server) PostApiV1VocabCardsIdGrade(w http.ResponseWriter, r *http.Request, id int) {
	card, set, ok := h.vocabCardSet(w, r, id)
	if !ok {
		return
	}
	if !h.vocabSetVisible(r, set) {
		writeError(w, http.StatusForbidden, "kein Zugriff auf dieses Set")
		return
	}
	var req api.GradeVocabCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	box := 1
	if req.Known {
		box = card.Box + 1
		if box > 5 {
			box = 5
		}
	}
	due := time.Now()
	switch box {
	case 2:
		due = due.Add(24 * time.Hour)
	case 3:
		due = due.Add(3 * 24 * time.Hour)
	case 4:
		due = due.Add(7 * 24 * time.Hour)
	case 5:
		due = due.Add(14 * 24 * time.Hour)
	}
	row := h.DB.QueryRowContext(r.Context(),
		`UPDATE vocab_cards SET box=$1, due_at=$2 WHERE id=$3 RETURNING id`, box, due, card.Id)
	var newID int
	if err := row.Scan(&newID); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	row = h.DB.QueryRowContext(r.Context(),
		`SELECT id, set_id, front, back, hint, box, due_at FROM vocab_cards WHERE id=$1`, newID)
	updated, err := scanVocabCard(row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
