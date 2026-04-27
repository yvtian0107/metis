package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// StreamEncoder encodes internal Event values into a concrete outbound stream format.
type StreamEncoder interface {
	Encode(evt Event) error
	Close() error
}

type heartbeatStreamEncoder interface {
	Heartbeat() error
}

// StreamEncoderFactory creates a new encoder bound to the provided writer.
type StreamEncoderFactory func(w io.Writer) StreamEncoder

// UIMessageStreamEncoder encodes internal Event structures into Vercel AI SDK
// UI Message Stream format (JSON SSE lines). This format is consumed by
// @ai-sdk/react useChat when combined with a custom transport.
type UIMessageStreamEncoder struct {
	mu          sync.Mutex
	w           io.Writer
	messageID   string
	started     bool
	textBlock   blockState
	reasonBlock blockState
}

type blockState struct {
	id      string
	started bool
}

// NewUIMessageStreamEncoder creates an encoder that writes to w.
func NewUIMessageStreamEncoder(w io.Writer) *UIMessageStreamEncoder {
	return &UIMessageStreamEncoder{
		w:         w,
		messageID: "msg-0",
	}
}

// Encode translates a single internal Event into zero or more UI Message Stream
// SSE lines and writes them to the underlying writer.
func (enc *UIMessageStreamEncoder) Encode(evt Event) error {
	enc.mu.Lock()
	defer enc.mu.Unlock()

	switch evt.Type {
	case EventTypeLLMStart:
		if enc.started {
			return nil
		}
		enc.started = true
		enc.messageID = fmt.Sprintf("msg-%d", evt.Sequence)
		return enc.writeLine(map[string]any{
			"type":      "start",
			"messageId": enc.messageID,
		})

	case EventTypeContentDelta:
		if !enc.textBlock.started {
			enc.textBlock.id = fmt.Sprintf("text-%d", evt.Sequence)
			enc.textBlock.started = true
			if err := enc.writeLine(map[string]any{
				"type": "text-start",
				"id":   enc.textBlock.id,
			}); err != nil {
				return err
			}
		}
		return enc.writeLine(map[string]any{
			"type":  "text-delta",
			"id":    enc.textBlock.id,
			"delta": evt.Text,
		})

	case EventTypeThinkingDelta:
		if !enc.reasonBlock.started {
			enc.reasonBlock.id = fmt.Sprintf("reason-%d", evt.Sequence)
			enc.reasonBlock.started = true
			if err := enc.writeLine(map[string]any{
				"type": "reasoning-start",
				"id":   enc.reasonBlock.id,
			}); err != nil {
				return err
			}
		}
		return enc.writeLine(map[string]any{
			"type":  "reasoning-delta",
			"id":    enc.reasonBlock.id,
			"delta": evt.Text,
		})

	case EventTypeThinkingDone:
		if enc.reasonBlock.started {
			enc.reasonBlock.started = false
			return enc.writeLine(map[string]any{
				"type": "reasoning-end",
				"id":   enc.reasonBlock.id,
			})
		}
		return nil

	case EventTypePlan:
		return enc.writeLine(map[string]any{
			"type": "data-plan",
			"data": map[string]any{"steps": evt.Steps},
		})

	case EventTypeStepStart:
		return enc.writeLine(map[string]any{
			"type": "data-step",
			"data": map[string]any{
				"index":       evt.StepIndex,
				"description": evt.Description,
				"state":       "start",
			},
		})

	case EventTypeStepDone:
		return enc.writeLine(map[string]any{
			"type": "data-step",
			"data": map[string]any{
				"index":      evt.StepIndex,
				"state":      "done",
				"durationMs": evt.DurationMs,
			},
		})

	case EventTypeUISurface:
		var data any
		if len(evt.SurfaceData) > 0 {
			_ = json.Unmarshal(evt.SurfaceData, &data)
		}
		if data == nil {
			data = map[string]any{}
		}
		return enc.writeLine(map[string]any{
			"type": "data-ui-surface",
			"id":   evt.SurfaceID,
			"data": map[string]any{
				"surfaceId":   evt.SurfaceID,
				"surfaceType": evt.SurfaceType,
				"payload":     data,
			},
		})

	case EventTypeToolCall:
		if enc.textBlock.started {
			enc.textBlock.started = false
			if err := enc.writeLine(map[string]any{
				"type": "text-end",
				"id":   enc.textBlock.id,
			}); err != nil {
				return err
			}
		}
		var input any
		if len(evt.ToolArgs) > 0 {
			_ = json.Unmarshal(evt.ToolArgs, &input)
		}
		if input == nil {
			input = map[string]any{}
		}
		return enc.writeLine(map[string]any{
			"type":       "tool-input-available",
			"toolCallId": evt.ToolCallID,
			"toolName":   evt.ToolName,
			"input":      input,
		})

	case EventTypeToolResult:
		return enc.writeLine(map[string]any{
			"type":       "tool-output-available",
			"toolCallId": evt.ToolCallID,
			"output":     evt.ToolOutput,
		})

	case EventTypeDone:
		if enc.textBlock.started {
			enc.textBlock.started = false
			if err := enc.writeLine(map[string]any{
				"type": "text-end",
				"id":   enc.textBlock.id,
			}); err != nil {
				return err
			}
		}
		if enc.reasonBlock.started {
			enc.reasonBlock.started = false
			if err := enc.writeLine(map[string]any{
				"type": "reasoning-end",
				"id":   enc.reasonBlock.id,
			}); err != nil {
				return err
			}
		}
		if evt.InputTokens > 0 || evt.OutputTokens > 0 {
			if err := enc.writeLine(map[string]any{
				"type": "message-metadata",
				"messageMetadata": map[string]any{
					"usage": map[string]any{
						"promptTokens":     evt.InputTokens,
						"completionTokens": evt.OutputTokens,
					},
				},
			}); err != nil {
				return err
			}
		}
		return enc.writeLine(map[string]any{
			"type":         "finish",
			"finishReason": "stop",
		})

	case EventTypeError:
		return enc.writeLine(map[string]any{
			"type":      "error",
			"errorText": evt.Message,
		})

	case EventTypeCancelled:
		if enc.textBlock.started {
			enc.textBlock.started = false
			_ = enc.writeLine(map[string]any{
				"type": "text-end",
				"id":   enc.textBlock.id,
			})
		}
		if enc.reasonBlock.started {
			enc.reasonBlock.started = false
			_ = enc.writeLine(map[string]any{
				"type": "reasoning-end",
				"id":   enc.reasonBlock.id,
			})
		}
		return enc.writeLine(map[string]any{
			"type":         "finish",
			"finishReason": "other",
		})
	}

	return nil
}

// Close writes the mandatory [DONE] marker and any pending block ends.
func (enc *UIMessageStreamEncoder) Close() error {
	enc.mu.Lock()
	defer enc.mu.Unlock()

	if enc.textBlock.started {
		enc.textBlock.started = false
		_ = enc.writeLine(map[string]any{
			"type": "text-end",
			"id":   enc.textBlock.id,
		})
	}
	if enc.reasonBlock.started {
		enc.reasonBlock.started = false
		_ = enc.writeLine(map[string]any{
			"type": "reasoning-end",
			"id":   enc.reasonBlock.id,
		})
	}

	_, err := fmt.Fprintf(enc.w, "data: [DONE]\n\n")
	return err
}

// Heartbeat writes an SSE comment frame. UI Message Stream clients ignore it,
// while proxies and browsers still see activity on otherwise idle streams.
func (enc *UIMessageStreamEncoder) Heartbeat() error {
	enc.mu.Lock()
	defer enc.mu.Unlock()

	_, err := fmt.Fprintf(enc.w, ": heartbeat\n\n")
	return err
}

func (enc *UIMessageStreamEncoder) writeLine(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(enc.w, "data: %s\n\n", data)
	return err
}
