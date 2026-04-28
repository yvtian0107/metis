package definition

import (
	"context"
	"errors"
	"fmt"
	. "metis/internal/app/itsm/catalog"
	. "metis/internal/app/itsm/config"
	. "metis/internal/app/itsm/domain"
	"testing"
	"time"

	"gorm.io/gorm"
	ai "metis/internal/app/ai/runtime"
	"metis/internal/llm"
	"metis/internal/model"
)

type fakePublishHealthConfigProvider struct {
	cfg LLMEngineRuntimeConfig
	err error
}

func (f fakePublishHealthConfigProvider) HealthCheckRuntimeConfig() (LLMEngineRuntimeConfig, error) {
	return f.cfg, f.err
}

func (fakePublishHealthConfigProvider) DecisionMode() string     { return "direct_first" }
func (fakePublishHealthConfigProvider) DecisionAgentID() uint    { return 0 }
func (fakePublishHealthConfigProvider) FallbackAssigneeID() uint { return 0 }
func (fakePublishHealthConfigProvider) AuditLevel() string       { return "full" }

type fakePublishHealthLLMClient struct {
	resp *llm.ChatResponse
	err  error
}

func (f fakePublishHealthLLMClient) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func (fakePublishHealthLLMClient) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	return nil, llm.ErrNotSupported
}

func (fakePublishHealthLLMClient) Embedding(context.Context, llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrNotSupported
}

