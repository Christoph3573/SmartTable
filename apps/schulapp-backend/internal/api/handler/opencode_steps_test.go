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
