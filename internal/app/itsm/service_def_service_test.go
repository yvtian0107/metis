package itsm

import (
	"errors"
	"testing"

	"metis/internal/app/ai"
)

func TestServiceDefServiceCreate_RejectsMissingCatalog(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)

	_, err := svc.Create(&ServiceDefinition{
		Name:       "VPN",
		Code:       "vpn",
		CatalogID:  999,
		EngineType: "classic",
	})
	if !errors.Is(err, ErrCatalogNotFound) {
		t.Fatalf("expected ErrCatalogNotFound, got %v", err)
	}
}

func TestServiceDefServiceList_FiltersByEngineType(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	_, _ = svc.Create(&ServiceDefinition{Name: "Classic", Code: "classic", CatalogID: root.ID, EngineType: "classic"})
	_, _ = svc.Create(&ServiceDefinition{Name: "Smart", Code: "smart", CatalogID: root.ID, EngineType: "smart"})

	engineType := "smart"
	items, total, err := svc.List(ServiceDefListParams{EngineType: &engineType, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].EngineType != "smart" {
		t.Fatalf("unexpected filter result: total=%d items=%+v", total, items)
	}
}

func TestServiceDefServiceCreate_AllowsWorkflowJSONOnSmartService(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	created, err := svc.Create(&ServiceDefinition{
		Name:         "Smart",
		Code:         "smart",
		CatalogID:    root.ID,
		EngineType:   "smart",
		WorkflowJSON: JSONField(`{"nodes":[],"edges":[]}`),
	})
	if err != nil {
		t.Fatalf("smart service with workflowJSON should be allowed, got %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected created service to have ID")
	}
}

func TestServiceDefServiceCreate_RejectsAgentIDOnClassicService(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	agent := ai.Agent{Name: "agent", Type: ai.AgentTypeAssistant, IsActive: true, Visibility: ai.AgentVisibilityTeam, CreatedBy: 1}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	_, err := svc.Create(&ServiceDefinition{
		Name:       "Classic",
		Code:       "classic",
		CatalogID:  root.ID,
		EngineType: "classic",
		AgentID:    &agent.ID,
	})
	if err == nil || err.Error() != "service engine field mismatch" {
		t.Fatalf("expected service engine field mismatch, got %v", err)
	}
}

func TestServiceDefServiceUpdate_RejectsInactiveAgent(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	service, _ := svc.Create(&ServiceDefinition{Name: "Smart", Code: "smart", CatalogID: root.ID, EngineType: "smart"})
	agent := ai.Agent{Name: "agent", Type: ai.AgentTypeAssistant, IsActive: false, Visibility: ai.AgentVisibilityTeam, CreatedBy: 1}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := db.Model(&agent).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate agent: %v", err)
	}

	_, err := svc.Update(service.ID, map[string]any{"agent_id": agent.ID})
	if err == nil || err.Error() != "agent not available" {
		t.Fatalf("expected agent not available, got %v", err)
	}
}

func TestServiceDefinitionResponse_NoPublishHealthSnapshotBeforeGeneration(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	service, err := svc.Create(&ServiceDefinition{
		Name:       "Smart",
		Code:       "smart-no-snapshot",
		CatalogID:  root.ID,
		EngineType: "smart",
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	if service.ToResponse().PublishHealthCheck != nil {
		t.Fatal("expected publish health check to be nil before generation")
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_SavesSnapshot(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-health-snapshot",
		CatalogID:         root.ID,
		EngineType:        "smart",
		CollaborationSpec: "用户提交申请后由直属经理审批",
		WorkflowJSON:      JSONField(`{"nodes":[],"edges":[]}`),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check == nil || len(check.Items) == 0 {
		t.Fatalf("expected populated health check, got %+v", check)
	}

	reloaded, err := svc.Get(service.ID)
	if err != nil {
		t.Fatalf("reload service: %v", err)
	}
	resp := reloaded.ToResponse()
	if resp.PublishHealthCheck == nil {
		t.Fatal("expected saved publish health check in response")
	}
	if resp.PublishHealthCheck.ServiceID != service.ID {
		t.Fatalf("expected service id %d, got %d", service.ID, resp.PublishHealthCheck.ServiceID)
	}
	if resp.PublishHealthCheck.CheckedAt == nil {
		t.Fatal("expected checkedAt to be set")
	}
}
