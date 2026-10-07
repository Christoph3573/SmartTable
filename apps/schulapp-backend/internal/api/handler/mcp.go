// Package handler — Backend-MCP-Server für OpenCode (KI-Lernchat).
//
// OpenCode bekommt Schuldaten ausschließlich über diese Tools:
//
//	get_schedule      — Stundenplan der Woche (eigene Klassen)
//	get_substitutions — Vertretungen (eigene Klassen, Zeitraum)
//	get_homework      — Hausaufgaben (eigene Klassen)
//	get_learning_plan — LehrplanPLUS-Suche (öffentlich, tenantlos)
//	get_vocabularies  — eigene Vokabelsets + fällige Karten
//
// Transport: JSON-RPC 2.0 (MCP Streamable HTTP) auf POST /api/v1/mcp mit den
// Methoden initialize, tools/list, tools/call (plus notifications/initialized
// als No-op). Der Tenant kommt NIE aus Client-Parametern, sondern aus dem
// JWT des Browsers (normale Authorization) oder — wenn OpenCode als MCP-Client
// fragt — aus dem Session-Token im Header X-Session-Token (Tabelle
// opencode_session_tokens, 24h gültig, bei Session-Anlage erzeugt). Damit kann
// OpenCode niemals Daten anderer User sehen und niemals direkt auf
// SchoolConnect-Credentials zugreifen — SchoolConnect bleibt für Credentials
// + externe Schuldaten zuständig, das Backend setzt dort immer X-SC-Tenant
// aus dem eingebetteten Tenant.
package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	appmw "schulapp/internal/middleware"
)

type mcpRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type mcpToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func mcpResult(id any, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

func mcpError(id any, code int, message string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id,
		"error": map[string]any{"code": code, "message": message}}
}

func mcpTextResult(text string) map[string]any {
	return map[string]any{"content": []map[string]string{{"type": "text", "text": text}}}
}

// mcpTenant löst den Tenant für einen MCP-Request auf: zuerst das normale
// JWT (Browser), sonst X-Session-Token (OpenCode als MCP-Client →
// opencode_session_tokens). Gibt userID + Claims zurück; schreibt 401 bei
// ungültigem/fehlendem Nachweis.
func (h *Server) mcpTenant(w http.ResponseWriter, r *http.Request) (int, *appmw.Claims) {
	if c := appmw.GetClaims(r); c != nil {
		return c.UserID, c
	}
	token := strings.TrimSpace(r.Header.Get("X-Session-Token"))
	if token == "" {
		writeError(w, http.StatusUnauthorized, "nicht autorisiert (JWT oder X-Session-Token erforderlich)")
		return 0, nil
	}
	var userID int
	var role string
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT t.user_id, u.role FROM opencode_session_tokens t
		  JOIN users u ON u.id = t.user_id AND u.active
		 WHERE t.token=$1 AND t.expires_at > NOW()`, token,
	).Scan(&userID, &role)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "ungültiges oder abgelaufenes Session-Token")
		return 0, nil
	}
	return userID, &appmw.Claims{UserID: userID, Role: role}
}

// mcpUserClasses liefert die Klassen-IDs, die der Tenant lesen darf
// (Mitglied oder Lehrer; Superadmin: alle).
func (h *Server) mcpUserClasses(r *http.Request, userID int, role string) []int {
	if isSuperadmin(role) {
		rows, err := h.DB.QueryContext(r.Context(), `SELECT id FROM classes`)
		if err != nil {
			return nil
		}
		defer rows.Close()
		var out []int
		for rows.Next() {
			var id int
			if rows.Scan(&id) == nil {
				out = append(out, id)
			}
		}
		return out
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT class_id FROM class_members WHERE user_id=$1
		UNION SELECT class_id FROM class_teachers WHERE user_id=$1`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}
	return out
}

func mcpStrArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	s, _ := args[key].(string)
	return strings.TrimSpace(s)
}

func mcpIntArg(args map[string]any, key string) (int, bool) {
	if args == nil {
		return 0, false
	}
	switch v := args[key].(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n), true
		}
	}
	return 0, false
}

