package itsm

import (
	"context"
	"testing"

	"github.com/samber/do/v2"

	"metis/internal/app"
)

type countingDecisionExecutor struct {
	calls *int
}

func (e countingDecisionExecutor) Execute(context.Context, uint, app.AIDecisionRequest) (*app.AIDecisionResponse, error) {
	*e.calls = *e.calls + 1
	return &app.AIDecisionResponse{Content: "ok"}, nil
}

func TestLazyDecisionExecutorDefersProviderUntilExecute(t *testing.T) {
	injector := do.New()
	providerCalls := 0
	executeCalls := 0

	do.Provide(injector, func(do.Injector) (app.AIDecisionExecutor, error) {
		providerCalls++
		return countingDecisionExecutor{calls: &executeCalls}, nil
	})

	executor := newLazyDecisionExecutor(injector)
	if providerCalls != 0 {
		t.Fatalf("expected executor creation not to resolve provider, got %d calls", providerCalls)
	}

	resp, err := executor.Execute(context.Background(), 1, app.AIDecisionRequest{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("unexpected response content: %q", resp.Content)
	}
	if providerCalls != 1 {
		t.Fatalf("expected provider to resolve on execute, got %d calls", providerCalls)
	}
	if executeCalls != 1 {
		t.Fatalf("expected delegated executor to run once, got %d calls", executeCalls)
	}
}
