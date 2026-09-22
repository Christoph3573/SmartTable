// Package handler — OpenCode-Integration (KI-Lernchat).
//
// Das Go-Backend ist die Multi-Tenant-/Security-Grenze, OpenCode nur die
// Agent-Runtime: Der Browser spricht ausschließlich mit dem Backend
// (POST/GET/DELETE /api/v1/integrations/opencode/sessions...), niemals
// direkt mit `opencode serve`.
//
// Ein gemeinsamer OpenCode-Service (Compose-Service "opencode", nur internes
// Netz, OPENCODE_BASE_URL, Default http://opencode:8082) bedient alle User.
// Jeder User bekommt seinen eigenen Workspace unterhalb von
// OPENCODE_WORKSPACE_ROOT (Default /workspaces/<tenant-id>, aus der
// JWT-user_id — nie aus Client-Parametern). Sessions stehen in der Tabelle
// opencode_sessions mit user_id; jeder Zugriff prüft Ownership (Owner oder
// Superadmin).
//
// Schuldaten bekommt OpenCode ausschließlich über den Backend-MCP-Server
// (POST /api/v1/mcp, Tools get_schedule/get_substitutions/get_homework/
// get_learning_plan/get_vocabularies — siehe mcp.go): Der MCP-Server läuft im
// gleichen Backend-Prozess, der Tenant kommt dort pro in das Session-Token
// eingebettetem JWT-User (X-Session-Token), niemals client-seitig. Direkt auf
// SchoolConnect-Credentials anderer User kann OpenCode nicht zugreifen —
// SchoolConnect bleibt für Credentials + externe Schuldaten zuständig, das
// Backend setzt dort immer X-SC-Tenant aus der JWT-user_id.
//
// Ablauf Nachricht: Backend erzeugt pro Session (einmalig) eine
// OpenCode-Session (POST {base}/session?directory=<workspace>, Tools bis auf
// question deaktiviert, permission deny-all) und registriert den MCP-Server
// (POST {base}/mcp?directory=<workspace>, type remote, Backend-URL mit
// X-Session-Token). Der User-Prompt geht an POST
// {base}/session/{id}/message (Antwort kommt vollständig — kein
// Token-Streaming, Transfer chunked), Text-Parts werden aus der Antwort
// extrahiert, als Verlauf in opencode_messages gespeichert und zusätzlich als
// WebSocket-Event (type "opencode") an den User gepusht.
package handler

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"schulapp/internal/api"
)

const openCodeDefaultBaseURL = "http://opencode:8082"

// openCodeDefaultModel ist das Default-Modell für neue Lernchat-Sessions
// (OpenRouter-ID im Format provider/modell). Env OPENCODE_MODEL schlägt
// den Default; die Session-Antwort von POST /session enthält provider+model
// und wird in opencode_sessions gespeichert.
const openCodeDefaultModel = "openrouter/deepseek/deepseek-v4.1-flash"

const openCodeHint = "OpenCode-Service prüfen: läuft als Compose-Service " +
	"`opencode` (intern http://opencode:8082, kein Host-Port, `opencode serve " +
	"--hostname 0.0.0.0 --port 8082`) — lokal via `opencode serve` und " +
	"OPENCODE_BASE_URL=http://127.0.0.1:8082, Workspace-Root via " +
	"OPENCODE_WORKSPACE_ROOT (Default /workspaces, lokal z. B. ./data/workspaces)"

// openCodeHintCopy ist eine adressierbare Kopie für optionale Pointer-Felder.
var openCodeHintCopy = openCodeHint

