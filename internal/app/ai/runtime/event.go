package runtime

import "encoding/json"

// Event types emitted by executors
const (
	EventTypeLLMStart      = "llm_start"
	EventTypeContentDelta  = "content_delta"
	EventTypeToolCall      = "tool_call"
	EventTypeToolResult    = "tool_result"
	EventTypePlan          = "plan"
	EventTypeStepStart     = "step_start"
	EventTypeThinkingDelta = "thinking_delta"
	EventTypeThinkingDone  = "thinking_done"
	EventTypeStepDone      = "step_done"
	EventTypeDone          = "done"
	EventTypeCancelled     = "cancelled"
	EventTypeError         = "error"
	EventTypeMemoryUpdate  = "memory_update"
	EventTypeUISurface     = "ui_surface"
)

// Event is the unified event emitted by all executors.
type Event struct {
	Type     string `json:"type"`
	Sequence int    `json:"sequence"`

	// llm_start
	Turn  int    `json:"turn,omitempty"`
	Model string `json:"model,omitempty"`

	// content_delta / thinking_delta
	Text string `json:"text,omitempty"`

	// tool_call
	ToolCallID string          `json:"id,omitempty"`
	ToolName   string          `json:"name,omitempty"`
	ToolArgs   json.RawMessage `json:"args,omitempty"`

	// tool_result
	ToolOutput  string `json:"output,omitempty"`
	DurationMs  int    `json:"duration_ms,omitempty"`
	ToolIsError bool   `json:"is_error,omitempty"`

	// plan
	Steps []PlanStep `json:"steps,omitempty"`

	// step_start / step_done
	StepIndex   int    `json:"step_index,omitempty"`
	Description string `json:"description,omitempty"`

	// done
	TotalTurns   int `json:"total_turns,omitempty"`
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`

	// error / cancelled
	Message string `json:"message,omitempty"`

	// memory_update
	MemoryKey     string `json:"memory_key,omitempty"`
	MemoryContent string `json:"memory_content,omitempty"`

	// ui_surface
	SurfaceID   string          `json:"surface_id,omitempty"`
	SurfaceType string          `json:"surface_type,omitempty"`
	SurfaceData json.RawMessage `json:"surface_data,omitempty"`
}

// PlanStep represents a step in a Plan & Execute plan.
type PlanStep struct {
	Index       int    `json:"index"`
	Description string `json:"description"`
}