// HandleMCP bedient den MCP-Endpunkt (POST /api/v1/mcp) in zwei Modi:
//
//  1. Streamable HTTP (POST mit JSON-RPC {initialize, tools/list,
//     tools/call, notifications/initialized}) — moderner Modus (MCP
//     2025-03+).
//  2. SSE-Stream (GET /api/v1/mcp mit Accept: text/event-stream) —
//     Legacy-Modus, den `opencode serve` (1.17/1.18) für Remote-MCP-Server
//     verlangt: Zuerst GET als Event-Stream öffnen (liefert die
//     `endpoint`-URI für Nachrichten), dann JSON-RPC-POSTs an diese URI.
//
// Der Tenant wird pro Request aus JWT oder X-Session-Token aufgelöst
// (mcpTenant) — niemals aus Client-Parametern.
func (h *Server) HandleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.handleMCPStream(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "nur GET (SSE) und POST (JSON-RPC) werden unterstützt")
		return
	}
	// SSE-Legacy-Nachrichten an die per handleMCPStream vergebene
	// Endpoint-URI: ?sessionId=... (Session ist tenant-gebunden).
	if r.URL.Query().Get("sessionId") != "" {
		h.handleMCPMessage(w, r)
		return
	}
	var req mcpRPCRequest
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, mcpError(nil, -32700, "ungültiges JSON"))
		return
	}

	switch req.Method {
	case "initialize":
		writeJSON(w, http.StatusOK, mcpResult(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "smarttable", "version": "1.0.0"},
		}))
		return
	case "notifications/initialized":
		// No-op-Bestätigung des Clients — kein Result nötig.
		w.WriteHeader(http.StatusNoContent)
		return
	case "tools/list":
		writeJSON(w, http.StatusOK, mcpResult(req.ID, mcpToolsList()))
		return
	case "tools/call":
		// Tenant JETZT auflösen (JWT oder Session-Token) — vor jedem Tool-Call.
		userID, claims := h.mcpTenant(w, r)
		if claims == nil {
			return
		}
		var params mcpToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			writeJSON(w, http.StatusOK, mcpError(req.ID, -32602, "ungültige Tool-Parameter"))
			return
		}
		text, isErr := h.mcpCallTool(r, userID, claims.Role, params.Name, params.Arguments)
		result := mcpTextResult(text)
		if isErr {
			result["isError"] = true
		}
		writeJSON(w, http.StatusOK, mcpResult(req.ID, result))
		return
	default:
		writeJSON(w, http.StatusOK, mcpError(req.ID, -32601, "Methode "+req.Method+" nicht unterstützt"))
	}
}

// mcpToolsList liefert das Tool-Verzeichnis (für beide Modi identisch).
func mcpToolsList() map[string]any {
	return map[string]any{
		"tools": []map[string]any{
			{
				"name":        "get_schedule",
				"description": "Stundenplan der Woche (TimetableEntries: Fach, Tag, Stunde, Vertretung/Ausfall). week_of optional (YYYY-MM-DD), class_id optional.",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"class_id": map[string]any{"type": "integer", "description": "Klassen-ID (Default: erste eigene Klasse)"},
						"week_of":  map[string]any{"type": "string", "description": "Datum in der Woche (YYYY-MM-DD, Default: aktuelle Woche)"},
					},
				},
			},
			{
				"name":        "get_substitutions",
				"description": "Vertretungen/Ausfälle/Raumwechsel im Zeitraum (Default: heute + 7 Tage). Nur eigene Klassen.",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"class_id":  map[string]any{"type": "integer"},
						"date_from": map[string]any{"type": "string", "description": "YYYY-MM-DD"},
						"date_to":   map[string]any{"type": "string", "description": "YYYY-MM-DD"},
					},
				},
			},
			{
				"name":        "get_homework",
				"description": "Hausaufgaben der eigenen Klassen (Titel, Fach, Fälligkeitsdatum). class_id optional (Default: alle eigenen Klassen).",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"class_id": map[string]any{"type": "integer"},
					},
				},
			},
			{
				"name":        "get_learning_plan",
				"description": "LehrplanPLUS (Bayern) durchsuchen: schulart/fach/jahrgangsstufe/lehrplankapitel/query. Öffentlich, kein Login nötig.",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"schulart":        map[string]any{"type": "string"},
						"fach":            map[string]any{"type": "string"},
						"jahrgangsstufe":  map[string]any{"type": "string"},
						"lehrplankapitel": map[string]any{"type": "string"},
						"query":           map[string]any{"type": "string", "description": "Suchtext"},
					},
				},
			},
			{
				"name":        "get_vocabularies",
				"description": "Eigene Vokabelsets + fällige Karten (Leitner). only_due=true (Default) liefert nur fällige Karten.",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"only_due": map[string]any{"type": "boolean"},
					},
				},
			},
		},
	}
}

