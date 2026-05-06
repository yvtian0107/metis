package itsm

import (
	"context"

	"github.com/samber/do/v2"

	"metis/internal/app"
)

type lazyDecisionExecutor struct {
	injector do.Injector
}

func newLazyDecisionExecutor(injector do.Injector) app.AIDecisionExecutor {
	return lazyDecisionExecutor{injector: injector}
}

func (e lazyDecisionExecutor) Execute(ctx context.Context, agentID uint, req app.AIDecisionRequest) (*app.AIDecisionResponse, error) {
	executor, err := do.InvokeAs[app.AIDecisionExecutor](e.injector)
	if err != nil {
		return nil, err
	}
	return executor.Execute(ctx, agentID, req)
}
