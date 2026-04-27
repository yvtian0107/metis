package definition

import (
	"errors"
	"fmt"
	. "metis/internal/app/itsm/catalog"
	. "metis/internal/app/itsm/config"
	. "metis/internal/app/itsm/domain"
	"testing"

	"gorm.io/gorm"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/engine"
	"metis/internal/model"
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

func TestServiceDefServiceParticipantRiskAllowsRequester(t *testing.T) {
	svc := &ServiceDefService{}
	if issue := svc.checkParticipantRisk("form", engine.Participant{Type: "requester"}); issue != nil {
		t.Fatalf("expected requester participant to be allowed, got %+v", issue)
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

func TestServiceDefServiceCreate_RejectsInvalidClassicWorkflow(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	_, err := svc.Create(&ServiceDefinition{
		Name:       "Classic",
		Code:       "classic-invalid-workflow",
		CatalogID:  root.ID,
		EngineType: "classic",
		WorkflowJSON: JSONField(`{
			"nodes": [
				{"id":"start","type":"start","data":{"label":"开始"}},
				{"id":"action","type":"action","data":{"label":"动作"}},
				{"id":"end","type":"end","data":{"label":"结束"}}
			],
			"edges": [
				{"id":"e1","source":"start","target":"action","data":{}},
				{"id":"e2","source":"action","target":"end","data":{}}
			]
		}`),
	})
	if !errors.Is(err, ErrWorkflowValidation) {
		t.Fatalf("expected ErrWorkflowValidation, got %v", err)
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
	user := createServiceHealthUser(t, db, "operator", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-health-snapshot",
		CatalogID:         root.ID,
		EngineType:        "smart",
		CollaborationSpec: "用户提交申请后由直属经理处理",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(validServiceHealthWorkflow(user.ID)),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check == nil {
		t.Fatal("expected health check")
	}
	if check.Status != "pass" {
		t.Fatalf("expected pass health check, got %+v", check)
	}
	if len(check.Items) != 0 {
		t.Fatalf("expected no risk items, got %+v", check.Items)
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
	if resp.PublishHealthCheck.Status != "pass" || len(resp.PublishHealthCheck.Items) != 0 {
		t.Fatalf("expected saved empty pass snapshot, got %+v", resp.PublishHealthCheck)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_CoreFailures(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, db *gorm.DB, service *ServiceDefinition, serviceAgent *ai.Agent, decisionAgent *ai.Agent)
		wantKey    string
		wantStatus string
	}{
		{
			name: "empty collaboration spec",
			setup: func(t *testing.T, db *gorm.DB, service *ServiceDefinition, serviceAgent *ai.Agent, decisionAgent *ai.Agent) {
				service.CollaborationSpec = ""
			},
			wantKey:    "collaboration_spec",
			wantStatus: "fail",
		},
		{
			name: "inactive service agent",
			setup: func(t *testing.T, db *gorm.DB, service *ServiceDefinition, serviceAgent *ai.Agent, decisionAgent *ai.Agent) {
				if err := db.Model(serviceAgent).Update("is_active", false).Error; err != nil {
					t.Fatalf("deactivate service agent: %v", err)
				}
			},
			wantKey:    "service_agent",
			wantStatus: "fail",
		},
		{
			name: "inactive decision agent",
			setup: func(t *testing.T, db *gorm.DB, service *ServiceDefinition, serviceAgent *ai.Agent, decisionAgent *ai.Agent) {
				if err := db.Model(decisionAgent).Update("is_active", false).Error; err != nil {
					t.Fatalf("deactivate decision agent: %v", err)
				}
			},
			wantKey:    "decision_agent",
			wantStatus: "fail",
		},
		{
			name: "path engine model missing",
			setup: func(t *testing.T, db *gorm.DB, service *ServiceDefinition, serviceAgent *ai.Agent, decisionAgent *ai.Agent) {
				if err := db.Model(&model.SystemConfig{}).
					Where("\"key\" = ?", SmartTicketPathModelKey).
					Update("value", "0").Error; err != nil {
					t.Fatalf("clear path engine model: %v", err)
				}
			},
			wantKey:    "path_engine",
			wantStatus: "fail",
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			svc := newServiceDefServiceForTest(t, db)
			catSvc := newCatalogServiceForTest(t, db)
			root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
			user := createServiceHealthUser(t, db, "operator", true)
			serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
			decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
			setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
			seedServiceHealthPathEngine(t, db)
			service, err := svc.Create(&ServiceDefinition{
				Name:              "Smart",
				Code:              fmt.Sprintf("smart-core-%d", i),
				CatalogID:         root.ID,
				EngineType:        "smart",
				CollaborationSpec: "用户提交申请后由处理人处理",
				AgentID:           &serviceAgent.ID,
				WorkflowJSON:      JSONField(validServiceHealthWorkflow(user.ID)),
			})
			if err != nil {
				t.Fatalf("create service: %v", err)
			}
			tt.setup(t, db, service, &serviceAgent, &decisionAgent)
			if err := db.Save(service).Error; err != nil {
				t.Fatalf("save service updates: %v", err)
			}

			check, err := svc.RefreshPublishHealthCheck(service.ID)
			if err != nil {
				t.Fatalf("refresh health check: %v", err)
			}
			if check.Status != tt.wantStatus {
				t.Fatalf("expected status %s, got %+v", tt.wantStatus, check)
			}
			if !serviceHealthHasItem(check, tt.wantKey, tt.wantStatus) {
				t.Fatalf("expected %s item with status %s, got %+v", tt.wantKey, tt.wantStatus, check.Items)
			}
		})
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_WarnsOnInvalidReferencedAction(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-invalid-action",
		CatalogID:         root.ID,
		EngineType:        "smart",
		CollaborationSpec: "用户提交申请后执行自动化动作",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(workflowWithActionID(999)),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "warn" {
		t.Fatalf("expected warn status, got %+v", check)
	}
	if !serviceHealthHasItem(check, "reference_path_action", "warn") {
		t.Fatalf("expected invalid action warning, got %+v", check.Items)
	}
}

func createServiceHealthAgent(t *testing.T, db *gorm.DB, name string, active bool) ai.Agent {
	t.Helper()
	agent := ai.Agent{Name: name, Type: ai.AgentTypeAssistant, IsActive: active, Visibility: ai.AgentVisibilityTeam, CreatedBy: 1}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	return agent
}

func createServiceHealthUser(t *testing.T, db *gorm.DB, username string, active bool) model.User {
	t.Helper()
	user := model.User{Username: username, IsActive: active}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func setServiceHealthDecisionAgent(t *testing.T, db *gorm.DB, agentID uint) {
	t.Helper()
	cfg := model.SystemConfig{Key: SmartTicketDecisionAgentKey, Value: fmt.Sprintf("%d", agentID)}
	if err := db.Save(&cfg).Error; err != nil {
		t.Fatalf("set decision agent config: %v", err)
	}
}

func seedServiceHealthPathEngine(t *testing.T, db *gorm.DB) ai.AIModel {
	t.Helper()
	provider := ai.Provider{
		Name:     "Path Provider",
		Type:     ai.ProviderTypeOpenAI,
		Protocol: "openai",
		BaseURL:  "https://example.test",
		Status:   ai.ProviderStatusActive,
	}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create path provider: %v", err)
	}
	pathModel := ai.AIModel{
		ProviderID:  provider.ID,
		ModelID:     "path-test",
		DisplayName: "Path Test",
		Type:        ai.ModelTypeLLM,
		Status:      ai.ModelStatusActive,
	}
	if err := db.Create(&pathModel).Error; err != nil {
		t.Fatalf("create path model: %v", err)
	}
	cfg := model.SystemConfig{Key: SmartTicketPathModelKey, Value: fmt.Sprintf("%d", pathModel.ID)}
	if err := db.Save(&cfg).Error; err != nil {
		t.Fatalf("set path model config: %v", err)
	}
	return pathModel
}

func validServiceHealthWorkflow(userID uint) string {
	return fmt.Sprintf(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"form","type":"form","data":{"label":"收集信息","participants":[{"type":"user","value":"%d"}]}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"form","data":{}},
			{"id":"e2","source":"form","target":"end","data":{}}
		]
	}`, userID)
}

func workflowWithActionID(actionID uint) string {
	return fmt.Sprintf(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"action","type":"action","data":{"label":"预检","action_id":%d}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"action","data":{}},
			{"id":"e2","source":"action","target":"end","data":{}}
		]
	}`, actionID)
}

func serviceHealthHasItem(check *ServiceHealthCheck, key string, status string) bool {
	for _, item := range check.Items {
		if item.Key == key && item.Status == status {
			return true
		}
	}
	return false
}
