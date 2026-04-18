package itsm

// steps_recovery_test.go — BDD step definitions for smart engine recovery scenarios.

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cucumber/godog"

	"metis/internal/app/itsm/engine"
)

// trackingSubmitter records submitted task payloads for assertions.
type trackingSubmitter struct {
	mu       sync.Mutex
	payloads []json.RawMessage
}

func (ts *trackingSubmitter) SubmitTask(_ string, payload json.RawMessage) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.payloads = append(ts.payloads, payload)
	return nil
}

func (ts *trackingSubmitter) Count() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.payloads)
}

// Compile-time check.
var _ engine.TaskSubmitter = (*trackingSubmitter)(nil)

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
		// Create a smart engine wired to the tracking submitter.
		recoveryEngine := engine.NewSmartEngine(nil, nil, nil, nil, submitter, nil)
		handler := engine.HandleSmartRecovery(bc.db, recoveryEngine)
		return handler(context.Background(), nil)
	})

	sc.Then(`^恢复任务已提交$`, func() error {
		if submitter == nil {
			return fmt.Errorf("submitter not initialized")
		}
		if submitter.Count() == 0 {
			return fmt.Errorf("expected recovery task to be submitted, but none were")
		}
		return nil
	})

	sc.Then(`^恢复任务未提交$`, func() error {
		if submitter == nil {
			return fmt.Errorf("submitter not initialized")
		}
		if submitter.Count() != 0 {
			return fmt.Errorf("expected no recovery tasks, but %d were submitted", submitter.Count())
		}
		return nil
	})
}