// mcpCallTool führt ein Tool mit Tenant-Sichtbarkeit aus (nur lesend).
func (h *Server) mcpCallTool(r *http.Request, userID int, role, name string, args map[string]any) (string, bool) {
	switch name {
	case "get_schedule":
		return h.mcpSchedule(r, userID, role, args)
	case "get_substitutions":
		return h.mcpSubstitutions(r, userID, role, args)
	case "get_homework":
		return h.mcpHomework(r, userID, role, args)
	case "get_learning_plan":
		return h.mcpLearningPlan(args)
	case "get_vocabularies":
		return h.mcpVocab(r, userID, role, args)
	default:
		return "Unbekanntes Tool: " + name, true
	}
}

// mcpResolveClass prüft class_id gegen die Tenant-Sichtbarkeit bzw. wählt
// die erste eigene Klasse.
func (h *Server) mcpResolveClass(r *http.Request, userID int, role string, args map[string]any) (int, string, bool) {
	own := h.mcpUserClasses(r, userID, role)
	if len(own) == 0 {
		return 0, "Du bist noch keiner Klasse zugeordnet.", true
	}
	if id, ok := mcpIntArg(args, "class_id"); ok && id > 0 {
		for _, o := range own {
			if o == id {
				return id, "", false
			}
		}
		return 0, "Kein Zugriff auf diese Klasse.", true
	}
	return own[0], "", false
}

func (h *Server) mcpSchedule(r *http.Request, userID int, role string, args map[string]any) (string, bool) {
	// Ohne SmartTable-Klasse (z. B. SchoolConnect-Nutzer) auf das
	// Schülerportal zurückfallen, damit die KI trotzdem echte Daten bekommt.
	if len(h.mcpUserClasses(r, userID, role)) == 0 {
		return h.mcpScheduleExternal(r, userID)
	}
	classID, errText, isErr := h.mcpResolveClass(r, userID, role, args)
	if isErr {
		return errText, true
	}
	weekOf := mcpStrArg(args, "week_of")
	monday, friday := mcpWeekRange(weekOf)
	rows, err := h.DB.QueryContext(r.Context(), `SELECT l.day_of_week, l.period, s.name, l.room,
		sub.type, sub.room AS sub_room, sub.note
		FROM lessons l JOIN subjects s ON s.id=l.subject_id
		LEFT JOIN substitutions sub ON sub.class_id=l.class_id
		  AND sub.period=l.period AND sub.date = $3 + (l.day_of_week-1)
		WHERE l.class_id=$1 AND $3 + (l.day_of_week-1) BETWEEN $3 AND $4
		ORDER BY l.day_of_week, l.period`, classID, monday, monday, friday)
	if err != nil {
		return "Stundenplan konnte nicht geladen werden.", true
	}
	defer rows.Close()
	type entry struct {
		day     int
		period  int
		subject string
		room    string
		subType sql.NullString
		subRoom sql.NullString
		note    sql.NullString
	}
	var lines []string
	days := []string{"", "Mo", "Di", "Mi", "Do", "Fr"}
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.day, &e.period, &e.subject, &e.room, &e.subType, &e.subRoom, &e.note); err != nil {
			continue
		}
		room := e.room
		suffix := ""
		if e.subType.Valid {
			switch e.subType.String {
			case "cancellation":
				suffix = " (ENTFÄLLT)"
			case "room_change":
				if e.subRoom.Valid {
					room = e.subRoom.String
				}
				suffix = " (Raumwechsel)"
			default:
				suffix = " (Vertretung)"
				if e.subRoom.Valid && e.subRoom.String != "" {
					room = e.subRoom.String
				}
			}
			if e.note.Valid && e.note.String != "" {
				suffix += " [" + e.note.String + "]"
			}
		}
		day := days[e.day]
		if e.day < 1 || e.day > 5 {
			day = "Tag " + itoa(e.day)
		}
		lines = append(lines, day+" "+itoa(e.period)+". Std: "+e.subject+
			(roomStr(room))+suffix)
	}
	if len(lines) == 0 {
		return "Für diese Woche steht kein Stundenplan an (Klasse " + itoa(classID) + ").", false
	}
	return "Stundenplan (Woche ab " + monday.Format("02.01.2006") + "):\n" + strings.Join(lines, "\n"), false
}

func roomStr(room string) string {
	if strings.TrimSpace(room) == "" {
		return ""
	}
	return " (Raum " + room + ")"
}

// mcpWeekRange liefert Montag..Freitag der Woche um weekOf (YYYY-MM-DD);
// leer/ungültig → aktuelle Woche.
func mcpWeekRange(weekOf string) (time.Time, time.Time) {
	t := time.Now()
	if weekOf != "" {
		if parsed, err := time.Parse("2006-01-02", weekOf); err == nil {
			t = parsed
		}
	}
	monday := mondayOf(t)
	return monday, monday.AddDate(0, 0, 4)
}

