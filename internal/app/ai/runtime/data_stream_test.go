package runtime

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func extractDataLines(t *testing.T, buf *bytes.Buffer) []string {
	t.Helper()
	var lines []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.HasPrefix(line, "data: ") {
			lines = append(lines, strings.TrimPrefix(line, "data: "))
		}
	}
	return lines
}

func TestUIMessageStreamEncoder_TextDelta(t *testing.T) {
	var buf bytes.Buffer
	enc := NewUIMessageStreamEncoder(&buf)

	_ = enc.Encode(Event{Type: EventTypeLLMStart, Sequence: 1})
	_ = enc.Encode(Event{Type: EventTypeContentDelta, Sequence: 2, Text: "Hello "})
	_ = enc.Encode(Event{Type: EventTypeContentDelta, Sequence: 3, Text: "world"})
	_ = enc.Encode(Event{Type: EventTypeDone, InputTokens: 10, OutputTokens: 20})
	_ = enc.Close()

	lines := extractDataLines(t, &buf)
	if len(lines) < 6 {
		t.Fatalf("expected at least 6 data lines, got %d: %v", len(lines), lines)
	}
	assertAIUIMessageChunkSchemaSubset(t, lines)

	assertJSONField(t, lines[0], "type", "start")
	assertJSONField(t, lines[1], "type", "text-start")
	assertJSONField(t, lines[2], "type", "text-delta")
	assertJSONField(t, lines[2], "delta", "Hello ")
	assertJSONField(t, lines[3], "type", "text-delta")
	assertJSONField(t, lines[3], "delta", "world")
	assertJSONField(t, lines[4], "type", "text-end")
	assertJSONField(t, lines[5], "type", "message-metadata")
	assertNestedJSONField(t, lines[5], "messageMetadata", "usage", map[string]any{
		"promptTokens":     float64(10),
		"completionTokens": float64(20),
	})
	assertJSONField(t, lines[6], "type", "finish")
	assertJSONMissingField(t, lines[6], "usage")
	if lines[len(lines)-1] != "[DONE]" {
		t.Errorf("expected last line to be [DONE], got %s", lines[len(lines)-1])
	}
}

func TestUIMessageStreamEncoder_EmitsSingleStartAcrossReactTurns(t *testing.T) {
	var buf bytes.Buffer
	enc := NewUIMessageStreamEncoder(&buf)

	_ = enc.Encode(Event{Type: EventTypeLLMStart, Sequence: 1})
	_ = enc.Encode(Event{Type: EventTypeToolCall, Sequence: 2, ToolCallID: "call_1", ToolName: "search", ToolArgs: json.RawMessage(`{"q":"x"}`)})
	_ = enc.Encode(Event{Type: EventTypeToolResult, Sequence: 3, ToolCallID: "call_1", ToolOutput: "result"})
	_ = enc.Encode(Event{Type: EventTypeLLMStart, Sequence: 4})
	_ = enc.Encode(Event{Type: EventTypeContentDelta, Sequence: 5, Text: "Done"})
	_ = enc.Encode(Event{Type: EventTypeDone})
	_ = enc.Close()

	lines := extractDataLines(t, &buf)
	startCount := 0
	for _, line := range lines {
		if line == "[DONE]" {
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			t.Fatalf("invalid json line %q: %v", line, err)
		}
		if chunk["type"] == "start" {
			startCount++
			if chunk["messageId"] != "msg-1" {
				t.Fatalf("expected first start messageId to remain stable, got %#v", chunk["messageId"])
			}
		}
	}
	if startCount != 1 {
		t.Fatalf("expected one start chunk across one gateway run, got %d: %v", startCount, lines)
	}
}