// openCodeSystemPrompt legt Identität, Zuständigkeit und Ausgabeformat der
// Lern-KI fest: freundlicher Nachhilfelehrer, Schuldaten ausschließlich über
// die SmartTable-MCP-Tools (nie raten), kurze Markdown-Antworten auf Deutsch.
const openCodeSystemPrompt = "Du bist die Lern-KI von SmartTable — ein freundlicher, geduldiger " +
	"Nachhilfelehrer für Schülerinnen und Schüler. Du hilfst bei Hausaufgaben, erklärst Themen " +
	"Schritt für Schritt und motivierst. Antworte immer auf Deutsch, kurz und klar.\n\n" +
	"Dir stehen ausschließlich diese SmartTable-Tools zur Verfügung — alle persönlichen " +
	"Schuldaten kommen über sie (gespeist aus SchoolConnect); du hast keinen Shell-, Datei- " +
	"oder Internetzugriff:\n" +
	"- get_schedule: Stundenplan der Woche (Fächer, Tag, Stunde, Raum, Vertretung/Ausfall). " +
	"Nutze es z. B. bei „Welche Fächer habe ich morgen?“, „Was habe ich am Dienstag?“ oder " +
	"„Wie viele Stunden habe ich?“.\n" +
	"- get_substitutions: Vertretungen, Ausfälle und Raumwechsel im Zeitraum.\n" +
	"- get_homework: Hausaufgaben der eigenen Klassen mit Fälligkeitsdatum.\n" +
	"- get_learning_plan: LehrplanPLUS (Bayern) durchsuchen (schulart, fach, jahrgangsstufe, query).\n" +
	"- get_vocabularies: eigene Vokabelsets und fällige Karten.\n" +
	"- question: genau eine Rückfrage an die Nutzerin/den Nutzer stellen.\n\n" +
	"Wichtig: Beantworte Fragen zum persönlichen Lernstand, Stundenplan, Vertretungen, " +
	"Hausaufgaben oder Vokabeln NIE aus dem Gedächtnis und rate nicht — rufe zuerst das passende " +
	"Tool auf und beziehe dich konkret auf die zurückgegebenen Daten. Fragt jemand z. B. nach den " +
	"Fächern von morgen, nutze get_schedule und nenne die konkreten Fächer. Meldet ein Tool, dass " +
	"keine Klasse zugeordnet ist, erkläre freundlich, dass die Zuordnung über die Lehrkraft oder " +
	"Schulverwaltung erfolgt, und hilf bei anderen Fragen trotzdem weiter.\n\n" +
	"Format: Nutze Markdown (kurze Absätze, **fett** für wichtige Begriffe, Aufzählungen mit „-“ " +
	"wo sinnvoll, `Code` für Fachbegriffe). Keine Hausaufgaben-Lösungen zum Abschreiben — " +
	"erkläre den Lösungsweg Schritt für Schritt."

// openCodeDenyAll sperrt alle ausführenden Tools in der OpenCode-Session;
// question (Rückfragen an den User) bleibt erlaubt.
var openCodeDenyAll = []map[string]string{
	{"permission": "bash", "pattern": "*", "action": "deny"},
	{"permission": "read", "pattern": "*", "action": "deny"},
	{"permission": "glob", "pattern": "*", "action": "deny"},
	{"permission": "grep", "pattern": "*", "action": "deny"},
	{"permission": "edit", "pattern": "*", "action": "deny"},
	{"permission": "write", "pattern": "*", "action": "deny"},
	{"permission": "task", "pattern": "*", "action": "deny"},
	{"permission": "webfetch", "pattern": "*", "action": "deny"},
	{"permission": "websearch", "pattern": "*", "action": "deny"},
	{"permission": "skill", "pattern": "*", "action": "deny"},
	{"permission": "todowrite", "pattern": "*", "action": "deny"},
}

// openCodeBase liefert die konfigurierte OpenCode-Adresse (ohne trailing
// slash). Env schlägt Server-Feld, Default ist http://opencode:8082.
func (h *Server) openCodeBase() string {
	base := h.OpenCodeBaseURL
	if env := strings.TrimSpace(os.Getenv("OPENCODE_BASE_URL")); env != "" {
		base = env
	}
	base = strings.TrimSpace(base)
	if base == "" {
		base = openCodeDefaultBaseURL
	}
	return strings.TrimRight(base, "/")
}

// openCodeModel liefert das Default-Modell (ohne Leerzeichen). Env
// OPENCODE_MODEL schlägt Server-Feld; Default ist
// openrouter/deepseek/deepseek-v4.1-flash. Format provider/modell (z. B.
// openrouter/deepseek/deepseek-v4.1-flash) — das Backend splittet beim Senden an
// POST /session/{id}/message in {providerID, modelID}.
func (h *Server) openCodeModel() string {
	model := h.OpenCodeModel
	if env := strings.TrimSpace(os.Getenv("OPENCODE_MODEL")); env != "" {
		model = env
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = openCodeDefaultModel
	}
	return model
}

// openCodeModelPayload splittet "provider/modell" in das {providerID,
// modelID}-Objekt für POST /session/{id}/message. Ohne "/" kommt nil
// zurück (OpenCode nutzt dann sein eigenes Default aus opencode.json).
func openCodeModelPayload(model string) map[string]string {
	parts := strings.SplitN(strings.TrimSpace(model), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil
	}
	return map[string]string{"providerID": parts[0], "modelID": parts[1]}
}

