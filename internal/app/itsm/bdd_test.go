package itsm

// bdd_test.go — godog BDD test suite entry point for ITSM.
//
// Run BDD tests (requires LLM_TEST_* env vars):
//   LLM_TEST_BASE_URL=... LLM_TEST_API_KEY=... LLM_TEST_MODEL=... go test ./internal/app/itsm/ -run TestBDD -v
//   make test-bdd

import (
	"context"
	"testing"

	"github.com/cucumber/godog"
)

func TestBDD(t *testing.T) {
	if !hasLLMConfig() {
		t.Skip("BDD tests require LLM: set LLM_TEST_BASE_URL, LLM_TEST_API_KEY, LLM_TEST_MODEL")
	}

	suite := godog.TestSuite{
		Name:                "itsm-bdd",
		ScenarioInitializer: initializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			Tags:     "~@wip",
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run BDD feature tests")
	}
}

func initializeScenario(sc *godog.ScenarioContext) {
	bc := newBDDContext()

	sc.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
		bc.reset()
		return ctx, nil
	})

	registerCommonSteps(sc, bc)
	registerClassicSteps(sc, bc)
	registerSmartSteps(sc, bc)
	registerServerAccessSteps(sc, bc)
	registerDbBackupSteps(sc, bc)
	registerBossSteps(sc, bc)
	registerWithdrawSteps(sc, bc)
	registerParticipantSteps(sc, bc)
	registerDeterministicSteps(sc, bc)
	registerDialogValidationSteps(sc, bc)
	registerDraftRecoverySteps(sc, bc)
	registerCountersignSteps(sc, bc)
	registerRecoverySteps(sc, bc)
	registerE2EDialogSteps(sc, bc)
	registerSessionIsolationSteps(sc, bc)
	registerKnowledgeRoutingSteps(sc, bc)
}
