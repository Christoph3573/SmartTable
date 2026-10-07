package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOpenCodeSteps_Mapping(t *testing.T) {
	parts := []openCodePart{
		{ID: "prt_1", Type: "text", Text: "Hallo"},
		{ID: "prt_2", Type: "reasoning", Text: "Überlege …"},
		{ID: "prt_3", Type: "step-start"},
		{ID: "prt_4", Type: "step-finish"},
		{ID: "prt_5", Type: "snapshot"},
		{ID: "prt_6", Type: "fancy-future-type", Text: "x"},
		{
			ID: "prt_7", Type: "tool", CallID: "call_1", Tool: "smarttable_get_schedule",
			State: &openCodeToolState{Status: "completed", Input: json.RawMessage(`{"day":"tomorrow"}`), Output: "Mathe, Deutsch", Title: "get_schedule"},
		},
		{
			ID: "prt_8", Type: "tool", CallID: "call_2", Tool: "smarttable_get_homework",
			State: &openCodeToolState{Status: "running", Input: json.RawMessage(`{"class":"1a"}`)},
		},
		{
			ID: "prt_9", Type: "tool", CallID: "call_3", Tool: "smarttable_get_schedule",
			State: &openCodeToolState{Status: "error", Error: "keine Klasse zugeordnet"},
		},
		{ID: "prt_10", Type: "reasoning", Text: "   "},
	}
	steps := openCodeSteps(parts)
	if len(steps) != 4 {
		t.Fatalf("erwarte 4 Steps (reasoning+3 tools), bekam %d: %+v", len(steps), steps)
	}
	if steps[0].Kind != "reasoning" || steps[0].Status != "done" || steps[0].Key != "reasoning:prt_2" {
		t.Errorf("reasoning-Step falsch: %+v", steps[0])
	}
	if steps[0].Label != "Gedankengang" {
		t.Errorf("reasoning-Label falsch: %q", steps[0].Label)
	}
	if steps[1].Kind != "tool" || steps[1].Status != "done" || steps[1].Key != "tool:call_1" {
		t.Errorf("tool-Step (completed) falsch: %+v", steps[1])
	}
	if steps[1].Label != "Ruft Stundenplan ab" {
		t.Errorf("tool-Label falsch: %q", steps[1].Label)
	}
	if steps[2].Status != "running" {
		t.Errorf("tool-Step (running) falsch: %+v", steps[2])
	}
	if steps[3].Status != "error" {
		t.Errorf("tool-Step (error) falsch: %+v", steps[3])
	}
}

func TestOpenCodeSteps_Empty(t *testing.T) {
	if got := openCodeSteps(nil); len(got) != 0 {
		t.Errorf("nil → leer erwartet, bekam %+v", got)
	}
	if got := openCodeSteps([]openCodePart{{Type: "text", Text: "hi"}}); len(got) != 0 {
		t.Errorf("nur text → leer erwartet, bekam %+v", got)
	}
}

func TestOpenCodeTruncate(t *testing.T) {
	long := strings.Repeat("a", 600)
	got := openCodeTruncate(long, 500)
	if len([]rune(got)) > 520 {
		t.Errorf("zu lang: %d Runen", len([]rune(got)))
	}
	if !strings.Contains(got, "gekürzt") {
		t.Errorf("Hinweis fehlt: %q", got)
	}
	if got := openCodeTruncate("kurz", 500); got != "kurz" {
		t.Errorf("kurzer Text darf nicht verändert werden: %q", got)
	}
}

