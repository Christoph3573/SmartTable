package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appmw "schulapp/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
)

func mustToken(t *testing.T, userID int, role string) string {
	t.Helper()
	claims := appmw.Claims{
		UserID: userID,
		Email:  "test@schule.de",
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func mcpPost(t *testing.T, srv *Server, target, body, token, sessionToken string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if sessionToken != "" {
		req.Header.Set("X-Session-Token", sessionToken)
	}
	rec := httptest.NewRecorder()
	srv.HandleMCP(rec, req)
	return rec
}

// TestMCPToolsListPublic prüft tools/list ohne Auth (Tool-Verzeichnis ist
// öffentliche Metainfo; der Tenant wird erst bei tools/call aufgelöst).
func TestMCPToolsListPublic(t *testing.T) {
	srv := newTestServer(t)
	rec := mcpPost(t, srv, "/api/v1/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var res struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := map[string]bool{
		"get_schedule": true, "get_substitutions": true, "get_homework": true,
		"get_learning_plan": true, "get_vocabularies": true,
	}
	for _, tool := range res.Result.Tools {
		delete(want, tool.Name)
	}
	if len(want) != 0 {
		t.Fatalf("fehlende Tools: %v", want)
	}
}

// TestMCPCallTenantIsolation: Zwei User, zwei Klassen — User A darf über
// tools/call weder Klasse noch Vokabeln von User B sehen. Der Tenant kommt
// aus JWT/Session-Token, nie aus Tool-Argumenten.
func TestMCPCallTenantIsolation(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)
	schoolID := mustSchool(t, "MCP-Schule")
	classA := mustClass(t, schoolID, "9a")
	classB := mustClass(t, schoolID, "9b")
	userA := mustUser(t, "a@schule.de", "student", &schoolID)
	userB := mustUser(t, "b@schule.de", "student", &schoolID)
	if _, err := testDB.Exec(`INSERT INTO class_members(class_id,user_id) VALUES($1,$2)`, classA, userA); err != nil {
		t.Fatal(err)
	}
	if _, err := testDB.Exec(`INSERT INTO class_members(class_id,user_id) VALUES($1,$2)`, classB, userB); err != nil {
		t.Fatal(err)
	}
	var setB int
	if err := testDB.QueryRow(`INSERT INTO vocab_sets(owner_id,title,source_lang,target_lang) VALUES($1,'B-Set','de','en') RETURNING id`, userB).Scan(&setB); err != nil {
		t.Fatal(err)
	}
	if _, err := testDB.Exec(`INSERT INTO vocab_cards(set_id,front,back) VALUES($1,'Apfel','apple')`, setB); err != nil {
		t.Fatal(err)
	}
	tokenA := mustToken(t, userA, "student")

	call := func(body string) map[string]any {
		t.Helper()
		rec := mcpPost(t, srv, "/api/v1/mcp", body, tokenA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
		}
		var res map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return res
	}

	// 1. Fremde class_id wird abgewiesen (get_homework).
	res := call(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_homework","arguments":{"class_id":` + itoa(classB) + `}}}`)
	text := mcpResultText(t, res)
	if !strings.Contains(text, "Kein Zugriff") {
		t.Fatalf("fremde Klasse nicht abgewiesen: %q", text)
	}

	// 2. Fremde Vokabeln tauchen nicht auf (get_vocabularies).
	res = call(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_vocabularies","arguments":{"only_due":false}}}`)
	text = mcpResultText(t, res)
	if strings.Contains(text, "Apfel") || strings.Contains(text, "B-Set") {
		t.Fatalf("fremde Vokabeln geleakt: %q", text)
	}

	// 3. Ohne Token → 401 (weder JWT noch X-Session-Token).
	rec := mcpPost(t, srv, "/api/v1/mcp", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_homework","arguments":{}}}`, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Token: status = %d, want 401", rec.Code)
	}

	// 4. Session-Token-Pfad: Token für User A anlegen → gleiche Sicht wie JWT.
	sessionToken := "test-session-token-aaa"
	if _, err := testDB.Exec(`INSERT INTO opencode_session_tokens(token,user_id,expires_at) VALUES($1,$2,NOW()+INTERVAL '1 hour')`, sessionToken, userA); err != nil {
		t.Fatal(err)
	}
	rec2 := mcpPost(t, srv, "/api/v1/mcp", `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_vocabularies","arguments":{"only_due":false}}}`, "", sessionToken)
	if rec2.Code != http.StatusOK {
		t.Fatalf("session-token: status = %d (%s)", rec2.Code, rec2.Body.String())
	}
	var res2 map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &res2); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(mcpResultText(t, res2), "B-Set") {
		t.Fatal("Session-Token leak: fremde Vokabeln sichtbar")
	}

	// 5. Abgelaufenes Session-Token → 401.
	if _, err := testDB.Exec(`UPDATE opencode_session_tokens SET expires_at=NOW()-INTERVAL '1 hour' WHERE token=$1`, sessionToken); err != nil {
		t.Fatal(err)
	}
	rec3 := mcpPost(t, srv, "/api/v1/mcp", `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_homework","arguments":{}}}`, "", sessionToken)
	if rec3.Code != http.StatusUnauthorized {
		t.Fatalf("abgelaufenes Token: status = %d, want 401", rec3.Code)
	}

	// 6. SSE-Legacy: GET-Stream mit JWT liefert endpoint-Event; POST an die
	// Endpoint-URI mit sessionId beantwortet tools/list.
	streamReq := httptest.NewRequest(http.MethodGet, "/api/v1/mcp", nil)
	streamReq.Header.Set("Authorization", "Bearer "+tokenA)
	streamRec := httptest.NewRecorder()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		srv.HandleMCP(streamRec, streamReq)
	}()
	deadline := time.Now().Add(3 * time.Second)
	var body string
	for {
		body = streamRec.Body.String()
		if strings.Contains(body, "event: endpoint") || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(body, "event: endpoint") || !strings.Contains(body, "sessionId=") {
		t.Fatalf("kein endpoint-Event im SSE-Stream: %q", body)
	}
	start := strings.Index(body, "sessionId=")
	sessionID := body[start+len("sessionId="):]
	if i := strings.Index(sessionID, "\n"); i >= 0 {
		sessionID = sessionID[:i]
	}
	sessionID = strings.TrimSpace(sessionID)
	listRec := mcpPost(t, srv, "/api/v1/mcp/messages?sessionId="+sessionID,
		`{"jsonrpc":"2.0","id":9,"method":"tools/list"}`, "", "")
	if listRec.Code != http.StatusOK {
		t.Fatalf("SSE tools/list: status = %d", listRec.Code)
	}
}

func mcpResultText(t *testing.T, res map[string]any) string {
	t.Helper()
	result, _ := res["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("kein content in %v", res)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	return text
}
