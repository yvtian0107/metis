package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"metis/internal/scheduler"
)

func TestHandleWaitTimer_NotReady(t *testing.T) {
	// When execute_after is in the future, handler should return ErrNotReady
	future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	payload, _ := json.Marshal(WaitTimerPayload{
		TicketID:     1,
		ActivityID:   1,
		ExecuteAfter: future,
	})

	// We can call the handler directly — it will return ErrNotReady before touching DB
	handler := HandleWaitTimer(nil, nil)
	err := handler(context.Background(), payload)
	if !errors.Is(err, scheduler.ErrNotReady) {
		t.Errorf("expected ErrNotReady for future timer, got: %v", err)
	}
}

func TestHandleWaitTimer_InvalidPayload(t *testing.T) {
	handler := HandleWaitTimer(nil, nil)
	err := handler(context.Background(), json.RawMessage(`{invalid}`))
	if err == nil {
		t.Error("expected error for invalid payload")
	}
}

func TestHandleWaitTimer_InvalidTime(t *testing.T) {
	payload, _ := json.Marshal(WaitTimerPayload{
		TicketID:     1,
		ActivityID:   1,
		ExecuteAfter: "not-a-time",
	})
	handler := HandleWaitTimer(nil, nil)
	err := handler(context.Background(), payload)
	if err == nil {
		t.Error("expected error for invalid time format")
	}
}

func TestHandleBoundaryTimer_NotReady(t *testing.T) {
	future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	payload, _ := json.Marshal(BoundaryTimerPayload{
		TicketID:        1,
		BoundaryTokenID: 1,
		BoundaryNodeID:  "bt1",
		HostTokenID:     2,
		ExecuteAfter:    future,
	})

	handler := HandleBoundaryTimer(nil, nil)
	err := handler(context.Background(), payload)
	if !errors.Is(err, scheduler.ErrNotReady) {
		t.Errorf("expected ErrNotReady for future boundary timer, got: %v", err)
	}
}

func TestHandleBoundaryTimer_InvalidPayload(t *testing.T) {
	handler := HandleBoundaryTimer(nil, nil)
	err := handler(context.Background(), json.RawMessage(`{bad}`))
	if err == nil {
		t.Error("expected error for invalid payload")
	}
}