// openCodeWorkspaceRoot liefert das Root-Verzeichnis für Tenant-Workspaces
// (der opencode-Container mountet dasselbe Volume). Env
// OPENCODE_WORKSPACE_ROOT schlägt Server-Feld; Default /workspaces.
func (h *Server) openCodeWorkspaceRoot() string {
	root := h.OpenCodeWorkspaceRoot
	if env := strings.TrimSpace(os.Getenv("OPENCODE_WORKSPACE_ROOT")); env != "" {
		root = env
	}
	root = strings.TrimSpace(root)
	if root == "" {
		root = "/workspaces"
	}
	return strings.TrimRight(root, "/")
}

// openCodeWorkspaceDir liefert /workspaces/<tenant-id> aus der JWT-user_id
// (pro App-Benutzer isoliert — nie aus Client-Parametern) und legt das
// Verzeichnis lokal an, falls das Backend denselben Mount sieht (schadet
// sonst nicht: Fehler wird ignoriert, opencode nutzt directory als cwd).
func (h *Server) openCodeWorkspaceDir(userID int) string {
	dir := h.openCodeWorkspaceRoot() + "/" + strconv.Itoa(userID)
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func openCodeClient() *http.Client {
	return &http.Client{Timeout: 120 * time.Second}
}

// openCodeDo schickt JSON an OpenCode und gibt Statuscode + Body (max 8 MB)
// zurück.
func openCodeDo(client *http.Client, method, rawURL string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, rawURL, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, raw, nil
}

func openCodeUnreachable(w http.ResponseWriter, base string) {
	writeJSON(w, http.StatusBadGateway, map[string]string{
		"error":   "OpenCode ist nicht erreichbar",
		"hint":    openCodeHint,
		"baseUrl": base,
	})
}

type openCodeSessionRow struct {
	api.OpenCodeSession
	OpencodeID string
}

func scanOpenCodeSession(row interface{ Scan(...any) error }) (openCodeSessionRow, error) {
	var s openCodeSessionRow
	var provider, model sql.NullString
	var created time.Time
	err := row.Scan(&s.Id, &s.OpencodeID, &s.Title, &s.Workspace, &provider, &model, &s.MessageCount, &created)
	if err != nil {
		return s, err
	}
	if provider.Valid {
		x := provider.String
		s.ModelProvider = &x
	}
	if model.Valid {
		x := model.String
		s.ModelId = &x
	}
	s.CreatedAt = created
	return s, nil
}

const openCodeSessionSelect = `SELECT s.id, s.opencode_id, s.title, s.workspace,
	s.model_provider, s.model_id,
	(SELECT COUNT(*) FROM opencode_messages m WHERE m.session_id = s.id),
	s.created_at FROM opencode_sessions s`

// openCodeSessionOwned lädt eine Session nur, wenn der Aufrufer Owner oder
// Superadmin ist — sonst 404 (kein Leak fremder IDs) bzw. 403.
func (h *Server) openCodeSessionOwned(w http.ResponseWriter, r *http.Request, id int) (openCodeSessionRow, bool) {
	c := h.claims(w, r)
	if c == nil {
		return openCodeSessionRow{}, false
	}
	row := h.DB.QueryRowContext(r.Context(), openCodeSessionSelect+` WHERE s.id=$1`, id)
	s, err := scanOpenCodeSession(row)
	if notFound(w, err, "Session") {
		return openCodeSessionRow{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return openCodeSessionRow{}, false
	}
	var ownerID int
	if err := h.DB.QueryRowContext(r.Context(), `SELECT owner_id FROM opencode_sessions WHERE id=$1`, id).Scan(&ownerID); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return openCodeSessionRow{}, false
	}
	if ownerID != c.UserID && !isSuperadmin(c.Role) {
		writeError(w, http.StatusForbidden, "kein Zugriff auf diese Session")
		return openCodeSessionRow{}, false
	}
	return s, true
}

// GetApiV1IntegrationsOpencodeStatus meldet Erreichbarkeit des Sidecars.
// Immer HTTP 200 mit reachable-Flag, analog zum SchoolConnect-Status.
func (h *Server) GetApiV1IntegrationsOpencodeStatus(w http.ResponseWriter, r *http.Request) {
	if h.claims(w, r) == nil {
		return
	}
	base := h.openCodeBase()
	status, _, err := openCodeDo(openCodeClient(), http.MethodGet, base+"/global/health", nil)
	if err != nil || status != http.StatusOK {
		writeJSON(w, http.StatusOK, api.OpenCodeStatus{
			Configured: true,
			Reachable:  false,
			BaseUrl:    &base,
			Hint:       &openCodeHintCopy,
		})
		return
	}
	writeJSON(w, http.StatusOK, api.OpenCodeStatus{
		Configured: true,
		Reachable:  true,
		BaseUrl:    &base,
	})
}

// GetApiV1IntegrationsOpencodeSessions listet die eigenen Sessions
// (Superadmin sieht alle).
func (h *Server) GetApiV1IntegrationsOpencodeSessions(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	query := openCodeSessionSelect + ` WHERE s.owner_id=$1 ORDER BY s.updated_at DESC`
	args := []any{c.UserID}
	if isSuperadmin(c.Role) {
		query = openCodeSessionSelect + ` ORDER BY s.updated_at DESC`
		args = nil
	}
	rows, err := h.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.OpenCodeSession{}
	for rows.Next() {
		s, err := scanOpenCodeSession(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Datenbankfehler")
			return
		}
		out = append(out, s.OpenCodeSession)
	}
	writeJSON(w, http.StatusOK, out)
}

// PostApiV1IntegrationsOpencodeSessions legt eine Session an: zuerst in
// OpenCode (directory = eigener Tenant-Workspace, Tools bis auf question
// deaktiviert), dann wird der Backend-MCP-Server registriert und die
// Verknüpfung in opencode_sessions gespeichert.
func (h *Server) PostApiV1IntegrationsOpencodeSessions(w http.ResponseWriter, r *http.Request) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	var req api.CreateOpenCodeSessionRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "ungültiges JSON")
			return
		}
	}
	title := ""
	if req.Title != nil {
		title = strings.TrimSpace(*req.Title)
	}
	if title == "" {
		title = "Lern-Chat"
	}
	base := h.openCodeBase()
	workspace := h.openCodeWorkspaceDir(c.UserID)
	client := openCodeClient()

	status, raw, err := openCodeDo(client, http.MethodPost,
		base+"/session?directory="+url.QueryEscape(workspace),
		map[string]any{"title": title, "permission": openCodeDenyAll})
	if err != nil {
		openCodeUnreachable(w, base)
		return
	}
	if status != http.StatusOK && status != http.StatusCreated {
		writeError(w, http.StatusBadGateway, "OpenCode-Session konnte nicht angelegt werden (Status "+strconv.Itoa(status)+")")
		return
	}
	var created struct {
		ID            string `json:"id"`
		ModelID       string `json:"modelID"`
		ProviderID    string `json:"providerID"`
		ModelProvider string `json:"modelProvider"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		writeError(w, http.StatusBadGateway, "OpenCode-Antwort unverständlich")
		return
	}

	// MCP-Server registrieren: Backend-URL ist aus dem opencode-Container
	// "http://backend:8080" (Compose-Netz). Der Tenant steckt im
	// Session-Token (X-Session-Token) — OpenCode sieht nie user_ids.
	var provider, model *string
	modelProvider := created.ModelProvider
	if modelProvider == "" {
		modelProvider = created.ProviderID
	}
	if modelProvider != "" {
		provider = &modelProvider
	}
	if created.ModelID != "" {
		model = &created.ModelID
	}
	sessionToken, ok := h.openCodeSessionToken(w, r, c.UserID)
	if !ok {
		h.openCodeDeleteSession(base, workspace, created.ID)
		return
	}
	if err := h.openCodeRegisterMCP(w, base, workspace, sessionToken); err != nil {
		h.openCodeDeleteSession(base, workspace, created.ID)
		return
	}

	var id int
	err = h.DB.QueryRowContext(r.Context(),
		`INSERT INTO opencode_sessions (owner_id, opencode_id, title, workspace, model_provider, model_id)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		c.UserID, created.ID, title, workspace, nullableString(provider), nullableString(model),
	).Scan(&id)
	if err != nil {
		h.openCodeDeleteSession(base, workspace, created.ID)
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	row := h.DB.QueryRowContext(r.Context(), openCodeSessionSelect+` WHERE s.id=$1`, id)
	s, err := scanOpenCodeSession(row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	writeJSON(w, http.StatusCreated, s.OpenCodeSession)
}

// openCodeSessionToken erzeugt ein kurzlebiges Session-Token (JWT-User
// eingebettet), das der MCP-Server als Tenant-Nachweis akzeptiert.
func (h *Server) openCodeSessionToken(w http.ResponseWriter, r *http.Request, userID int) (string, bool) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return "", false
	}
	token := hex.EncodeToString(raw[:])
	if _, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO opencode_session_tokens (token, user_id, expires_at) VALUES ($1,$2,NOW() + INTERVAL '24 hours')`,
		token, userID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return "", false
	}
	return token, true
}

// openCodeRegisterMCP registriert den Backend-MCP-Server in der
// OpenCode-Session (directory-scoped, tenant-isoliert). Remote-URL zeigt auf
// das Backend im Compose-Netz; der Tenant reist als Header mit.
func (h *Server) openCodeRegisterMCP(w http.ResponseWriter, base, workspace, sessionToken string) error {
	mcpURL := h.openCodeMCPURL()
	status, raw, err := openCodeDo(openCodeClient(), http.MethodPost,
		base+"/mcp?directory="+url.QueryEscape(workspace),
		map[string]any{
			"name": "smarttable",
			"config": map[string]any{
				"type":    "remote",
				"url":     mcpURL,
				"enabled": true,
				"headers": map[string]string{"X-Session-Token": sessionToken},
				"oauth":   false,
			},
		})
	if err != nil {
		openCodeUnreachable(w, base)
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		writeError(w, http.StatusBadGateway, "MCP-Server konnte nicht registriert werden (Status "+strconv.Itoa(status)+"): "+string(raw))
		return fmt.Errorf("mcp register status %d", status)
	}
	return nil
}

// openCodeMCPURL ist die Backend-MCP-URL aus Sicht des opencode-Containers
// (Compose-Netz: http://backend:8080/api/v1/mcp). Lokal ohne Compose via
// OPENCODE_MCP_URL überschreibbar. Der MCP-Server spricht beide Modi:
// SSE-Legacy (GET als Event-Stream, was `opencode serve` 1.17/1.18 für
// Remote-MCP verlangt) und Streamable HTTP (POST mit JSON-RPC).
func (h *Server) openCodeMCPURL() string {
	if env := strings.TrimSpace(os.Getenv("OPENCODE_MCP_URL")); env != "" {
		return strings.TrimRight(env, "/")
	}
	return "http://backend:8080/api/v1/mcp"
}

func (h *Server) openCodeDeleteSession(base, workspace, opencodeID string) {
	_, _, _ = openCodeDo(openCodeClient(), http.MethodDelete,
		base+"/session/"+url.PathEscape(opencodeID)+"?directory="+url.QueryEscape(workspace), nil)
}

// GetApiV1IntegrationsOpencodeSessionsId gibt den Verlauf zurück
// (älteste zuerst, aus opencode_messages).
func (h *Server) GetApiV1IntegrationsOpencodeSessionsId(w http.ResponseWriter, r *http.Request, id int) {
	if _, ok := h.openCodeSessionOwned(w, r, id); !ok {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, role, content, tokens, created_at FROM opencode_messages WHERE session_id=$1 ORDER BY id`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	defer rows.Close()
	out := []api.OpenCodeMessage{}
	for rows.Next() {
		var m api.OpenCodeMessage
		var created time.Time
		var role string
		var tokens *int
		if err := rows.Scan(&m.Id, &role, &m.Content, &tokens, &created); err != nil {
			writeError(w, http.StatusInternalServerError, "Datenbankfehler")
			return
		}
		m.Role = api.OpenCodeMessageRole(role)
		m.Tokens = tokens
		m.CreatedAt = created
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, out)
}

