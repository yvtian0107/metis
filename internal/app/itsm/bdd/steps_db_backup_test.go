package bdd

// steps_db_backup_test.go — step definitions for the DB backup whitelist BDD scenarios.

import (
	"encoding/json"
	"fmt"
	. "metis/internal/app/itsm/domain"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// registerDbBackupSteps registers all DB backup whitelist step definitions.
func registerDbBackupSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已定义数据库备份白名单临时放行协作规范$`, bc.givenDbBackupCollaborationSpec)
	sc.Given(`^已基于协作规范发布数据库备份白名单放行服务（智能引擎）$`, bc.givenDbBackupSmartServicePublished)
	sc.Given(`^"([^"]*)" 已创建数据库备份白名单放行工单，场景为 "([^"]*)"$`, bc.givenDbBackupTicketCreated)
	sc.Given(`^"([^"]*)" 已创建数据库备份白名单放行工单 "([^"]*)"，场景为 "([^"]*)"$`, bc.givenDbBackupTicketCreatedWithAlias)

	sc.Then(`^预检动作已为当前工单触发$`, bc.thenPrecheckActionTriggered)
	sc.Then(`^放行动作已为当前工单触发$`, bc.thenApplyActionTriggered)
	sc.Then(`^放行动作未为当前工单触发$`, bc.thenApplyActionNotTriggered)
	sc.Then(`^工单 "([^"]*)" 的动作记录与工单 "([^"]*)" 完全隔离$`, bc.thenActionRecordsIsolated)
}

// --- Given steps ---

func (bc *bddContext) givenDbBackupCollaborationSpec() error {
	bc.collaborationSpec = dbBackupCollaborationSpec
	return nil
}

func (bc *bddContext) givenDbBackupSmartServicePublished() error {
	return publishDbBackupSmartService(bc)
}

func (bc *bddContext) givenDbBackupTicketCreated(username, caseKey string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	payload, ok := dbBackupCasePayloads[caseKey]
	if !ok {
		return fmt.Errorf("unknown case key %q, expected one of: requester-1, requester-2", caseKey)
	}

	formJSON, _ := json.Marshal(payload.FormData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("DB-%s-%d", caseKey, time.Now().UnixNano()),
		Title:        payload.Summary,
		ServiceID:    bc.service.ID,
		EngineType:   "smart",
		Status:       "pending",
		PriorityID:   bc.priority.ID,
		RequesterID:  user.ID,
		FormData:     JSONField(formJSON),
		WorkflowJSON: bc.service.WorkflowJSON,
	}
	if err := bc.db.Create(ticket).Error; err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	bc.ticket = ticket
	return nil
}

func (bc *bddContext) givenDbBackupTicketCreatedWithAlias(username, alias, caseKey string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	payload, ok := dbBackupCasePayloads[caseKey]
	if !ok {
		return fmt.Errorf("unknown case key %q", caseKey)
	}

	formJSON, _ := json.Marshal(payload.FormData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("DB-%s-%d", alias, time.Now().UnixNano()),
		Title:        payload.Summary,
		ServiceID:    bc.service.ID,
		EngineType:   "smart",
		Status:       "pending",
		PriorityID:   bc.priority.ID,
		RequesterID:  user.ID,
		FormData:     JSONField(formJSON),
		WorkflowJSON: bc.service.WorkflowJSON,
	}
	if err := bc.db.Create(ticket).Error; err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	bc.tickets[alias] = ticket
	bc.ticket = ticket // also set as current ticket
	return nil
}

// --- Then steps ---

func (bc *bddContext) thenPrecheckActionTriggered() error {
	return bc.assertActionTriggered("db_backup_whitelist_precheck", "/precheck")
}

func (bc *bddContext) thenApplyActionTriggered() error {
	return bc.assertActionTriggered("db_backup_whitelist_apply", "/apply")
}

func (bc *bddContext) thenApplyActionNotTriggered() error {
	return bc.assertActionNotTriggered("db_backup_whitelist_apply", "/apply")
}

