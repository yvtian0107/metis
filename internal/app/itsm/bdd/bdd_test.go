package bdd

// bdd_test.go — godog BDD test suite entry points for ITSM.
//
// Test levels:
//   make test-bdd          -> domain/service BDD, no LLM requirement
//   make test-bdd-api      -> API BDD with httptest + Actor client
//   make test-bdd-agentic  -> true LLM Agentic BDD, requires .env.test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cucumber/godog"

	bddsteps "metis/internal/app/itsm/bdd/steps"
	"metis/internal/app/itsm/bdd/support"
)

func TestBDD(t *testing.T) {
	runGodogSuite(t, "itsm-bdd-domain", initializeScenario, bddPaths("features/domain/vpn_smart_engine_deterministic.feature", "features/domain/smart_engine_recovery.feature"), bddTags("~@wip && ~@api && ~@agentic && ~@llm"))
}

func TestBDDAPI(t *testing.T) {
	runGodogSuite(t, "itsm-bdd-api", initializeAPIScenario, bddPaths("features/api"), bddTags("~@wip"))
}

func TestBDDAgentic(t *testing.T) {
	if !hasLLMConfig() {
		t.Skip("Agentic BDD tests require LLM: set LLM_TEST_BASE_URL, LLM_TEST_API_KEY, LLM_TEST_MODEL")
	}
	runGodogSuite(t, "itsm-bdd-agentic", initializeScenario, bddPaths("features/agentic"), bddTags("~@wip && ~@deterministic"))
}

func TestBDDAgenticDeterministic(t *testing.T) {
	runGodogSuite(t, "itsm-bdd-agentic-deterministic", initializeScenario, bddPaths("features/agentic"), bddTags("@deterministic && ~@wip"))
}

func runGodogSuite(t *testing.T, name string, initializer func(*godog.ScenarioContext), paths []string, tags string) {
	t.Helper()
	suite := godog.TestSuite{
		Name:                name,
		ScenarioInitializer: initializer,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    paths,
			Tags:     tags,
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run BDD feature tests")
	}
}

func bddPaths(defaultPaths ...string) []string {
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
	return defaultPaths
}

func bddTags(defaultTags string) string {
	if tags := strings.TrimSpace(os.Getenv("ITSM_BDD_TAGS")); tags != "" {
		return tags
	}
	return defaultTags
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
	registerServerAccessExtendedSteps(sc, bc)
	registerServerAccessDialogSteps(sc, bc)
	registerDbBackupSteps(sc, bc)
	registerBossSteps(sc, bc)
	registerBossDialogSteps(sc, bc)
	registerBossExtraSteps(sc, bc)
	registerWithdrawSteps(sc, bc)
	registerParticipantSteps(sc, bc)
	registerDeterministicSteps(sc, bc)
	registerDialogValidationSteps(sc, bc)
	registerDraftRecoverySteps(sc, bc)
	registerCountersignSteps(sc, bc)
	registerParallelApprovalSteps(sc, bc)
	registerRecoverySteps(sc, bc)
	registerServiceDeskDialogSteps(sc, bc)
	registerSessionIsolationSteps(sc, bc)
	registerKnowledgeRoutingSteps(sc, bc)
	registerSLAAssuranceSteps(sc, bc)
	registerAgenticQualitySteps(sc, bc)
	registerParallelApprovalNewSteps(sc, bc)
}

func initializeAPIScenario(sc *godog.ScenarioContext) {
	apiCtx := support.NewContext()
	sc.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
		return ctx, apiCtx.Reset()
	})
	bddsteps.RegisterAPISteps(sc, apiCtx)
}