func (h *Server) mcpSubstitutions(r *http.Request, userID int, role string, args map[string]any) (string, bool) {
	own := h.mcpUserClasses(r, userID, role)
	if len(own) == 0 {
		return h.mcpSubstitutionsExternal(r, userID, args)
	}
	from := mcpStrArg(args, "date_from")
	to := mcpStrArg(args, "date_to")
	if from == "" {
		from = time.Now().Format("2006-01-02")
	}
	if to == "" {
		t, _ := time.Parse("2006-01-02", from)
		to = t.AddDate(0, 0, 7).Format("2006-01-02")
	}
	var classFilter any
	if id, ok := mcpIntArg(args, "class_id"); ok && id > 0 {
		allowed := false
		for _, o := range own {
			if o == id {
				allowed = true
			}
		}
		if !allowed {
			return "Kein Zugriff auf diese Klasse.", true
		}
		classFilter = id
	}
	query := `SELECT sub.date, sub.period, c.name, s.name, sub.type, sub.room, sub.note
		FROM substitutions sub
		LEFT JOIN classes c ON c.id=sub.class_id
		LEFT JOIN subjects s ON s.id=sub.subject_id
		WHERE sub.date BETWEEN $1::date AND $2::date
		  AND (sub.class_id IS NULL OR $3 IS NULL OR sub.class_id=$3)
		  AND (sub.class_id IS NULL OR EXISTS(SELECT 1 FROM class_members cm WHERE cm.class_id=sub.class_id AND cm.user_id=$4)
		       OR EXISTS(SELECT 1 FROM class_teachers ct WHERE ct.class_id=sub.class_id AND ct.user_id=$4)`
	rowsArgs := []any{from, to, classFilter, userID}
	if isSuperadmin(role) {
		query = `SELECT sub.date, sub.period, c.name, s.name, sub.type, sub.room, sub.note
			FROM substitutions sub
			LEFT JOIN classes c ON c.id=sub.class_id
			LEFT JOIN subjects s ON s.id=sub.subject_id
			WHERE sub.date BETWEEN $1::date AND $2::date
			  AND ($3::int IS NULL OR sub.class_id=$3::int)`
		rowsArgs = []any{from, to, classFilter}
	}
	rows, err := h.DB.QueryContext(r.Context(), query, rowsArgs...)
	if err != nil {
		return "Vertretungen konnten nicht geladen werden.", true
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var date time.Time
		var period int
		var className, subject, typ sql.NullString
		var room, note sql.NullString
		if err := rows.Scan(&date, &period, &className, &subject, &typ, &room, &note); err != nil {
			continue
		}
		line := date.Format("02.01.") + " " + itoa(period) + ". Std"
		if className.Valid {
			line += " (" + className.String + ")"
		}
		if subject.Valid {
			line += ": " + subject.String
		}
		if typ.Valid {
			switch typ.String {
			case "cancellation":
				line += " — ENTFÄLLT"
			case "room_change":
				line += " — Raumwechsel"
			case "extra":
				line += " — Zusatzstunde"
			default:
				line += " — Vertretung"
			}
		}
		if room.Valid && room.String != "" {
			line += " (Raum " + room.String + ")"
		}
		if note.Valid && note.String != "" {
			line += " [" + note.String + "]"
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return "Keine Vertretungen vom " + from + " bis " + to + ".", false
	}
	return "Vertretungen (" + from + " bis " + to + "):\n" + strings.Join(lines, "\n"), false
}

func (h *Server) mcpHomework(r *http.Request, userID int, role string, args map[string]any) (string, bool) {
	own := h.mcpUserClasses(r, userID, role)
	if len(own) == 0 {
		return h.mcpHomeworkExternal(r, userID)
	}
	var classFilter any
	if id, ok := mcpIntArg(args, "class_id"); ok && id > 0 {
		allowed := false
		for _, o := range own {
			if o == id {
				allowed = true
			}
		}
		if !allowed {
			return "Kein Zugriff auf diese Klasse.", true
		}
		classFilter = id
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT hw.title, hw.description, hw.due_date, c.name, s.name
		FROM homework hw JOIN classes c ON c.id=hw.class_id
		LEFT JOIN subjects s ON s.id=hw.subject_id
		WHERE ($1::int IS NULL OR hw.class_id=$1::int)
		  AND EXISTS(SELECT 1 FROM class_members cm WHERE cm.class_id=hw.class_id AND cm.user_id=$2
		             UNION ALL SELECT 1 FROM class_teachers ct WHERE ct.class_id=hw.class_id AND ct.user_id=$2
		             UNION ALL SELECT 1 WHERE $3 IN ('superadmin','admin'))
		ORDER BY hw.due_date LIMIT 30`, classFilter, userID, role)
	if err != nil {
		return "Hausaufgaben konnten nicht geladen werden.", true
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var title string
		var desc sql.NullString
		var due time.Time
		var className string
		var subject sql.NullString
		if err := rows.Scan(&title, &desc, &due, &className, &subject); err != nil {
			continue
		}
		line := "• " + title + " (" + className
		if subject.Valid {
			line += ", " + subject.String
		}
		line += ", fällig " + due.Format("02.01.2006") + ")"
		if desc.Valid && strings.TrimSpace(desc.String) != "" {
			line += " — " + desc.String
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return "Aktuell stehen keine Hausaufgaben an.", false
	}
	return "Hausaufgaben:\n" + strings.Join(lines, "\n"), false
}

// mcpSchoolConnectData ruft eine Schülerportal-Funktion über den
// SchoolConnect-Proxy mit dem Tenant des Users ab und liefert das `data`-Feld.
func (h *Server) mcpSchoolConnectData(r *http.Request, userID int, function string) (json.RawMessage, bool) {
	target := h.schoolConnectBase() + "/api/schuelerportal/" + function
	status, raw, err := schoolConnectDo(schoolConnectClient(), http.MethodGet, target, itoa(userID), nil)
	if err != nil || status != http.StatusOK {
		return nil, false
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Data) == 0 {
		return nil, false
	}
	return env.Data, true
}

var mcpDayLabels = []string{"Mo", "Di", "Mi", "Do", "Fr"}

// mcpScheduleExternal liefert den Schülerportal-Stundenplan als Text.
func (h *Server) mcpScheduleExternal(r *http.Request, userID int) (string, bool) {
	data, ok := h.mcpSchoolConnectData(r, userID, "stundenplan")
	if !ok {
		return "Weder eine SmartTable-Klasse zugeordnet noch das Schülerportal (SchoolConnect) erreichbar — bitte in den Einstellungen anmelden.", true
	}
	var tt struct {
		Schule    string `json:"schule"`
		Eintraege []struct {
			Tag    string `json:"tag"`
			Day    int    `json:"day"`
			Stunde int    `json:"stunde"`
			Kurs   string `json:"kurs"`
			Raum   string `json:"raum"`
		} `json:"eintraege"`
	}
	if err := json.Unmarshal(data, &tt); err != nil {
		return "Stundenplan-Antwort des Schülerportals unverständlich.", true
	}
	if len(tt.Eintraege) == 0 {
		return "Das Schülerportal meldet keine Stundenplaneinträge.", false
	}
	lines := make([]string, 0, len(tt.Eintraege))
	for _, e := range tt.Eintraege {
		day := e.Tag
		if e.Day >= 0 && e.Day < len(mcpDayLabels) {
			day = mcpDayLabels[e.Day]
		}
		line := day + " " + itoa(e.Stunde) + ". Std: " + e.Kurs
		if e.Raum != "" {
			line += " (Raum " + e.Raum + ")"
		}
		lines = append(lines, line)
	}
	prefix := "Stundenplan (Schülerportal via SchoolConnect)"
	if tt.Schule != "" {
		prefix += " — " + tt.Schule
	}
	return prefix + ":\n" + strings.Join(lines, "\n"), false
}

// mcpSubstitutionsExternal liefert die Schülerportal-Vertretungen als Text.
func (h *Server) mcpSubstitutionsExternal(r *http.Request, userID int, args map[string]any) (string, bool) {
	params := urlValues{}
	if datum := mcpStrArg(args, "date_from"); datum != "" {
		params["datum"] = datum
	} else if datum := mcpStrArg(args, "date"); datum != "" {
		params["datum"] = datum
	}
	function := "vertretungsplan"
	target := h.schoolConnectBase() + "/api/schuelerportal/" + function
	if len(params) > 0 {
		target += "?" + params.encode()
	}
	status, raw, err := schoolConnectDo(schoolConnectClient(), http.MethodGet, target, itoa(userID), nil)
	if err != nil || status != http.StatusOK {
		return "Vertretungsplan konnte weder aus SmartTable noch aus dem Schülerportal geladen werden.", true
	}
	var env struct {
		Data struct {
			Eintraege []map[string]any `json:"eintraege"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "Vertretungs-Antwort des Schülerportals unverständlich.", true
	}
	if len(env.Data.Eintraege) == 0 {
		return "Das Schülerportal meldet aktuell keine Vertretungen.", false
	}
	lines := make([]string, 0, len(env.Data.Eintraege))
	for _, e := range env.Data.Eintraege {
		lines = append(lines, "• "+mcpAnyStr(e, "date", "datum", "tag")+" "+
			mcpAnyStr(e, "hour", "stunde", "period")+". Std: "+
			mcpAnyStr(e, "uf", "kurs", "fach", "class")+
			mcpSubDetail(e))
	}
	return "Vertretungen (Schülerportal via SchoolConnect):\n" + strings.Join(lines, "\n"), false
}

func mcpSubDetail(e map[string]any) string {
	var parts []string
	if room := mcpAnyStr(e, "room", "raum"); room != "" {
		parts = append(parts, "Raum "+room)
	}
	if reason := mcpAnyStr(e, "reason", "grund", "text", "note"); reason != "" {
		parts = append(parts, reason)
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// mcpHomeworkExternal liefert die Schülerportal-Hausaufgaben als Text.
func (h *Server) mcpHomeworkExternal(r *http.Request, userID int) (string, bool) {
	data, ok := h.mcpSchoolConnectData(r, userID, "hausaufgaben")
	if !ok {
		return "Hausaufgaben konnten weder aus SmartTable noch aus dem Schülerportal geladen werden.", true
	}
	var hw struct {
		Aufgaben []map[string]any `json:"aufgaben"`
	}
	if err := json.Unmarshal(data, &hw); err != nil {
		return "Hausaufgaben-Antwort des Schülerportals unverständlich.", true
	}
	if len(hw.Aufgaben) == 0 {
		return "Das Schülerportal meldet aktuell keine Hausaufgaben.", false
	}
	lines := make([]string, 0, len(hw.Aufgaben))
	for _, a := range hw.Aufgaben {
		line := "• " + mcpAnyStr(a, "titel", "title", "aufgabe", "text")
		if fach := mcpAnyStr(a, "fach", "uf", "kurs", "subject"); fach != "" {
			line += " (" + fach + ")"
		}
		if due := mcpAnyStr(a, "faellig", "fällig", "due", "due_date", "datum", "abgabedatum"); due != "" {
			line += " — fällig " + due
		}
		lines = append(lines, line)
	}
	return "Hausaufgaben (Schülerportal via SchoolConnect):\n" + strings.Join(lines, "\n"), false
}

// mcpAnyStr liest den ersten nicht-leeren String-Wert aus mehreren möglichen
// Schlüsseln (die Schülerportal-Felder sind nicht strikt spezifiziert).
func mcpAnyStr(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return strings.TrimSpace(t)
				}
			case float64:
				return strconv.FormatFloat(t, 'f', -1, 64)
			case json.Number:
				return t.String()
			}
		}
	}
	return ""
}

