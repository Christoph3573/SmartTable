package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	appmw "schulapp/internal/middleware"
)

// serveParam wie serve, setzt aber einen chi-Routenparameter {id}.
func serveParam(srv *Server, tok, method, path, id string, body any, fn http.HandlerFunc) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	appmw.Auth(srv.JWTSecret)(fn).ServeHTTP(rec, req)
	return rec
}

func TestCoursesCreateToggleIsolation(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)
	schoolID := mustSchool(t, "Kurs-Schule")
	userA := mustUser(t, "a@schule.de", "student", &schoolID)
	userB := mustUser(t, "b@schule.de", "student", &schoolID)
	tokenA := token(t, srv, userA, "a@schule.de", "student")
	tokenB := token(t, srv, userB, "b@schule.de", "student")

	// Eigenen Kurs mit zwei Kursstunden anlegen.
	rec := serve(srv, tokenA, http.MethodPost, "/api/v1/courses", map[string]any{
		"name":  "Französisch",
		"short": "F",
		"color": "#7c3aed",
		"lessons": []map[string]any{
			{"day_of_week": 1, "period": 3, "room": "A1"},
			{"day_of_week": 3, "period": 5},
		},
	}, srv.PostApiV1Courses)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created course
	mustDecode(t, rec, &created)
	if created.Id == 0 || len(created.Lessons) != 2 || created.Provider != "custom" {
		t.Fatalf("unerwarteter Kurs: %+v", created)
	}

	// Kurs-Auswahl (SmartTable-Fach) abwählen.
	selected := false
	rec = serve(srv, tokenA, http.MethodPut, "/api/v1/courses/selection", map[string]any{
		"provider":     "smarttable",
		"external_key": "subject:1",
		"name":         "Mathematik",
		"short":        "MA",
		"selected":     &selected,
	}, srv.PutApiV1CoursesSelection)
	if rec.Code != http.StatusOK {
		t.Fatalf("selection: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Liste: beide Einträge für A, keiner für B.
	rec = serve(srv, tokenA, http.MethodGet, "/api/v1/courses", nil, srv.GetApiV1Courses)
	var listA []course
	mustDecode(t, rec, &listA)
	if len(listA) != 2 {
		t.Fatalf("A-Kurse = %d, want 2: %+v", len(listA), listA)
	}
	rec = serve(srv, tokenB, http.MethodGet, "/api/v1/courses", nil, srv.GetApiV1Courses)
	var listB []course
	mustDecode(t, rec, &listB)
	if len(listB) != 0 {
		t.Fatalf("B sieht fremde Kurse: %+v", listB)
	}

	// Umschalten via PATCH (selected=true) auf den eigenen Kurs.
	yes := true
	rec = serveParam(srv, tokenA, http.MethodPatch, "/api/v1/courses/"+strconv.Itoa(created.Id), strconv.Itoa(created.Id),
		map[string]any{"selected": &yes}, srv.PatchApiV1CoursesId)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// B darf den Kurs von A nicht löschen.
	rec = serveParam(srv, tokenB, http.MethodDelete, "/api/v1/courses/"+strconv.Itoa(created.Id), strconv.Itoa(created.Id), nil, srv.DeleteApiV1CoursesId)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("fremdes Löschen: status=%d, want 404", rec.Code)
	}
}
