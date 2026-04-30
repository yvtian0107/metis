package engine

import (
	"strconv"
	"strings"
	"testing"

	"metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/testutil"
)

func TestSmartEngineLoadServiceForTicket_UsesBoundRuntimeVersionSnapshot(t *testing.T) {
	db := testutil.NewTestDB(t)
	service := testutil.SeedSmartSubmissionService(t, db)
	version := domain.ServiceDefinitionVersion{
		ServiceID:         service.ID,
		Version:           1,
		ContentHash:       "snapshot-hash",
		EngineType:        "smart",
		CollaborationSpec: "snapshot collaboration spec",
		AgentID:           service.AgentID,
		AgentConfig:       domain.JSONField(`{"temperature":0.1}`),
		KnowledgeBaseIDs:  domain.JSONField(`[11,22]`),
		WorkflowJSON:      domain.JSONField(`{"nodes":[]}`),
		ActionsJSON:       domain.JSONField(`[]`),
		IntakeFormSchema:  domain.JSONField(`{"fields":[]}`),
	}
	if err := db.Create(&version).Error; err != nil {
		t.Fatalf("create service version: %v", err)
	}
	if err := db.Model(&domain.ServiceDefinition{}).Where("id = ?", service.ID).
		Update("collaboration_spec", "mutated live collaboration spec").Error; err != nil {
		t.Fatalf("mutate service: %v", err)
	}
	ticket := domain.Ticket{
		Code:             "TICK-SNAPSHOT-LOAD",
		Title:            "snapshot load",
		ServiceID:        service.ID,
		ServiceVersionID: &version.ID,
		EngineType:       "smart",
		Status:           TicketStatusDecisioning,
		PriorityID:       1,
		RequesterID:      1,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	engine := NewSmartEngine(nil, nil, nil, nil, nil, nil)
	loaded, err := engine.loadServiceForTicket(db, ticket.ID)
	if err != nil {
		t.Fatalf("load service for ticket: %v", err)
	}
	if loaded.CollaborationSpec != version.CollaborationSpec {
		t.Fatalf("expected snapshot spec %q, got %q", version.CollaborationSpec, loaded.CollaborationSpec)
	}
	if loaded.RuntimeVersionID == nil || *loaded.RuntimeVersionID != version.ID {
		t.Fatalf("expected runtime version id %d, got %v", version.ID, loaded.RuntimeVersionID)
	}
}

func TestDecisionDataStore_ListActionsUsesTicketRuntimeVersionSnapshot(t *testing.T) {
	db := testutil.NewTestDB(t)
	service := testutil.SeedSmartSubmissionService(t, db)
	liveAction := domain.ServiceAction{
		Name:       "Live mutated action",
		Code:       "notify",
		ActionType: "http",
		ConfigJSON: domain.JSONField(`{"url":"https://example.com/live","method":"POST","timeout":30,"retries":3}`),
		ServiceID:  service.ID,
		IsActive:   true,
	}
	if err := db.Create(&liveAction).Error; err != nil {
		t.Fatalf("create live action: %v", err)
	}
	version := domain.ServiceDefinitionVersion{
		ServiceID:   service.ID,
		Version:     1,
		ContentHash: "actions-snapshot",
		EngineType:  "smart",
		ActionsJSON: domain.JSONField(`[{"id":` + itoa(liveAction.ID) + `,"code":"notify","name":"Snapshot action","description":"old","actionType":"http","configJson":{"url":"https://example.com/snapshot","method":"POST","timeout":30,"retries":3},"isActive":true}]`),
	}
	if err := db.Create(&version).Error; err != nil {
		t.Fatalf("create service version: %v", err)
	}
	ticket := domain.Ticket{
		Code:             "TICK-SNAPSHOT-ACTIONS",
		Title:            "snapshot actions",
		ServiceID:        service.ID,
		ServiceVersionID: &version.ID,
		EngineType:       "smart",
		Status:           TicketStatusDecisioning,
		PriorityID:       1,
		RequesterID:      1,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	store := NewDecisionDataStore(db)
	actions, err := store.ListActiveServiceActions(ticket.ID, service.ID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if len(actions) != 1 || actions[0].Name != "Snapshot action" {
		t.Fatalf("expected snapshot action, got %+v", actions)
	}
	if !strings.Contains(actions[0].ConfigJSON, "https://example.com/snapshot") {
		t.Fatalf("expected snapshot config, got %s", actions[0].ConfigJSON)
	}

	action, err := store.GetServiceAction(ticket.ID, liveAction.ID, service.ID)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if action.Name != "Snapshot action" || strings.Contains(action.ConfigJSON, "https://example.com/live") {
		t.Fatalf("expected get action to use snapshot, got %+v", action)
	}
}

func itoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
