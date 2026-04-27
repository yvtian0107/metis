package bdd

// bdd_test.go — godog BDD test suite entry point for ITSM.
//
// Run BDD tests (requires LLM_TEST_* env vars):
//   LLM_TEST_BASE_URL=... LLM_TEST_API_KEY=... LLM_TEST_MODEL=... go test ./internal/app/itsm/ -run TestBDD -v
//   make test-bdd

import (
	"context"
	"os"
	"strings"
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
			Paths:    bddPaths(),
			Tags:     bddTags(),
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run BDD feature tests")
	}
}

func bddPaths() []string {
	if raw := strings.TrimSpace(os.Getenv("ITSM_BDD_PATHS")); raw != "" {
		parts := strings.Split(raw, ",")
		paths := make([]string, 0, len(parts))
		for _, part := range parts {
			if path := strings.TrimSpace(part); path != "" {
				paths = append(paths, path)
			}
		}
		if len(paths) > 0 {
			return paths
		}
	}
	return []string{"features"}
}

func bddTags() string {
	if tags := strings.TrimSpace(os.Getenv("ITSM_BDD_TAGS")); tags != "" {
		return tags
	}
	return "~@wip"
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
	registerServiceDeskDialogSteps(sc, bc)
	registerSessionIsolationSteps(sc, bc)
	registerKnowledgeRoutingSteps(sc, bc)
	registerSLAAssuranceSteps(sc, bc)
}
