package bdd

// steps_vpn_participant_test.go — step definitions for VPN participant validation BDD scenarios.

import (
	"fmt"
	. "metis/internal/app/itsm/domain"
	"time"

	"github.com/cucumber/godog"

	"metis/internal/app/itsm/engine"
)

// testConfigProvider implements engine.EngineConfigProvider for BDD tests.
type testConfigProvider struct {
	fallbackAssigneeID uint
	decisionMode       string
}

func (p *testConfigProvider) FallbackAssigneeID() uint {
	return p.fallbackAssigneeID
}

func (p *testConfigProvider) DecisionMode() string {
	if p.decisionMode != "" {
		return p.decisionMode
	}
	return "ai_only"
}

func (p *testConfigProvider) DecisionAgentID() uint {
	return 0
}

func (p *testConfigProvider) AuditLevel() string {
	return "full"
}

func (p *testConfigProvider) SLACriticalThresholdSeconds() int {
	return 1800
}

func (p *testConfigProvider) SLAWarningThresholdSeconds() int {
	return 3600
}

func (p *testConfigProvider) SimilarHistoryLimit() int {
	return 5
}

func (p *testConfigProvider) ParallelConvergenceTimeout() time.Duration { return 72 * time.Hour }

var _ engine.EngineConfigProvider = (*testConfigProvider)(nil)

// registerParticipantSteps registers participant validation step definitions.
func registerParticipantSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^引擎已配置兜底处理人为 "([^"]*)"$`, bc.givenFallbackAssignee)
	sc.When(`^引擎执行无参与者的处理决策$`, bc.whenExecutePlanWithoutParticipant)
	sc.Then(`^工单分配人为兜底处理人$`, bc.thenAssigneeIsFallback)
	sc.Then(`^时间线包含参与者兜底事件$`, bc.thenTimelineContainsFallbackEvent)
}

// --- Given steps ---

func (bc *bddContext) givenFallbackAssignee(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	// Rebuild SmartEngine with a config provider that returns this user as fallback.
	bc.fallbackUserID = user.ID
	configProvider := &testConfigProvider{fallbackAssigneeID: user.ID}

	executor := &testDecisionExecutor{db: bc.db, llmCfg: bc.llmCfg, recordToolCall: bc.recordToolCall}
	userProvider := &testUserProvider{db: bc.db}
	orgSvc := &testOrgService{db: bc.db}
	resolver := engine.NewParticipantResolver(orgSvc)

	bc.smartEngine = engine.NewSmartEngine(executor, nil, userProvider, resolver, &noopSubmitter{}, configProvider)
	return nil
}

// --- When steps ---

// whenExecutePlanWithoutParticipant directly calls ExecuteDecisionPlan with a
// crafted DecisionPlan that has no participant_id, bypassing the LLM to test
// the fallback assignment logic deterministically.
func (bc *bddContext) whenExecutePlanWithoutParticipant() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	// Set ticket to in_progress (as Start would do).
	if err := bc.db.Model(&Ticket{}).Where("id = ?", bc.ticket.ID).
		Update("status", "in_progress").Error; err != nil {
		return fmt.Errorf("update ticket status: %w", err)
	}

	plan := &engine.DecisionPlan{
		NextStepType: "process",
		Activities: []engine.DecisionActivity{
			{
				Type:         "process",
				Instructions: "处理 VPN 开通申请",
				// ParticipantID intentionally nil — triggers fallback
			},
		},
		Reasoning:  "测试兜底场景：无参与者的处理决策",
		Confidence: 0.9,
	}

	if err := bc.smartEngine.ExecuteDecisionPlan(bc.db, bc.ticket.ID, plan); err != nil {
		bc.lastErr = err
		return fmt.Errorf("execute decision plan: %w", err)
	}

	// Refresh ticket.
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

// --- Then steps ---

func (bc *bddContext) thenAssigneeIsFallback() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	if bc.fallbackUserID == 0 {
		return fmt.Errorf("fallback user ID not set")
	}

	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}

	if bc.ticket.AssigneeID == nil || *bc.ticket.AssigneeID != bc.fallbackUserID {
		actual := uint(0)
		if bc.ticket.AssigneeID != nil {
			actual = *bc.ticket.AssigneeID
		}
		return fmt.Errorf("expected ticket assignee_id=%d (fallback), got %d", bc.fallbackUserID, actual)
	}
	return nil
}

func (bc *bddContext) thenTimelineContainsFallbackEvent() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	var count int64
	if err := bc.db.Model(&TicketTimeline{}).
		Where("ticket_id = ? AND event_type = ?", bc.ticket.ID, "participant_fallback").
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no participant_fallback event found in timeline for ticket %d", bc.ticket.ID)
	}
	return nil
}
