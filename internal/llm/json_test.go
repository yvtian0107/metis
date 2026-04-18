package llm

import (
	"encoding/json"
	"testing"
)

func TestExtractJSON_ValidJSON(t *testing.T) {
	input := `{"key": "value", "num": 42}`
	got := ExtractJSON(input)
	if got != input {
		t.Errorf("expected valid JSON to pass through, got %q", got)
	}
}

func TestExtractJSON_MarkdownCodeFence(t *testing.T) {
	input := "Here is the result:\n```json\n{\"key\": \"value\"}\n```\nDone."
	want := `{"key": "value"}`
	got := ExtractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_MarkdownCodeFenceNoLang(t *testing.T) {
	input := "Result:\n```\n{\"key\": \"value\"}\n```"
	want := `{"key": "value"}`
	got := ExtractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_TrailingComma(t *testing.T) {
	input := `{"items": ["a", "b",], "count": 2,}`
	got := ExtractJSON(input)
	// Should be valid JSON after repair
	if got == "" {
		t.Fatal("expected non-empty result")
	}
	// Verify it's valid JSON
	if !isValidJSON(got) {
		t.Errorf("expected valid JSON after repair, got %q", got)
	}
}

func TestExtractJSON_Empty(t *testing.T) {
	got := ExtractJSON("")
	if got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}
}

func TestExtractJSON_PlainText(t *testing.T) {
	// jsonrepair may attempt to process plain text; just ensure no panic
	got := ExtractJSON("this is just plain text without any json")
	_ = got // no panic is the test
}

func TestExtractJSON_NestedJSON(t *testing.T) {
	input := `{"outer": {"inner": [1, 2, 3]}, "ok": true}`
	got := ExtractJSON(input)
	if got != input {
		t.Errorf("expected nested JSON to pass through, got %q", got)
	}
}

func isValidJSON(s string) bool {
	var v any
	return len(s) > 0 && s[0] == '{' && json.Unmarshal([]byte(s), &v) == nil
}
