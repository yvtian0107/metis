package bdd

// steps_recovery_test.go — BDD step definitions for smart engine recovery scenarios.

import (
	"context"
	"encoding/json"
	"fmt"
	. "metis/internal/app/itsm/domain"
	"sync"
	"time"

	"github.com/cucumber/godog"

	"metis/internal/app"
	"metis/internal/app/itsm/engine"
)

// trackingSubmitter records submitted task payloads for assertions.
type trackingSubmitter struct {
	mu       sync.Mutex
	names    []string
	payloads []json.RawMessage
}

func (ts *trackingSubmitter) SubmitTask(name string, payload json.RawMessage) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.names = append(ts.names, name)
	ts.payloads = append(ts.payloads, payload)
	return nil
}

func (ts *trackingSubmitter) Count() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.payloads)
}

func (ts *trackingSubmitter) LastName() string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if len(ts.names) == 0 {
		return ""
	}
	return ts.names[len(ts.names)-1]
}

// Compile-time check.
var _ engine.TaskSubmitter = (*trackingSubmitter)(nil)

type recoveryDecisionExecutor struct{}

func (recoveryDecisionExecutor) Execute(context.Context, uint, app.AIDecisionRequest) (*app.AIDecisionResponse, error) {
	return &app.AIDecisionResponse{
		Content: `{
			"next_step_type": "complete",
			"execution_mode": "single",
			"activities": [],
			"reasoning": "恢复扫描确认孤儿决策循环可结束",
			"confidence": 0.95
		}`,
		Turns: 1,
	}, nil
}

var _ app.AIDecisionExecutor = recoveryDecisionExecutor{}

func registerRecoverySteps(sc *godog.ScenarioContext, bc *bddContext) {
	var submitter *trackingSubmitter

	sc.Given(`^存在一个状态为 "([^"]*)" 的智能引擎票据且无活跃活动$`, func(status string) error {
		// Ensure the ai_disabled_reason column exists for HandleSmartRecovery queries.
		bc.db.Exec("ALTER TABLE itsm_tickets ADD COLUMN ai_disabled_reason TEXT DEFAULT ''")

		catalog := &ServiceCatalog{Name: "Recovery Test", Code: "recovery-test"}
		if err := bc.db.Create(catalog).Error; err != nil {
			return err
		}
		svc := &ServiceDefinition{
			Name:       "Recovery Service",
			Code:       "recovery-svc",
			CatalogID:  catalog.ID,
			EngineType: "smart",
			AgentID:    uintPtr(1),
		}
		if err := bc.db.Create(svc).Error; err != nil {
			return err
		}
		bc.service = svc

		ticket := &Ticket{
			Code:       "TK-RECOVERY-001",
			Title:      "Recovery Test Ticket",
			Status:     status,
			EngineType: "smart",
			ServiceID:  svc.ID,
		}
		if err := bc.db.Create(ticket).Error; err != nil {
			return err
		}
		bc.ticket = ticket
		return nil
	})

	sc.Given(`^存在一个状态为 "([^"]*)" 的智能引擎票据且有活跃活动$`, func(status string) error {
		// Ensure the ai_disabled_reason column exists for HandleSmartRecovery queries.
		bc.db.Exec("ALTER TABLE itsm_tickets ADD COLUMN ai_disabled_reason TEXT DEFAULT ''")

		catalog := &ServiceCatalog{Name: "Recovery Test2", Code: "recovery-test2"}
		if err := bc.db.Create(catalog).Error; err != nil {
			return err
		}
		svc := &ServiceDefinition{
			Name:       "Recovery Service 2",
			Code:       "recovery-svc2",
			CatalogID:  catalog.ID,
			EngineType: "smart",
			AgentID:    uintPtr(1),
		}
		if err := bc.db.Create(svc).Error; err != nil {
			return err
		}
		bc.service = svc

		ticket := &Ticket{
			Code:       "TK-RECOVERY-002",
			Title:      "Recovery Test Ticket 2",
			Status:     status,
			EngineType: "smart",
			ServiceID:  svc.ID,
		}
		if err := bc.db.Create(ticket).Error; err != nil {
			return err
		}
		bc.ticket = ticket

		// Add an active activity so recovery skips this ticket.
		activity := &TicketActivity{
			TicketID:     ticket.ID,
			ActivityType: "process",
			Name:         "Active Activity",
			Status:       "pending",
		}
		return bc.db.Create(activity).Error
	})

	sc.When(`^执行智能引擎恢复扫描$`, func() error {
		submitter = &trackingSubmitter{}
		recoveryEngine := engine.NewSmartEngine(recoveryDecisionExecutor{}, nil, nil, nil, submitter, &bddConfigProvider{bc: bc})
		recoveryEngine.SetDB(bc.db)
		handler := engine.HandleSmartRecovery(bc.db, recoveryEngine)
		return handler(context.Background(), nil)
	})

	sc.Then(`^恢复分发已触发智能决策$`, func() error {
		if submitter == nil {
			return fmt.Errorf("submitter not initialized")
		}
		if submitter.Count() != 1 || submitter.LastName() != "itsm-smart-progress" {
			return fmt.Errorf("expected recovery to enqueue one smart-progress task, got count=%d last=%q", submitter.Count(), submitter.LastName())
		}
		return nil
	})

	sc.Then(`^恢复分发未触发智能决策$`, func() error {
		if submitter == nil {
			return fmt.Errorf("submitter not initialized")
		}
		if submitter.Count() != 0 {
			return fmt.Errorf("expected no recovery tasks, but %d were submitted", submitter.Count())
		}
		time.Sleep(100 * time.Millisecond)
		count, err := bc.timelineEventCount("workflow_completed")
		if err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("expected no recovery decision event, got %d", count)
		}
		return nil
	})
}

func (bc *bddContext) waitForTimelineEvent(eventType string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastCount int64
	for {
		count, err := bc.timelineEventCount(eventType)
		if err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		lastCount = count
		if time.Now().After(deadline) {
			return fmt.Errorf("expected timeline event %q, got %d", eventType, lastCount)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (bc *bddContext) timelineEventCount(eventType string) (int64, error) {
	if bc.ticket == nil {
		return 0, fmt.Errorf("no ticket in context")
	}
	var count int64
	err := bc.db.Model(&TicketTimeline{}).
		Where("ticket_id = ? AND event_type = ?", bc.ticket.ID, eventType).
		Count(&count).Error
	return count, err
}

func uintPtr(v uint) *uint {
	return &v
}