func TestOpenCodeToolLabel(t *testing.T) {
	cases := map[string]string{
		"smarttable_get_schedule":      "Ruft Stundenplan ab",
		"smarttable_get_substitutions": "Ruft Vertretungen ab",
		"smarttable_get_homework":      "Ruft Hausaufgaben ab",
		"smarttable_get_learning_plan": "Durchsucht LehrplanPLUS",
		"smarttable_get_vocabularies":  "Ruft Vokabeln ab",
		"smarttable_question":          "Stellt Rückfrage",
		"question":                     "Stellt Rückfrage",
		"smarttable_custom_thing":      "custom thing",
		"weird_tool":                   "weird tool",
	}
	for in, want := range cases {
		if got := openCodeToolLabel(in); got != want {
			t.Errorf("openCodeToolLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestParseOpenCodeMessageResponse_Shapes: POST-Antwort als Objekt ODER Array
// (v1.17 vs. v1.18) — der Steps-Bug: Final-Message enthält oft nur text-Parts,
// Tool-Parts stehen in früheren Messages und werden via Verlauf nachgeholt.
func TestParseOpenCodeMessageResponse_Shapes(t *testing.T) {
	obj := `{"info":{"tokens":{"input":1,"output":2,"total":3},"cost":0.1},"parts":[{"id":"p1","type":"text","text":"Hallo"}]}`
	ans, types, err := parseOpenCodeMessageResponse([]byte(obj))
	if err != nil {
		t.Fatalf("Objekt-Parse: %v", err)
	}
	if len(ans.Parts) != 1 || types[0] != "text" {
		t.Fatalf("Objekt falsch: %+v %v", ans, types)
	}
	// Array: letztes Element = Final (text-only), früheres enthält Tool-Part.
	arr := `[{"info":{"tokens":{"input":1,"output":1,"total":2}},"parts":[{"id":"t1","type":"tool","callID":"c1","tool":"smarttable_get_schedule","state":{"status":"completed","output":"Mathe"}}]},{"info":{"tokens":{"input":1,"output":2,"total":3}},"parts":[{"id":"p2","type":"text","text":"Morgen: Mathe"}]}]`
	ans, types, err = parseOpenCodeMessageResponse([]byte(arr))
	if err != nil {
		t.Fatalf("Array-Parse: %v", err)
	}
	if len(ans.Parts) != 1 || ans.Parts[0].Type != "text" {
		t.Fatalf("Array-Final falsch: %+v", ans.Parts)
	}
	hist := parseOpenCodeMessageHistory([]byte(arr))
	if len(hist) != 2 {
		t.Fatalf("Verlauf: erwarte 2 Parts (Tool + Text), bekam %d", len(hist))
	}
	steps := mergeOpenCodeSteps(openCodeSteps(ans.Parts), openCodeSteps(hist))
	if len(steps) != 1 || steps[0].Label != "Ruft Stundenplan ab" {
		t.Fatalf("Merge-Steps falsch: %+v", steps)
	}
}

func TestParseOpenCodeMessageResponse_Errors(t *testing.T) {
	if _, _, err := parseOpenCodeMessageResponse([]byte(``)); err == nil {
		t.Error("leer → Fehler erwartet")
	}
	if _, _, err := parseOpenCodeMessageResponse([]byte(`[]`)); err == nil {
		t.Error("leeres Array → Fehler erwartet")
	}
	if _, _, err := parseOpenCodeMessageResponse([]byte(`{kaputt`)); err == nil {
		t.Error("ungültiges JSON → Fehler erwartet")
	}
	if got := parseOpenCodeMessageHistory([]byte(`{kaputt`)); got != nil {
		t.Errorf("Verlauf defekt → nil erwartet, bekam %+v", got)
	}
}

func TestMergeOpenCodeSteps_Dedupe(t *testing.T) {
	a := []openCodeStepView{{Key: "tool:c1", Kind: "tool", Label: "x", Status: "running"}}
	b := []openCodeStepView{
		{Key: "tool:c1", Kind: "tool", Label: "x", Status: "done"},
		{Key: "reasoning:p1", Kind: "reasoning", Label: "Gedankengang", Status: "done"},
	}
	got := mergeOpenCodeSteps(a, b)
	if len(got) != 2 {
		t.Fatalf("Dedupe: erwarte 2, bekam %+v", got)
	}
	if got[0].Status != "running" {
		t.Errorf("erste Gruppe gewinnt: %+v", got[0])
	}
}

func TestOpenCodePermissionsFor_Scoped(t *testing.T) {
	perms := openCodePermissionsFor("/workspaces/42")
	allow := map[string]map[string]bool{}
	for _, p := range perms {
		if p["action"] == "allow" {
			if allow[p["permission"]] == nil {
				allow[p["permission"]] = map[string]bool{}
			}
			allow[p["permission"]][p["pattern"]] = true
		}
	}
	for _, tool := range []string{"read", "write", "edit", "bash", "glob", "grep"} {
		if !allow[tool]["/workspaces/42"] || !allow[tool]["/workspaces/42/**"] {
			t.Errorf("%s nicht workspace-scoped: %v", tool, allow[tool])
		}
	}
	if !allow["smarttable_*"]["*"] || !allow["question"]["*"] {
		t.Error("MCP/question-Allow fehlt")
	}
	// deny-Rest bleibt.
	deny := false
	for _, p := range perms {
		if p["permission"] == "execute" && p["action"] == "deny" {
			deny = true
		}
	}
	if !deny {
		t.Error("execute-deny fehlt")
	}
	// Leer → deny-all-Fallback.
	if got := openCodePermissionsFor(""); len(got) == 0 {
		t.Error("Fallback darf nicht leer sein")
	}
}

func TestOpenCodeFileHelpers(t *testing.T) {
	if !openCodeUploadExtOK("a.pdf") || !openCodeUploadExtOK("B.PNG") {
		t.Error("Allowlist pdf/png erwartet")
	}
	if openCodeUploadExtOK("evil.exe") || openCodeUploadExtOK("keine-endung") {
		t.Error("exe/ohne-Endung muss abgewiesen werden")
	}
	if got := openCodeSafeName("../../etc/passwd"); got != "passwd" {
		t.Errorf("Base-Sanitizing: %q", got)
	}
	if got := openCodeSafeName("  "); got != "" {
		t.Errorf("leer → '': %q", got)
	}
	dir := "/workspaces/7/uploads/s3"
	if p, ok := openCodeResolveInDir(dir, "abc.pdf"); !ok || p != dir+"/abc.pdf" {
		t.Errorf("resolve ok: %q %v", p, ok)
	}
	if _, ok := openCodeResolveInDir(dir, "../evil"); ok {
		t.Error("Traversal muss abgewiesen werden")
	}
	if _, ok := openCodeResolveInDir(dir, ""); ok {
		t.Error("leer muss abgewiesen werden")
	}
	// uploadPromptPaths: nur existierende Dateien listen.
	if got := uploadPromptPaths("/nonexistent-ws", 999, []string{"a.pdf"}); got != "" {
		t.Errorf("fehlende Datei → '': %q", got)
	}
}

func TestOpenCodeStep_PayloadContract(t *testing.T) {
	// WS-Vertrag: type/session_id/step{key,kind,label,status,detail?,tool?}
	step := openCodeStepView{Key: "tool:call_1", Kind: "tool", Label: "Ruft Stundenplan ab", Status: "done", Detail: "Mathe", Tool: "smarttable_get_schedule"}
	raw, err := json.Marshal(struct {
		Type      string           `json:"type"`
		SessionID int              `json:"session_id"`
		Step      openCodeStepView `json:"step"`
	}{Type: "opencode_step", SessionID: 42, Step: step})
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Type      string `json:"type"`
		SessionID int    `json:"session_id"`
		Step      struct {
			Key    string `json:"key"`
			Kind   string `json:"kind"`
			Label  string `json:"label"`
			Status string `json:"status"`
		} `json:"step"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != "opencode_step" || decoded.SessionID != 42 || decoded.Step.Key != "tool:call_1" {
		t.Errorf("Vertrag verletzt: %s", string(raw))
	}
}
