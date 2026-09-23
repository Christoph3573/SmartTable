package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	appmw "schulapp/internal/middleware"
	"schulapp/internal/ws"

	openapi_types "github.com/oapi-codegen/runtime/types"

	_ "github.com/lib/pq"
	"golang.org/x/time/rate"
)

// testDB is a shared connection to a local Postgres instance used for all
// handler tests. There is no dedicated test-DB harness in this repo yet, so
// each test truncates the tables it cares about via resetDB before seeding
// its own fixtures - simple and fast enough for this suite's size, and it
// exercises the real SQL the handlers issue (not a mock).
var testDB *sql.DB

func TestMain(m *testing.M) {
	// Deliberately NOT falling back to DATABASE_URL: these tests TRUNCATE
	// every app table between runs (see resetDB), so pointing them at a dev
	// or prod database would destroy real data. Use a dedicated database
	// (e.g. "schulapp_test") migrated the same way as the dev DB.
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://schulapp:schulapp@127.0.0.1:5434/schulapp_test?sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}
	if err := db.Ping(); err != nil {
		panic("handler tests need a reachable Postgres (set TEST_DATABASE_URL): " + err.Error())
	}
	testDB = db
	code := m.Run()
	db.Close()
	os.Exit(code)
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return &Server{
		DB:                   testDB,
		JWTSecret:            []byte("test-secret"),
		LoginLimiter:         NewIPRateLimiter(rate.Every(0), 1000),
		RegisterLimiter:      NewIPRateLimiter(rate.Every(0), 1000),
		UploadDir:            t.TempDir(),
		Hub:                  ws.NewHub(),
		SchoolConnectBaseURL: "http://127.0.0.1:1",
	}
}

// resetDB truncates every app table so each test starts from a clean slate,
// regardless of what earlier tests left behind.
func resetDB(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(`TRUNCATE TABLE
		messages, chat_members, chat_channels,
		homework_submissions, homework,
		substitutions, events,
		files, file_folders,
		vocab_cards, vocab_sets,
		course_lessons, courses,
		opencode_messages, opencode_session_tokens, opencode_sessions,
		class_join_requests, class_teachers, class_members,
		refresh_tokens, classes, subjects, users, schools
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("resetDB: %v", err)
	}
}

func mustSchool(t *testing.T, name string) int {
	t.Helper()
	var id int
	if err := testDB.QueryRow(`INSERT INTO schools(name) VALUES($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("create school: %v", err)
	}
	return id
}

func mustClass(t *testing.T, schoolID int, name string) int {
	t.Helper()
	var id int
	if err := testDB.QueryRow(`INSERT INTO classes(name,school_year,school_id) VALUES($1,'2025/26',$2) RETURNING id`, name, schoolID).Scan(&id); err != nil {
		t.Fatalf("create class: %v", err)
	}
	return id
}

// mustUser inserts a user and returns its id. schoolID may be nil (superadmin).
func mustUser(t *testing.T, email, role string, schoolID *int) int {
	t.Helper()
	var id int
	err := testDB.QueryRow(
		`INSERT INTO users(email,password_hash,first_name,last_name,role,school_id) VALUES($1,'x','First','Last',$2,$3) RETURNING id`,
		email, role, schoolID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return id
}

func addMember(t *testing.T, classID, userID int) {
	t.Helper()
	if _, err := testDB.Exec(`INSERT INTO class_members(class_id,user_id) VALUES($1,$2)`, classID, userID); err != nil {
		t.Fatalf("add member: %v", err)
	}
}

func addTeacher(t *testing.T, classID, userID int) {
	t.Helper()
	if _, err := testDB.Exec(`INSERT INTO class_teachers(class_id,user_id) VALUES($1,$2)`, classID, userID); err != nil {
		t.Fatalf("add teacher: %v", err)
	}
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

func mustSubject(t *testing.T) int {
	t.Helper()
	var id int
	if err := testDB.QueryRow(`INSERT INTO subjects(name,short) VALUES('Mathematik','MA') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("create subject: %v", err)
	}
	return id
}

func today() openapi_types.Date {
	return openapi_types.Date{Time: time.Now().Truncate(24 * time.Hour)}
}

func mustDecode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
}

func token(t *testing.T, srv *Server, userID int, email, role string) string {
	t.Helper()
	tok, err := srv.generateAccessToken(userID, email, role)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

// serve runs fn through the real Auth middleware so handlers see claims the
// same way they do in production, then returns the recorded response.
func serve(srv *Server, tok, method, path string, body any, fn http.HandlerFunc) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	appmw.Auth(srv.JWTSecret)(fn).ServeHTTP(rec, req)
	return rec
}
