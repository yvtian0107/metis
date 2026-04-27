package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"metis/internal/app"
)

// ToolHandlerRegistry is the interface each App implements to register its tool handlers.
// GeneralToolRegistry and ITSM tools.Registry both naturally satisfy this.
type ToolHandlerRegistry interface {
	HasTool(name string) bool
	Execute(ctx context.Context, toolName string, userID uint, args json.RawMessage) (json.RawMessage, error)
}

// CompositeToolExecutor dispatches tool calls to the correct handler registry.
type CompositeToolExecutor struct {
	registries []ToolHandlerRegistry
	sessionID  uint
	userID     uint
}

// NewCompositeToolExecutor creates a tool executor backed by multiple registries.
func NewCompositeToolExecutor(registries []ToolHandlerRegistry, sessionID, userID uint) *CompositeToolExecutor {
	return &CompositeToolExecutor{
		registries: registries,
		sessionID:  sessionID,
		userID:     userID,
	}
}

// ExecuteTool implements ToolExecutor.
func (e *CompositeToolExecutor) ExecuteTool(ctx context.Context, call ToolCall) (ToolResult, error) {
	start := time.Now()

	// Inject session ID into context for stateful tools (e.g. ITSM service desk).
	ctx = context.WithValue(ctx, app.SessionIDKey, e.sessionID)

	for _, reg := range e.registries {
		if reg.HasTool(call.Name) {
			raw, err := reg.Execute(ctx, call.Name, e.userID, call.Args)
			dur := int(time.Since(start).Milliseconds())
			if err != nil {
				return ToolResult{
					ID:         call.ID,
					Output:     err.Error(),
					IsError:    true,
					DurationMs: dur,
				}, nil
			}
			return ToolResult{
				ID:         call.ID,
				Output:     string(raw),
				IsError:    false,
				DurationMs: dur,
			}, nil
		}
	}

	return ToolResult{
		ID:      call.ID,
		Output:  fmt.Sprintf("unknown tool: %s", call.Name),
		IsError: true,
	}, nil
}