// DeleteApiV1IntegrationsOpencodeSessionsId löscht Session + Verlauf
// (CASCADE) und räumt die OpenCode-Session auf.
func (h *Server) DeleteApiV1IntegrationsOpencodeSessionsId(w http.ResponseWriter, r *http.Request, id int) {
	s, ok := h.openCodeSessionOwned(w, r, id)
	if !ok {
		return
	}
	if _, err := h.DB.ExecContext(r.Context(), `DELETE FROM opencode_sessions WHERE id=$1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	h.openCodeDeleteSession(h.openCodeBase(), s.Workspace, s.OpencodeID)
	w.WriteHeader(http.StatusNoContent)
}

type openCodePart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// PostApiV1IntegrationsOpencodeSessionsIdMessages schickt den User-Prompt an
// OpenCode (system-Prompt + Default-Modell + Tools bis auf question
// deaktiviert), speichert User-Nachricht + Agent-Antwort und pusht die
// Antwort zusätzlich als WebSocket-Event (type "opencode", session_id).
func (h *Server) PostApiV1IntegrationsOpencodeSessionsIdMessages(w http.ResponseWriter, r *http.Request, id int) {
	c := h.claims(w, r)
	if c == nil {
		return
	}
	s, ok := h.openCodeSessionOwned(w, r, id)
	if !ok {
		return
	}
	var req api.SendOpenCodeMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiges JSON")
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		writeError(w, http.StatusBadRequest, "content darf nicht leer sein")
		return
	}
	if len(content) > 8000 {
		writeError(w, http.StatusBadRequest, "Nachricht zu lang (max. 8000 Zeichen)")
		return
	}
	base := h.openCodeBase()
	msgBody := map[string]any{
		"system": openCodeSystemPrompt,
		"tools":  map[string]bool{"question": true},
		"parts":  []map[string]string{{"type": "text", "text": content}},
	}
	if model := openCodeModelPayload(h.openCodeModel()); model != nil {
		msgBody["model"] = model
	}
	status, raw, err := openCodeDo(openCodeClient(), http.MethodPost,
		base+"/session/"+url.PathEscape(s.OpencodeID)+"/message?directory="+url.QueryEscape(s.Workspace),
		msgBody)
	if err != nil {
		openCodeUnreachable(w, base)
		return
	}
	if status != http.StatusOK && status != http.StatusCreated {
		writeError(w, http.StatusBadGateway, "OpenCode antwortet nicht (Status "+strconv.Itoa(status)+")")
		return
	}
	var answer struct {
		Info struct {
			Tokens struct {
				Input  int `json:"input"`
				Output int `json:"output"`
				Total  int `json:"total"`
			} `json:"tokens"`
			Cost float64 `json:"cost"`
		} `json:"info"`
		Parts []openCodePart `json:"parts"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		writeError(w, http.StatusBadGateway, "OpenCode-Antwort unverständlich")
		return
	}
	var texts []string
	for _, p := range answer.Parts {
		if p.Type == "text" && strings.TrimSpace(p.Text) != "" {
			texts = append(texts, strings.TrimSpace(p.Text))
		}
	}
	reply := strings.Join(texts, "\n\n")
	if reply == "" {
		reply = "Ich habe dazu leider keine Antwort erhalten — versuch es anders zu formulieren."
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	defer tx.Rollback()
	zeroTokens := 0
	var userMsgID, asstMsgID int
	var created time.Time
	if err := tx.QueryRowContext(r.Context(),
		`INSERT INTO opencode_messages (session_id, role, content) VALUES ($1,'user',$2) RETURNING id, created_at`,
		id, content,
	).Scan(&userMsgID, &created); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	userMsg := api.OpenCodeMessage{Id: userMsgID, Role: "user", Content: content, Tokens: &zeroTokens, CreatedAt: created}
	tokens := answer.Info.Tokens.Total
	if err := tx.QueryRowContext(r.Context(),
		`INSERT INTO opencode_messages (session_id, role, content, tokens, cost) VALUES ($1,'assistant',$2,$3,$4) RETURNING id, created_at`,
		id, reply, tokens, answer.Info.Cost,
	).Scan(&asstMsgID, &created); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	asstMsg := api.OpenCodeMessage{Id: asstMsgID, Role: "assistant", Content: reply, Tokens: &tokens, CreatedAt: created}
	if _, err := tx.ExecContext(r.Context(), `UPDATE opencode_sessions SET updated_at=NOW() WHERE id=$1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Datenbankfehler")
		return
	}

	// Antwort zusätzlich als WebSocket-Event an den User (bestehende
	// WS-Infrastruktur, Typ "opencode" — der Chat-Socket ignoriert ihn,
	// der KI-Chat-Fallback aktualisiert sich darüber).
	if payload, err := json.Marshal(struct {
		Type      string              `json:"type"`
		SessionID int                 `json:"session_id"`
		Message   api.OpenCodeMessage `json:"message"`
	}{Type: "opencode", SessionID: id, Message: asstMsg}); err == nil {
		h.Hub.SendToUser(c.UserID, payload)
	}

	writeJSON(w, http.StatusCreated, []api.OpenCodeMessage{userMsg, asstMsg})
}

// opencodeSessionIDParam liest {id} (chi) für manuell registrierte Routen.
func opencodeSessionIDParam(r *http.Request) (int, error) {
	return strconv.Atoi(chi.URLParam(r, "id"))
}
