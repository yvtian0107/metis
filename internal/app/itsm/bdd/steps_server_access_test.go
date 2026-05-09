package bdd

// steps_server_access_test.go — step definitions for the server access BDD scenarios.

import (
	"encoding/json"
	"fmt"
	. "metis/internal/app/itsm/domain"
	"time"

	"github.com/cucumber/godog"

	"metis/internal/app/itsm/engine"
)

// registerServerAccessSteps registers all server access step definitions.
func registerServerAccessSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已定义生产服务器临时访问申请协作规范$`, bc.givenServerAccessCollaborationSpec)
	sc.Given(`^已基于协作规范发布生产服务器临时访问服务（智能引擎）$`, bc.givenServerAccessSmartServicePublished)
	sc.Given(`^"([^"]*)" 已创建生产服务器访问工单，场景为 "([^"]*)"$`, bc.givenServerAccessTicketCreated)
	sc.Given(`^"([^"]*)" 已创建生产服务器访问工单，表单数据为:$`, bc.givenServerAccessTicketCreatedWithFormData)
	sc.Given(`^生产服务器访问工作流参考图错误地把岗位 "([^"]*)" 标成 "([^"]*)"$`, bc.givenServerAccessWorkflowPositionMislabeled)
	sc.Given(`^生产服务器访问工作流参考图错误地把驳回指向申请人补充表单$`, bc.givenServerAccessWorkflowRejectedReturnsRequesterForm)
	sc.Given(`^生产服务器访问岗位 "([^"]*)" 处理人已停用$`, bc.givenServerAccessPositionInactive)
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
		return fmt.Errorf("unknown case key %q", caseKey)
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
	if err := bc.db.Create(&TicketTimeline{
		TicketID:   ticket.ID,
		OperatorID: user.ID,
		EventType:  "ticket_created",
		Message:    "BDD server access ticket created",
	}).Error; err != nil {
		return fmt.Errorf("create ticket timeline: %w", err)
	}
	bc.ticket = ticket
	return nil
}

func (bc *bddContext) givenServerAccessTicketCreatedWithFormData(username string, doc *godog.DocString) error {
	if doc == nil {
		return fmt.Errorf("missing form data doc string")
	}
	var formData map[string]any
	if err := json.Unmarshal([]byte(doc.Content), &formData); err != nil {
		return fmt.Errorf("parse form data JSON: %w", err)
	}
	return bc.createServerAccessTicket(username, "生产服务器临时访问申请：corner case", formData, bc.service.WorkflowJSON)
}

func (bc *bddContext) createServerAccessTicket(username, title string, formData map[string]any, workflowJSON JSONField) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	formJSON, _ := json.Marshal(formData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("SA-CORNER-%d", time.Now().UnixNano()),
		Title:        title,
		ServiceID:    bc.service.ID,
		EngineType:   "smart",
		Status:       "pending",
		PriorityID:   bc.priority.ID,
		RequesterID:  user.ID,
		FormData:     JSONField(formJSON),
		WorkflowJSON: workflowJSON,
	}
	if err := bc.db.Create(ticket).Error; err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	bc.ticket = ticket
	return nil
}

func (bc *bddContext) givenServerAccessWorkflowPositionMislabeled(fromPosition, toPosition string) error {
	if bc.service == nil {
		return fmt.Errorf("no service in context")
	}
	corrupted, err := corruptWorkflowProcessPosition(json.RawMessage(bc.service.WorkflowJSON), fromPosition, toPosition)
	if err != nil {
		return err
	}
	bc.service.WorkflowJSON = JSONField(corrupted)
	return bc.db.Save(bc.service).Error
}

func (bc *bddContext) givenServerAccessWorkflowRejectedReturnsRequesterForm() error {
	if bc.service == nil {
		return fmt.Errorf("no service in context")
	}
	corrupted, err := corruptVPNWorkflowRejectedTarget(json.RawMessage(bc.service.WorkflowJSON))
	if err != nil {
		return err
	}
	bc.service.WorkflowJSON = JSONField(corrupted)
	return bc.db.Save(bc.service).Error
}

func (bc *bddContext) givenServerAccessPositionInactive(positionCode string) error {
	usernamesByPosition := map[string][]string{
		"ops_admin":      {"ops-operator"},
		"network_admin":  {"network-operator"},
		"security_admin": {"security-operator"},
	}
	usernames, ok := usernamesByPosition[positionCode]
	if !ok {
		return fmt.Errorf("unsupported server access position %q", positionCode)
	}
	for _, username := range usernames {
		if err := bc.deactivateUser(username); err != nil {
			return err
		}
	}
	return nil
}

func corruptWorkflowProcessPosition(raw json.RawMessage, fromPosition, toPosition string) (json.RawMessage, error) {
	var wf vpnWorkflowDoc
	if err := json.Unmarshal(raw, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow_json: %w", err)
	}

	changed := false
	for i := range wf.Nodes {
		if wf.Nodes[i].Type != engine.NodeProcess && wf.Nodes[i].Type != engine.NodeApprove {
			continue
		}
		rawParticipants, ok := wf.Nodes[i].Data["participants"].([]any)
		if !ok {
			continue
		}
		for _, rawParticipant := range rawParticipants {
			participant, ok := rawParticipant.(map[string]any)
			if !ok {
				continue
			}
			if fmt.Sprint(participant["type"]) == "position_department" &&
				fmt.Sprint(participant["position_code"]) == fromPosition {
				participant["position_code"] = toPosition
				changed = true
			}
		}
	}
	if !changed {
		return nil, fmt.Errorf("workflow_json missing process node for position %q", fromPosition)
	}

	corrupted, err := json.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("marshal corrupted workflow_json: %w", err)
	}
	return corrupted, nil
}