func (bc *bddContext) assertActionTriggered(actionCode, receiverPath string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	action, ok := bc.serviceActions[actionCode]
	if !ok {
		return fmt.Errorf("service action %q not found in context", actionCode)
	}

	// Check TicketActionExecution record exists with status=success.
	var count int64
	if err := bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ? AND service_action_id = ? AND status = ?", bc.ticket.ID, action.ID, "success").
		Count(&count).Error; err != nil {
		return fmt.Errorf("query action execution: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no successful execution found for action %q (id=%d) on ticket %d", actionCode, action.ID, bc.ticket.ID)
	}

	// Check LocalActionReceiver received the request.
	if bc.actionReceiver == nil {
		return fmt.Errorf("action receiver not initialized")
	}
	records := bc.actionReceiver.RecordsByPath(receiverPath)
	// Find records matching this ticket's code in the body.
	var matched bool
	for _, rec := range records {
		if strings.Contains(rec.Body, bc.ticket.Code) {
			matched = true
			break
		}
	}
	if !matched {
		return fmt.Errorf("action receiver path %q has no request matching ticket code %q (total records: %d)",
			receiverPath, bc.ticket.Code, len(records))
	}

	return nil
}

func (bc *bddContext) assertActionNotTriggered(actionCode, receiverPath string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	action, ok := bc.serviceActions[actionCode]
	if !ok {
		return fmt.Errorf("service action %q not found in context", actionCode)
	}

	// Check no TicketActionExecution record exists.
	var count int64
	if err := bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ? AND service_action_id = ?", bc.ticket.ID, action.ID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("query action execution: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("found %d execution records for action %q on ticket %d, expected 0", count, actionCode, bc.ticket.ID)
	}

	// Check LocalActionReceiver has no request matching this ticket.
	if bc.actionReceiver != nil {
		records := bc.actionReceiver.RecordsByPath(receiverPath)
		for _, rec := range records {
			if strings.Contains(rec.Body, bc.ticket.Code) {
				return fmt.Errorf("action receiver path %q has request matching ticket code %q, expected none",
					receiverPath, bc.ticket.Code)
			}
		}
	}

	return nil
}

func (bc *bddContext) thenActionRecordsIsolated(ticketRefA, ticketRefB string) error {
	ticketA, ok := bc.tickets[ticketRefA]
	if !ok {
		return fmt.Errorf("ticket alias %q not found in context", ticketRefA)
	}
	ticketB, ok := bc.tickets[ticketRefB]
	if !ok {
		return fmt.Errorf("ticket alias %q not found in context", ticketRefB)
	}

	// Check A's execution records don't contain B's ticket_id.
	var countAinB int64
	bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ?", ticketA.ID).
		Count(&countAinB)
	if countAinB == 0 {
		return fmt.Errorf("ticket %q has no action execution records", ticketRefA)
	}

	var countBinA int64
	bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ?", ticketB.ID).
		Count(&countBinA)
	if countBinA == 0 {
		return fmt.Errorf("ticket %q has no action execution records", ticketRefB)
	}

	// Verify no cross-contamination: A's records should only have A's ticket_id.
	var crossCount int64
	bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ? AND ticket_id = ?", ticketA.ID, ticketB.ID).
		Count(&crossCount)
	// This query always returns 0 since ticketA.ID != ticketB.ID (unless they're the same ticket).

	// Check receiver records: A's code should not appear in B's execution payloads and vice versa.
	var execsA []TicketActionExecution
	bc.db.Where("ticket_id = ?", ticketA.ID).Find(&execsA)
	for _, e := range execsA {
		if strings.Contains(string(e.RequestPayload), ticketB.Code) {
			return fmt.Errorf("ticket %q's action execution contains ticket %q's code in request payload",
				ticketRefA, ticketRefB)
		}
	}

	var execsB []TicketActionExecution
	bc.db.Where("ticket_id = ?", ticketB.ID).Find(&execsB)
	for _, e := range execsB {
		if strings.Contains(string(e.RequestPayload), ticketA.Code) {
			return fmt.Errorf("ticket %q's action execution contains ticket %q's code in request payload",
				ticketRefB, ticketRefA)
		}
	}

	return nil
}
