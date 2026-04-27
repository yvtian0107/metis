package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"metis/internal/llm"
)

var (
	agentExecutionTimeout = 2 * time.Minute
	llmTurnTimeout        = 2 * time.Minute
)

type chatStreamResult struct {
	ch  <-chan llm.StreamEvent
	err error
}

type chatResult struct {
	resp *llm.ChatResponse
	err  error
}

func openChatStreamWithTimeout(ctx context.Context, client llm.Client, req llm.ChatRequest) (<-chan llm.StreamEvent, context.Context, context.CancelFunc, error) {
	turnCtx, cancel := context.WithTimeout(ctx, llmTurnTimeout)
	resultCh := make(chan chatStreamResult, 1)

	go func() {
		ch, err := client.ChatStream(turnCtx, req)
		resultCh <- chatStreamResult{ch: ch, err: err}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			cancel()
			return nil, nil, nil, result.err
		}
		return result.ch, turnCtx, cancel, nil
	case <-turnCtx.Done():
		err := turnCtx.Err()
		cancel()
		return nil, nil, nil, err
	}
}

func chatWithTimeout(ctx context.Context, client llm.Client, req llm.ChatRequest) (*llm.ChatResponse, error) {
	turnCtx, cancel := context.WithTimeout(ctx, llmTurnTimeout)
	defer cancel()

	resultCh := make(chan chatResult, 1)
	go func() {
		resp, err := client.Chat(turnCtx, req)
		resultCh <- chatResult{resp: resp, err: err}
	}()

	select {
	case result := <-resultCh:
		return result.resp, result.err
	case <-turnCtx.Done():
		return nil, turnCtx.Err()
	}
}

func llmCallErrorMessage(phase string, err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("%s timed out after %s", phase, llmTurnTimeout)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Sprintf("%s cancelled", phase)
	}
	return fmt.Sprintf("%s failed: %v", phase, err)
}

func stoppedEvent(err error, timeoutPhase string) Event {
	if errors.Is(err, context.DeadlineExceeded) {
		return Event{Type: EventTypeError, Message: llmCallErrorMessage(timeoutPhase, err)}
	}
	return Event{Type: EventTypeCancelled, Message: err.Error()}
}