func testPublishHealthRuntimeConfig() LLMEngineRuntimeConfig {
	return LLMEngineRuntimeConfig{
		Model:          "gpt-test",
		Protocol:       llm.ProtocolOpenAI,
		BaseURL:        "https://example.test/v1",
		APIKey:         "test-key",
		Temperature:    0.2,
		MaxTokens:      1024,
		MaxRetries:     1,
		TimeoutSeconds: 45,
		SystemPrompt:   "health prompt",
	}
}

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
				{"id":"start","type":"start","data":{"label":"閻庢鍠掗崑鎾斥攽?}},
				{"id":"action","type":"action","data":{"label":"闂佸憡鏌ｉ崝瀣礊?}},
				{"id":"end","type":"end","data":{"label":"缂傚倷鐒﹂幐璇差焽?}}
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

func TestServiceDefServiceRefreshPublishHealthCheck_SavesAISnapshot(t *testing.T) {
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
		IntakeFormSchema:  serviceHealthIntakeFormSchema(),
		CollaborationSpec: "submit request and end after approval",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(validServiceHealthWorkflow(user.ID)),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{
			resp: &llm.ChatResponse{
				Content: `{"status":"warn","items":[{"key":"spec","label":"协作规范","status":"warn","message":"协作规范中缺少升级路径","location":{"kind":"collaboration_spec","path":"service.collaborationSpec"},"recommendation":"在协作规范补充升级处理路径。","evidence":"当前协作规范未定义升级处理规则。"}]}`,
			},
		}, nil
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check == nil {
		t.Fatal("expected health check")
	}
	if check.Status != "warn" || len(check.Items) != 1 {
		t.Fatalf("unexpected health check: %+v", check)
	}
	if check.Items[0].Key != "spec" || check.Items[0].Status != "warn" {
		t.Fatalf("unexpected health item: %+v", check.Items[0])
	}

	reloaded, err := svc.Get(service.ID)
	if err != nil {
		t.Fatalf("reload service: %v", err)
	}
	resp := reloaded.ToResponse()
	if resp.PublishHealthCheck == nil {
		t.Fatal("expected saved publish health check in response")
	}
	if resp.PublishHealthCheck.CheckedAt == nil {
		t.Fatal("expected checkedAt to be set")
	}
	if resp.PublishHealthCheck.Status != "warn" || len(resp.PublishHealthCheck.Items) != 1 {
		t.Fatalf("expected saved warn snapshot, got %+v", resp.PublishHealthCheck)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_FailureBlocks(t *testing.T) {
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
		Code:              "smart-health-failure",
		CatalogID:         root.ID,
		EngineType:        "smart",
		IntakeFormSchema:  serviceHealthIntakeFormSchema(),
		CollaborationSpec: "submit request and end after approval",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(validServiceHealthWorkflow(user.ID)),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{err: ErrEngineNotConfigured}
	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check should not return error when llm fails: %v", err)
	}
	if check.Status != "fail" {
		t.Fatalf("expected fail status, got %+v", check)
	}
	if len(check.Items) != 1 || check.Items[0].Key != "health_engine" || check.Items[0].Status != "fail" {
		t.Fatalf("expected health_engine fail item, got %+v", check.Items)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_WarnsOnMissingSmartIntakeForm(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	user := createServiceHealthUser(t, db, "operator", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-missing-intake-form",
		CatalogID:         root.ID,
		EngineType:        "smart",
		CollaborationSpec: "collect request details and route them",
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
	if check.Status != "warn" {
		t.Fatalf("expected warn status, got %+v", check)
	}
	if !serviceHealthHasItem(check, "intake_form", "warn") {
		t.Fatalf("expected intake_form warning, got %+v", check.Items)
	}

	if _, err := svc.Update(service.ID, map[string]any{"intake_form_schema": serviceHealthIntakeFormSchema()}); err != nil {
		t.Fatalf("update intake form schema: %v", err)
	}
	check, err = svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check after form update: %v", err)
	}
	if serviceHealthHasItem(check, "intake_form", "warn") {
		t.Fatalf("expected intake_form warning to clear, got %+v", check.Items)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_CoreFailures(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, db *gorm.DB, service *ServiceDefinition, serviceAgent *ai.Agent, decisionAgent *ai.Agent)
		wantKey    string
		wantStatus string
	}{
		{name: "empty collaboration spec", setup: func(t *testing.T, db *gorm.DB, service *ServiceDefinition, serviceAgent *ai.Agent, decisionAgent *ai.Agent) {
			service.CollaborationSpec = ""
		}, wantKey: "collaboration_spec", wantStatus: "fail"},
		{name: "inactive service agent", setup: func(t *testing.T, db *gorm.DB, service *ServiceDefinition, serviceAgent *ai.Agent, decisionAgent *ai.Agent) {
			if err := db.Model(serviceAgent).Update("is_active", false).Error; err != nil {
				t.Fatalf("deactivate service agent: %v", err)
			}
		}, wantKey: "service_agent", wantStatus: "fail"},
		{name: "inactive decision agent", setup: func(t *testing.T, db *gorm.DB, service *ServiceDefinition, serviceAgent *ai.Agent, decisionAgent *ai.Agent) {
			if err := db.Model(decisionAgent).Update("is_active", false).Error; err != nil {
				t.Fatalf("deactivate decision agent: %v", err)
			}
		}, wantKey: "decision_agent", wantStatus: "fail"},
		{name: "path engine model missing", setup: func(t *testing.T, db *gorm.DB, service *ServiceDefinition, serviceAgent *ai.Agent, decisionAgent *ai.Agent) {
			if err := db.Model(&model.SystemConfig{}).Where("\"key\" = ?", SmartTicketPathModelKey).Update("value", "0").Error; err != nil {
				t.Fatalf("clear path engine model: %v", err)
			}
		}, wantKey: "path_engine", wantStatus: "fail"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			svc := newServiceDefServiceForTest(t, db)
			svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
			catSvc := newCatalogServiceForTest(t, db)
			root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
			user := createServiceHealthUser(t, db, "operator", true)
			serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
			decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
			setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
			seedServiceHealthPathEngine(t, db)
			service, err := svc.Create(&ServiceDefinition{Name: "Smart", Code: fmt.Sprintf("smart-core-%d", i), CatalogID: root.ID, EngineType: "smart", IntakeFormSchema: serviceHealthIntakeFormSchema(), CollaborationSpec: "route the request to the handler", AgentID: &serviceAgent.ID, WorkflowJSON: JSONField(validServiceHealthWorkflow(user.ID))})
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
	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := svc.Create(&ServiceDefinition{Name: "Smart", Code: "smart-invalid-action-ref", CatalogID: root.ID, EngineType: "smart", IntakeFormSchema: serviceHealthIntakeFormSchema(), CollaborationSpec: "collect details and run a precheck action", AgentID: &serviceAgent.ID, WorkflowJSON: JSONField(workflowWithActionID(999999))})
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
		t.Fatalf("expected reference_path_action warning, got %+v", check.Items)
	}
}

func TestServiceDefServiceHealthCheck_RefreshesLatestSnapshot(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-health-refresh",
		CatalogID:         root.ID,
		EngineType:        "smart",
		CollaborationSpec: "submit request and end after approval",
		WorkflowJSON:      JSONField(`{"nodes":[],"edges":[]}`),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: LLMEngineRuntimeConfig{
		Model:          "gpt-test",
		Protocol:       llm.ProtocolOpenAI,
		BaseURL:        "https://example.test/v1",
		APIKey:         "test-key",
		Temperature:    0.2,
		MaxTokens:      1024,
		MaxRetries:     1,
		TimeoutSeconds: 45,
		SystemPrompt:   "health prompt",
	}}
	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{
			resp: &llm.ChatResponse{
				Content: `{"status":"pass","items":[]}`,
			},
		}, nil
	}

	first, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if first.Status != "pass" {
		t.Fatalf("expected first pass, got %+v", first)
	}
	firstCheckedAt := *first.CheckedAt

	time.Sleep(10 * time.Millisecond)

	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{
			resp: &llm.ChatResponse{
				Content: `{"status":"fail","items":[{"key":"workflow","label":"参考路径","status":"fail","message":"存在阻塞节点","location":{"kind":"runtime_config","path":"runtime.decisionMode"},"recommendation":"修复阻塞节点后重新检查。","evidence":"流程验证存在阻塞风险。"}]}`,
			},
		}, nil
	}

	latest, err := svc.HealthCheck(service.ID)
	if err != nil {
		t.Fatalf("health check refresh: %v", err)
	}
	if latest.Status != "fail" {
		t.Fatalf("expected latest fail, got %+v", latest)
	}
	if latest.CheckedAt == nil || !latest.CheckedAt.After(firstCheckedAt) {
		t.Fatalf("expected checkedAt to be refreshed, first=%s latest=%v", firstCheckedAt, latest.CheckedAt)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_FiltersIncompleteItems(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	user := createServiceHealthUser(t, db, "operator", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := svc.Create(&ServiceDefinition{Name: "Smart", Code: "smart-health-incomplete-item", CatalogID: root.ID, EngineType: "smart", IntakeFormSchema: serviceHealthIntakeFormSchema(), CollaborationSpec: "submit request and end after approval", AgentID: &serviceAgent.ID, WorkflowJSON: JSONField(validServiceHealthWorkflow(user.ID))})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{
			resp: &llm.ChatResponse{
				Content: `{"status":"warn","items":[{"key":"spec","label":"协作规范","status":"warn","message":"缺少处理路径","location":{"kind":"collaboration_spec","path":"service.collaborationSpec"},"evidence":"协作规范没有说明驳回后的路径。"}]}`,
			},
		}, nil
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "fail" {
		t.Fatalf("expected fail status after filtering invalid items, got %+v", check)
	}
	if len(check.Items) != 1 || check.Items[0].Key != "health_engine" {
		t.Fatalf("expected health_engine fail item, got %+v", check.Items)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_RejectsUnknownLocationPath(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	user := createServiceHealthUser(t, db, "operator", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := svc.Create(&ServiceDefinition{Name: "Smart", Code: "smart-health-bad-location", CatalogID: root.ID, EngineType: "smart", IntakeFormSchema: serviceHealthIntakeFormSchema(), CollaborationSpec: "submit request and end after approval", AgentID: &serviceAgent.ID, WorkflowJSON: JSONField(validServiceHealthWorkflow(user.ID))})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{
			resp: &llm.ChatResponse{
				Content: `{"status":"warn","items":[{"key":"workflow","label":"流程节点","status":"warn","message":"存在歧义","location":{"kind":"workflow_node","path":"service.unknown.nodes","refId":"n1"},"recommendation":"补充节点说明。","evidence":"节点描述不清。"}]}`,
			},
		}, nil
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "fail" {
		t.Fatalf("expected fail status after filtering unmapped location, got %+v", check)
	}
	if len(check.Items) != 1 || check.Items[0].Key != "health_engine" {
		t.Fatalf("expected health_engine fail item, got %+v", check.Items)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_ActionIssuesNeedEvidence(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	user := createServiceHealthUser(t, db, "operator", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := svc.Create(&ServiceDefinition{Name: "Smart", Code: "smart-health-action-evidence", CatalogID: root.ID, EngineType: "smart", IntakeFormSchema: serviceHealthIntakeFormSchema(), CollaborationSpec: "submit request and end after approval", AgentID: &serviceAgent.ID, WorkflowJSON: JSONField(validServiceHealthWorkflow(user.ID))})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{resp: &llm.ChatResponse{Content: `{"status":"warn","items":[{"key":"action_missing","label":"动作缺失","status":"warn","message":"缺少动作","location":{"kind":"action","path":"actions[id=deploy]","refId":"deploy"},"recommendation":"补充动作配置。","evidence":"流程需要自动化动作。"}]}`}}, nil
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "fail" {
		t.Fatalf("expected fail status after filtering action issue without evidence, got %+v", check)
	}
	if len(check.Items) != 1 || check.Items[0].Key != "health_engine" {
		t.Fatalf("expected health_engine fail item, got %+v", check.Items)
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
			{"id":"start","type":"start","data":{"label":"Start"}},
			{"id":"form","type":"form","data":{"label":"Collect details","participants":[{"type":"user","value":"%d"}]}},
			{"id":"end","type":"end","data":{"label":"End"}}
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
			{"id":"start","type":"start","data":{"label":"Start"}},
			{"id":"action","type":"action","data":{"label":"Precheck","action_id":%d}},
			{"id":"end","type":"end","data":{"label":"End"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"action","data":{}},
			{"id":"e2","source":"action","target":"end","data":{}}
		]
	}`, actionID)
}

func serviceHealthIntakeFormSchema() JSONField {
	return JSONField(`{"version":1,"fields":[{"key":"summary","type":"text","label":"Summary","required":true}]}`)
}

func serviceHealthHasItem(check *ServiceHealthCheck, key string, status string) bool {
	for _, item := range check.Items {
		if item.Key == key && item.Status == status {
			return true
		}
	}
	return false
}