func assertAIUIMessageChunkSchemaSubset(t *testing.T, lines []string) {
	t.Helper()
	allowed := map[string]map[string]bool{
		"start":                 {"type": true, "messageId": true},
		"text-start":            {"type": true, "id": true},
		"text-delta":            {"type": true, "id": true, "delta": true},
		"text-end":              {"type": true, "id": true},
		"reasoning-start":       {"type": true, "id": true},
		"reasoning-delta":       {"type": true, "id": true, "delta": true},
		"reasoning-end":         {"type": true, "id": true},
		"tool-input-available":  {"type": true, "toolCallId": true, "toolName": true, "input": true},
		"tool-output-available": {"type": true, "toolCallId": true, "output": true},
		"data-plan":             {"type": true, "data": true},
		"data-step":             {"type": true, "data": true},
		"data-ui-surface":       {"type": true, "id": true, "data": true},
		"message-metadata":      {"type": true, "messageMetadata": true},
		"finish":                {"type": true, "finishReason": true},
		"error":                 {"type": true, "errorText": true},
	}
	required := map[string][]string{
		"start":                 {"messageId"},
		"text-start":            {"id"},
		"text-delta":            {"id", "delta"},
		"text-end":              {"id"},
		"reasoning-start":       {"id"},
		"reasoning-delta":       {"id", "delta"},
		"reasoning-end":         {"id"},
		"tool-input-available":  {"toolCallId", "toolName", "input"},
		"tool-output-available": {"toolCallId", "output"},
		"data-plan":             {"data"},
		"data-step":             {"data"},
		"data-ui-surface":       {"id", "data"},
		"message-metadata":      {"messageMetadata"},
		"finish":                {"finishReason"},
		"error":                 {"errorText"},
	}

	for _, line := range lines {
		if line == "[DONE]" {
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			t.Fatalf("invalid json line %q: %v", line, err)
		}
		chunkType, ok := chunk["type"].(string)
		if !ok || chunkType == "" {
			t.Fatalf("chunk missing string type: %s", line)
		}
		allowedKeys, ok := allowed[chunkType]
		if !ok {
			t.Fatalf("unsupported chunk type %q in %s", chunkType, line)
		}
		for key := range chunk {
			if !allowedKeys[key] {
				t.Fatalf("chunk type %q has schema-incompatible key %q in %s", chunkType, key, line)
			}
		}
		for _, key := range required[chunkType] {
			if _, ok := chunk[key]; !ok {
				t.Fatalf("chunk type %q missing required key %q in %s", chunkType, key, line)
			}
		}
	}
}

func TestUIMessageStreamEncoder_Reasoning(t *testing.T) {
	var buf bytes.Buffer
	enc := NewUIMessageStreamEncoder(&buf)

	_ = enc.Encode(Event{Type: EventTypeLLMStart, Sequence: 1})
	_ = enc.Encode(Event{Type: EventTypeThinkingDelta, Sequence: 2, Text: "think"})
	_ = enc.Encode(Event{Type: EventTypeThinkingDone, Sequence: 3})
	_ = enc.Encode(Event{Type: EventTypeDone})
	_ = enc.Close()

	lines := extractDataLines(t, &buf)
	assertJSONField(t, lines[1], "type", "reasoning-start")
	assertJSONField(t, lines[2], "type", "reasoning-delta")
	assertJSONField(t, lines[3], "type", "reasoning-end")
	assertJSONField(t, lines[4], "type", "finish")
}

func TestUIMessageStreamEncoder_ToolCallAndResult(t *testing.T) {
	var buf bytes.Buffer
	enc := NewUIMessageStreamEncoder(&buf)

	_ = enc.Encode(Event{Type: EventTypeLLMStart, Sequence: 1})
	_ = enc.Encode(Event{Type: EventTypeContentDelta, Sequence: 2, Text: "Let me check"})
	_ = enc.Encode(Event{Type: EventTypeToolCall, Sequence: 3, ToolCallID: "call_1", ToolName: "search", ToolArgs: json.RawMessage(`{"q":"x"}`)})
	_ = enc.Encode(Event{Type: EventTypeToolResult, Sequence: 4, ToolCallID: "call_1", ToolName: "search", ToolOutput: "result"})
	_ = enc.Encode(Event{Type: EventTypeDone})
	_ = enc.Close()

	lines := extractDataLines(t, &buf)
	assertJSONField(t, lines[1], "type", "text-start")
	assertJSONField(t, lines[2], "type", "text-delta")
	assertJSONField(t, lines[3], "type", "text-end")
	assertJSONField(t, lines[4], "type", "tool-input-available")
	assertJSONField(t, lines[4], "toolName", "search")
	assertJSONField(t, lines[5], "type", "tool-output-available")
	assertJSONField(t, lines[5], "output", "result")
}

func assertNestedJSONField(t *testing.T, line, nestedKey, key string, expected any) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid json line %q: %v", line, err)
	}
	nested, ok := m[nestedKey].(map[string]any)
	if !ok {
		t.Fatalf("missing or invalid nested key %q in %s", nestedKey, line)
	}
	v, ok := nested[key]
	if !ok {
		t.Fatalf("missing key %q in nested %q of %s", key, nestedKey, line)
	}
	if gotJSON, expectedJSON := canonicalJSON(t, v), canonicalJSON(t, expected); gotJSON != expectedJSON {
		t.Errorf("%s.%s: expected %s, got %s", nestedKey, key, expectedJSON, gotJSON)
	}
}

func canonicalJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}
	return string(data)
}