// mcpLearningPlan fragt LehrplanPLUS über SchoolConnect (tenantlos,
// öffentlich — kein Login nötig). Nutzt query-Args als Suchparameter.
func (h *Server) mcpLearningPlan(args map[string]any) (string, bool) {
	params := urlValues{}
	for _, k := range []string{"schulart", "fach", "jahrgangsstufe", "lehrplankapitel", "query", "q", "search", "fachbereich"} {
		if v := mcpStrArg(args, k); v != "" {
			out := v
			if k == "query" || k == "q" || k == "search" {
				k = "query"
			}
			_ = out
			params[k] = v
		}
	}
	base := h.schoolConnectBase()
	target := base + "/api/lernplan-bayern/search"
	if len(params) > 0 {
		target += "?" + params.encode()
	}
	status, raw, err := schoolConnectDo(schoolConnectClient(), http.MethodGet, target, "", nil)
	if err != nil {
		return "LehrplanPLUS ist gerade nicht erreichbar.", true
	}
	if status != http.StatusOK {
		return "LehrplanPLUS antwortet nicht (Status " + itoa(status) + ").", true
	}
	var envelope struct {
		Data any `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data == nil {
		return "LehrplanPLUS-Antwort unverständlich.", true
	}
	pretty, _ := json.Marshal(envelope.Data)
	text := string(pretty)
	if len(text) > 4000 {
		text = text[:4000] + "… (gekürzt)"
	}
	if text == "" || text == "null" {
		return "Keine Lehrplan-Treffer gefunden.", false
	}
	return "LehrplanPLUS-Treffer:\n" + text, false
}

type urlValues map[string]string

func (v urlValues) encode() string {
	q := url.Values{}
	for k, val := range v {
		q.Set(k, val)
	}
	return q.Encode()
}

// mcpVocab fasst eigene Vokabelsets + fällige Karten zusammen
// (Sichtbarkeit wie GET /api/v1/vocab/sets: eigene + Klassensets).
func (h *Server) mcpVocab(r *http.Request, userID int, role string, args map[string]any) (string, bool) {
	onlyDue := true
	if v, ok := args["only_due"]; ok {
		if b, ok := v.(bool); ok {
			onlyDue = b
		}
	}
	query := `SELECT s.id, s.title, s.source_lang, s.target_lang,
		(SELECT COUNT(*) FROM vocab_cards c WHERE c.set_id=s.id),
		(SELECT COUNT(*) FROM vocab_cards c WHERE c.set_id=s.id AND c.due_at <= NOW())
		FROM vocab_sets s
		WHERE s.owner_id=$1
		   OR (s.class_id IS NOT NULL AND (
		         EXISTS(SELECT 1 FROM class_members cm WHERE cm.class_id=s.class_id AND cm.user_id=$1)
		      OR EXISTS(SELECT 1 FROM class_teachers ct WHERE ct.class_id=s.class_id AND ct.user_id=$1)))
		ORDER BY s.title`
	rowsArgs := []any{userID}
	if isSuperadmin(role) {
		query = `SELECT s.id, s.title, s.source_lang, s.target_lang,
			(SELECT COUNT(*) FROM vocab_cards c WHERE c.set_id=s.id),
			(SELECT COUNT(*) FROM vocab_cards c WHERE c.set_id=s.id AND c.due_at <= NOW())
			FROM vocab_sets s ORDER BY s.title`
		rowsArgs = nil
	}
	rows, err := h.DB.QueryContext(r.Context(), query, rowsArgs...)
	if err != nil {
		return "Vokabeln konnten nicht geladen werden.", true
	}
	defer rows.Close()
	type setInfo struct {
		id, total, due  int
		title, src, dst string
	}
	var sets []setInfo
	dueTotal := 0
	for rows.Next() {
		var s setInfo
		if err := rows.Scan(&s.id, &s.title, &s.src, &s.dst, &s.total, &s.due); err != nil {
			continue
		}
		sets = append(sets, s)
		dueTotal += s.due
	}
	if len(sets) == 0 {
		return "Noch keine Vokabelsets angelegt — lege unter „Lernen → Vokabeln“ ein Set an.", false
	}
	var lines []string
	for _, s := range sets {
		lines = append(lines, "• "+s.title+" ("+s.src+"→"+s.dst+"): "+itoa(s.total)+" Karten, "+itoa(s.due)+" fällig")
	}
	head := "Vokabelsets (" + itoa(len(sets)) + ", " + itoa(dueTotal) + " Karten fällig):\n" + strings.Join(lines, "\n")
	if !onlyDue || dueTotal == 0 {
		return head, false
	}
	// Fällige Karten auflisten (Front-Seiten + Hinweis, max. 20).
	rows2, err := h.DB.QueryContext(r.Context(), `SELECT s.title, c.front, c.hint, c.box
		FROM vocab_cards c JOIN vocab_sets s ON s.id=c.set_id
		WHERE c.due_at <= NOW() AND (s.owner_id=$1
		   OR (s.class_id IS NOT NULL AND (
		         EXISTS(SELECT 1 FROM class_members cm WHERE cm.class_id=s.class_id AND cm.user_id=$1)
		      OR EXISTS(SELECT 1 FROM class_teachers ct WHERE ct.class_id=s.class_id AND ct.user_id=$1))))
		ORDER BY c.due_at LIMIT 20`, userID)
	if err != nil {
		return head, false
	}
	defer rows2.Close()
	var due []string
	for rows2.Next() {
		var title, front string
		var hint sql.NullString
		var box int
		if err := rows2.Scan(&title, &front, &hint, &box); err != nil {
			continue
		}
		line := "• [" + title + "] " + front + " (Box " + itoa(box) + ")"
		if hint.Valid && hint.String != "" {
			line += " — Hinweis: " + hint.String
		}
		due = append(due, line)
	}
	if len(due) == 0 {
		return head, false
	}
	return head + "\n\nFällige Karten (ohne Lösungen — abfragen!):\n" + strings.Join(due, "\n"), false
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

type mcpSSESession struct {
	userID  int
	role    string
	out     chan []byte
	created time.Time
}

var mcpSSESessions = struct {
	sync.RWMutex
	m map[string]*mcpSSESession
}{m: map[string]*mcpSSESession{}}

func mcpNewSSESession(userID int, role string) (string, *mcpSSESession) {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	id := hex.EncodeToString(raw[:])
	s := &mcpSSESession{userID: userID, role: role, out: make(chan []byte, 16), created: time.Now()}
	mcpSSESessions.Lock()
	mcpSSESessions.m[id] = s
	mcpSSESessions.Unlock()
	return id, s
}

func mcpGetSSESession(id string) *mcpSSESession {
	mcpSSESessions.RLock()
	defer mcpSSESessions.RUnlock()
	return mcpSSESessions.m[id]
}

func mcpDropSSESession(id string) {
	mcpSSESessions.Lock()
	delete(mcpSSESessions.m, id)
	mcpSSESessions.Unlock()
}

// handleMCPStream öffnet den SSE-Stream (Legacy-Modus): Tenant auflösen,
// Session anlegen, `endpoint`-Event schicken, dann blockieren bis der Client
// trennt. Responses auf tools/list/call werden via Session-Channel als
// `message`-Events geschrieben.
func (h *Server) handleMCPStream(w http.ResponseWriter, r *http.Request) {
	userID, claims := h.mcpTenant(w, r)
	if claims == nil {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "SSE wird nicht unterstützt")
		return
	}
	sessionID, session := mcpNewSSESession(userID, claims.Role)
	defer mcpDropSSESession(sessionID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	endpoint := "/api/v1/mcp/messages?sessionId=" + url.QueryEscape(sessionID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpoint)
	flusher.Flush()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case payload := <-session.out:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", payload)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// handleMCPMessage bedient JSON-RPC-POSTs an die Endpoint-URI
// (?sessionId=...): Tenant aus der SSE-Session, Ergebnis als SSE-Event auf
// dem offenen Stream + (für initialize/tools/list) zusätzlich als HTTP-Body,
// damit beide Client-Varianten bedient sind.
func (h *Server) handleMCPMessage(w http.ResponseWriter, r *http.Request) {
	session := mcpGetSSESession(r.URL.Query().Get("sessionId"))
	if session == nil {
		writeJSON(w, http.StatusOK, mcpError(nil, -32001, "unbekannte SSE-Session (Stream zuerst via GET öffnen)"))
		return
	}
	var req mcpRPCRequest
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, mcpError(nil, -32700, "ungültiges JSON"))
		return
	}
	reply := func(v any) {
		raw, _ := json.Marshal(v)
		select {
		case session.out <- raw:
		default:
		}
	}
	switch req.Method {
	case "initialize":
		res := mcpResult(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "smarttable", "version": "1.0.0"},
		})
		reply(res)
		writeJSON(w, http.StatusOK, res)
		return
	case "notifications/initialized":
		w.WriteHeader(http.StatusNoContent)
		return
	case "tools/list":
		res := mcpResult(req.ID, mcpToolsList())
		reply(res)
		writeJSON(w, http.StatusOK, res)
		return
	case "tools/call":
		var params mcpToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			res := mcpError(req.ID, -32602, "ungültige Tool-Parameter")
			reply(res)
			writeJSON(w, http.StatusOK, res)
			return
		}
		text, isErr := h.mcpCallTool(r, session.userID, session.role, params.Name, params.Arguments)
		result := mcpTextResult(text)
		if isErr {
			result["isError"] = true
		}
		res := mcpResult(req.ID, result)
		reply(res)
		// Antwort NUR als SSE-Event (kein HTTP-Body): Der Client wartet auf
		// dem Stream. 202 ohne Body signalisiert Annahme.
		w.WriteHeader(http.StatusAccepted)
		return
	default:
		res := mcpError(req.ID, -32601, "Methode "+req.Method+" nicht unterstützt")
		reply(res)
		writeJSON(w, http.StatusOK, res)
	}
}

// --- SSE-Legacy-Modus (für `opencode serve` als Remote-MCP-Client) ---
//
// Ablauf: Client öffnet GET /api/v1/mcp (Accept: text/event-stream) und
// bekommt zuerst ein `endpoint`-Event mit der Nachrichten-URI
// (/api/v1/mcp/messages?sessionId=...). Danach schickt er JSON-RPC-POSTs
// (initialize, tools/list, tools/call) an diese URI; Responses/Requests
// kommen als SSE-Events auf dem offenen Stream zurück.
//
// Tenant-Bindung: Die SSE-Session wird beim GET mit JWT oder
// X-Session-Token erzeugt und speichert userID+role serverseitig —
// handleMCPMessage löst den Tenant aus der Session-ID auf, niemals aus
// Client-Parametern.
