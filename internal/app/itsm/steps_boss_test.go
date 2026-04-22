package itsm

// steps_boss_test.go — step definitions for the Boss serial process BDD scenarios.

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cucumber/godog"
)

// registerBossSteps registers all Boss serial process step definitions.
func registerBossSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已定义高风险变更协同申请协作规范$`, bc.givenBossCollaborationSpec)
	sc.Given(`^已基于协作规范发布高风险变更协同申请服务（智能引擎）$`, bc.givenBossSmartServicePublished)
	sc.Given(`^"([^"]*)" 已创建高风险变更工单，场景为 "([^"]*)"$`, bc.givenBossTicketCreated)
	sc.Given(`^"([^"]*)" 已创建高风险变更工单 "([^"]*)"，场景为 "([^"]*)"$`, bc.givenBossTicketCreatedWithAlias)

	sc.Then(`^工单的表单数据中包含完整的 resource_items 明细表格$`, bc.thenFormDataContainsResourceItems)
	sc.Then(`^工单 "([^"]*)" 的处理记录与工单 "([^"]*)" 完全隔离$`, bc.thenProcessRecordsIsolated)
}

// --- Given steps ---

func (bc *bddContext) givenBossCollaborationSpec() error {
	bc.collaborationSpec = bossCollaborationSpec
	return nil
}

func (bc *bddContext) givenBossSmartServicePublished() error {
	return publishBossSmartService(bc)
}

func (bc *bddContext) givenBossTicketCreated(username, caseKey string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	payload, ok := bossCasePayloads[caseKey]
	if !ok {
		return fmt.Errorf("unknown case key %q, expected one of: requester-1, requester-2", caseKey)
	}

	formJSON, _ := json.Marshal(payload.FormData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("BOSS-%s-%d", caseKey, time.Now().UnixNano()),
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

func (bc *bddContext) givenBossTicketCreatedWithAlias(username, alias, caseKey string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	payload, ok := bossCasePayloads[caseKey]
	if !ok {
		return fmt.Errorf("unknown case key %q", caseKey)
	}

	formJSON, _ := json.Marshal(payload.FormData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("BOSS-%s-%d", alias, time.Now().UnixNano()),
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

// thenFormDataContainsResourceItems asserts that the current ticket's form_data
// contains a complete resource_items array with expected fields.
func (bc *bddContext) thenFormDataContainsResourceItems() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	// Refresh ticket from DB
	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}

	var formData map[string]any
	if err := json.Unmarshal([]byte(bc.ticket.FormData), &formData); err != nil {
		return fmt.Errorf("parse form_data: %w", err)
	}

	itemsRaw, ok := formData["resource_items"]
	if !ok {
		keys := make([]string, 0, len(formData))
		for k := range formData {
			keys = append(keys, k)
		}
		return fmt.Errorf("form_data missing 'resource_items' key, got keys: %v", keys)
	}

	items, ok := itemsRaw.([]any)
	if !ok {
		return fmt.Errorf("resource_items is not an array, type: %T", itemsRaw)
	}

	if len(items) == 0 {
		return fmt.Errorf("resource_items array is empty")
	}

	// Find the original payload to compare
	var expectedItems []map[string]any
	for _, cp := range bossCasePayloads {
		fd := cp.FormData
		if ri, ok := fd["resource_items"].([]map[string]any); ok {
			if len(ri) == len(items) {
				expectedItems = ri
				break
			}
		}
	}

	requiredFields := []string{"system_name", "resource_account", "permission_level", "target_operation"}

	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("resource_items[%d] is not an object, type: %T", i, item)
		}
		for _, field := range requiredFields {
			val, exists := m[field]
			if !exists {
				return fmt.Errorf("resource_items[%d] missing field %q", i, field)
			}
			strVal, ok := val.(string)
			if !ok || strVal == "" {
				return fmt.Errorf("resource_items[%d].%s is empty or non-string", i, field)
			}
			// If we have expected items, verify values match
			if expectedItems != nil && i < len(expectedItems) {
				if expected, ok := expectedItems[i][field].(string); ok && expected != strVal {
					return fmt.Errorf("resource_items[%d].%s = %q, expected %q", i, field, strVal, expected)
				}
			}
		}
	}

	return nil
}

// thenProcessRecordsIsolated asserts that two tickets' TicketAssignment records are completely isolated.
func (bc *bddContext) thenProcessRecordsIsolated(ticketRefA, ticketRefB string) error {
	ticketA, ok := bc.tickets[ticketRefA]
	if !ok {
		return fmt.Errorf("ticket alias %q not found in context", ticketRefA)
	}
	ticketB, ok := bc.tickets[ticketRefB]
	if !ok {
		return fmt.Errorf("ticket alias %q not found in context", ticketRefB)
	}

	// Check A has assignment records
	var countA int64
	bc.db.Model(&TicketAssignment{}).Where("ticket_id = ?", ticketA.ID).Count(&countA)
	if countA == 0 {
		return fmt.Errorf("ticket %q has no assignment records", ticketRefA)
	}

	// Check B has assignment records
	var countB int64
	bc.db.Model(&TicketAssignment{}).Where("ticket_id = ?", ticketB.ID).Count(&countB)
	if countB == 0 {
		return fmt.Errorf("ticket %q has no assignment records", ticketRefB)
	}

	// Verify no cross-contamination via activity IDs
	var activitiesA []TicketActivity
	bc.db.Where("ticket_id = ?", ticketA.ID).Find(&activitiesA)
	activityIDsA := make(map[uint]bool)
	for _, a := range activitiesA {
		activityIDsA[a.ID] = true
	}

	var activitiesB []TicketActivity
	bc.db.Where("ticket_id = ?", ticketB.ID).Find(&activitiesB)
	activityIDsB := make(map[uint]bool)
	for _, a := range activitiesB {
		activityIDsB[a.ID] = true
	}

	// Verify A's assignments only reference A's activities
	var assignmentsA []TicketAssignment
	bc.db.Where("ticket_id = ?", ticketA.ID).Find(&assignmentsA)
	for _, asgn := range assignmentsA {
		if activityIDsB[asgn.ActivityID] {
			return fmt.Errorf("ticket %q's assignment references ticket %q's activity %d", ticketRefA, ticketRefB, asgn.ActivityID)
		}
	}

	// Verify B's assignments only reference B's activities
	var assignmentsB []TicketAssignment
	bc.db.Where("ticket_id = ?", ticketB.ID).Find(&assignmentsB)
	for _, asgn := range assignmentsB {
		if activityIDsA[asgn.ActivityID] {
			return fmt.Errorf("ticket %q's assignment references ticket %q's activity %d", ticketRefB, ticketRefA, asgn.ActivityID)
		}
	}

	return nil
}
