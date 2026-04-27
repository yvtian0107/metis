package bdd

// steps_server_access_test.go — step definitions for the server access BDD scenarios.

import (
	"encoding/json"
	"fmt"
	. "metis/internal/app/itsm/domain"
	"time"

	"github.com/cucumber/godog"
)

// registerServerAccessSteps registers all server access step definitions.
func registerServerAccessSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已定义生产服务器临时访问申请协作规范$`, bc.givenServerAccessCollaborationSpec)
	sc.Given(`^已基于协作规范发布生产服务器临时访问服务（智能引擎）$`, bc.givenServerAccessSmartServicePublished)
	sc.Given(`^"([^"]*)" 已创建生产服务器访问工单，场景为 "([^"]*)"$`, bc.givenServerAccessTicketCreated)
}

// givenServerAccessCollaborationSpec stores the server access collaboration spec in bddContext.
func (bc *bddContext) givenServerAccessCollaborationSpec() error {
	bc.collaborationSpec = serverAccessCollaborationSpec
	return nil
}

// givenServerAccessSmartServicePublished generates a server access workflow via LLM
// and publishes a smart service definition.
func (bc *bddContext) givenServerAccessSmartServicePublished() error {
	return publishServerAccessSmartService(bc)
}

// givenServerAccessTicketCreated creates a ticket with form data from the specified case payload.
func (bc *bddContext) givenServerAccessTicketCreated(username, caseKey string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	payload, ok := serverAccessCasePayloads[caseKey]
	if !ok {
		return fmt.Errorf("unknown case key %q, expected one of: ops, network, security, boundary_security", caseKey)
	}

	formJSON, _ := json.Marshal(payload.FormData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("SA-%s-%d", caseKey, time.Now().UnixNano()),
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