func TestUIMessageStreamEncoder_PlanAndSteps(t *testing.T) {
	var buf bytes.Buffer
	enc := NewUIMessageStreamEncoder(&buf)

	_ = enc.Encode(Event{Type: EventTypeLLMStart, Sequence: 1})
	_ = enc.Encode(Event{Type: EventTypePlan, Sequence: 2, Steps: []PlanStep{{Index: 1, Description: "step1"}}})
	_ = enc.Encode(Event{Type: EventTypeStepStart, Sequence: 3, StepIndex: 1, Description: "step1"})
	_ = enc.Encode(Event{Type: EventTypeStepDone, Sequence: 4, StepIndex: 1, DurationMs: 100})
	_ = enc.Encode(Event{Type: EventTypeDone})
	_ = enc.Close()

	lines := extractDataLines(t, &buf)
	assertJSONField(t, lines[1], "type", "data-plan")
	assertJSONField(t, lines[2], "type", "data-step")
	assertNestedJSONField(t, lines[2], "data", "state", "start")
	assertJSONField(t, lines[3], "type", "data-step")
	assertNestedJSONField(t, lines[3], "data", "state", "done")
}

func TestUIMessageStreamEncoder_UISurface(t *testing.T) {
	var buf bytes.Buffer
	enc := NewUIMessageStreamEncoder(&buf)

	_ = enc.Encode(Event{Type: EventTypeLLMStart, Sequence: 1})
	_ = enc.Encode(Event{
		Type:        EventTypeUISurface,
		Sequence:    2,
		SurfaceID:   "draft-1",
		SurfaceType: "itsm.draft_form",
		SurfaceData: json.RawMessage(`{"status":"ready"}`),
	})
	_ = enc.Encode(Event{Type: EventTypeDone})
	_ = enc.Close()

	lines := extractDataLines(t, &buf)
	assertJSONField(t, lines[1], "type", "data-ui-surface")
	assertJSONField(t, lines[1], "id", "draft-1")
	assertNestedJSONField(t, lines[1], "data", "surfaceId", "draft-1")
	assertNestedJSONField(t, lines[1], "data", "surfaceType", "itsm.draft_form")
}

func TestUIMessageStreamEncoder_Error(t *testing.T) {
	var buf bytes.Buffer
	enc := NewUIMessageStreamEncoder(&buf)

	_ = enc.Encode(Event{Type: EventTypeLLMStart, Sequence: 1})
	_ = enc.Encode(Event{Type: EventTypeError, Message: "boom"})
	_ = enc.Close()

	lines := extractDataLines(t, &buf)
	assertJSONField(t, lines[1], "type", "error")
	assertJSONField(t, lines[1], "errorText", "boom")
}

func TestUIMessageStreamEncoder_CancelledClosesBlocks(t *testing.T) {
	var buf bytes.Buffer
	enc := NewUIMessageStreamEncoder(&buf)

	_ = enc.Encode(Event{Type: EventTypeLLMStart, Sequence: 1})
	_ = enc.Encode(Event{Type: EventTypeContentDelta, Sequence: 2, Text: "partial"})
	_ = enc.Encode(Event{Type: EventTypeCancelled, Sequence: 3})
	_ = enc.Close()

	lines := extractDataLines(t, &buf)
	assertJSONField(t, lines[1], "type", "text-start")
	assertJSONField(t, lines[2], "type", "text-delta")
	assertJSONField(t, lines[3], "type", "text-end")
	assertJSONField(t, lines[4], "type", "finish")
	assertJSONField(t, lines[4], "finishReason", "other")
}

func TestUIMessageStreamEncoder_Heartbeat(t *testing.T) {
	var buf bytes.Buffer
	enc := NewUIMessageStreamEncoder(&buf)

	if err := enc.Heartbeat(); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	if got := buf.String(); got != ": heartbeat\n\n" {
		t.Fatalf("expected heartbeat comment frame, got %q", got)
	}
}

func assertJSONField(t *testing.T, line, key, expected string) {
	t.Helper()
	if line == "[DONE]" {
		if expected == "[DONE]" {
			return
		}
		t.Fatalf("unexpected [DONE] line looking for %s=%s", key, expected)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid json line %q: %v", line, err)
	}
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q in %s", key, line)
	}
	if got := v.(string); got != expected {
		t.Errorf("%s: expected %q, got %q", key, expected, got)
	}
}

func assertJSONMissingField(t *testing.T, line, key string) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid json line %q: %v", line, err)
	}
	if _, ok := m[key]; ok {
		t.Fatalf("expected key %q to be absent in %s", key, line)
	}
}
