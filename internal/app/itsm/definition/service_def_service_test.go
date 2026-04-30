package definition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	. "metis/internal/app/itsm/catalog"
	. "metis/internal/app/itsm/config"
	. "metis/internal/app/itsm/domain"
	orgdomain "metis/internal/app/org/domain"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	ai "metis/internal/app/ai/runtime"
	"metis/internal/llm"
	"metis/internal/model"
)

type fakePublishHealthConfigProvider struct {
	cfg                LLMEngineRuntimeConfig
	err                error
	fallbackAssigneeID uint
}

func (f fakePublishHealthConfigProvider) HealthCheckRuntimeConfig() (LLMEngineRuntimeConfig, error) {
	return f.cfg, f.err
}

func (fakePublishHealthConfigProvider) DecisionMode() string  { return "direct_first" }
func (fakePublishHealthConfigProvider) DecisionAgentID() uint { return 0 }
func (f fakePublishHealthConfigProvider) FallbackAssigneeID() uint {
	return f.fallbackAssigneeID
}
func (fakePublishHealthConfigProvider) AuditLevel() string { return "full" }

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

func TestServiceDefServiceList_FiltersByRootCatalog(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, err := catSvc.Create("Root", "root", "", "", nil, 10)
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	child, err := catSvc.Create("Child", "child", "", "", &root.ID, 10)
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	otherRoot, err := catSvc.Create("Other", "other", "", "", nil, 20)
	if err != nil {
		t.Fatalf("create other root: %v", err)
	}
	if _, err := svc.Create(&ServiceDefinition{Name: "Root Direct", Code: "root-direct", CatalogID: root.ID, EngineType: "classic"}); err != nil {
		t.Fatalf("create root direct service: %v", err)
	}
	if _, err := svc.Create(&ServiceDefinition{Name: "Child Service", Code: "child-service", CatalogID: child.ID, EngineType: "classic"}); err != nil {
		t.Fatalf("create child service: %v", err)
	}
	if _, err := svc.Create(&ServiceDefinition{Name: "Other Service", Code: "other-service", CatalogID: otherRoot.ID, EngineType: "classic"}); err != nil {
		t.Fatalf("create other service: %v", err)
	}

	items, total, err := svc.List(ServiceDefListParams{RootCatalogID: &root.ID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list by root catalog: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected root direct and child services only, total=%d items=%+v", total, items)
	}
	codes := map[string]bool{}
	for _, item := range items {
		codes[item.Code] = true
	}
	if !codes["root-direct"] || !codes["child-service"] || codes["other-service"] {
		t.Fatalf("unexpected root catalog result: %+v", codes)
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

func TestServiceRuntimeVersionSnapshotsSLAAndEscalationRules(t *testing.T) {
	db := newTestDB(t)
	catSvc := newCatalogServiceForTest(t, db)
	root, err := catSvc.Create("Root", "root-sla-snapshot", "", "", nil, 10)
	if err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	sla := SLATemplate{Name: "Gold SLA", Code: "gold-sla", ResponseMinutes: 5, ResolutionMinutes: 60, IsActive: true}
	if err := db.Create(&sla).Error; err != nil {
		t.Fatalf("create sla: %v", err)
	}
	service := ServiceDefinition{Name: "VPN", Code: "vpn-sla-snapshot", CatalogID: root.ID, EngineType: "smart", SLAID: &sla.ID, IsActive: true}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	rule := EscalationRule{
		SLAID:        sla.ID,
		TriggerType:  "response_timeout",
		Level:        1,
		WaitMinutes:  0,
		ActionType:   "notify",
		TargetConfig: JSONField(`{"recipients":[{"type":"user","value":"10"}],"channelId":5}`),
		IsActive:     true,
	}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatalf("create escalation rule: %v", err)
	}

	version, err := GetOrCreateServiceRuntimeVersion(db, service.ID)
	if err != nil {
		t.Fatalf("create runtime version: %v", err)
	}
	if len(version.SLATemplateJSON) == 0 {
		t.Fatal("expected runtime version to snapshot SLA template")
	}
	if len(version.EscalationRulesJSON) == 0 {
		t.Fatal("expected runtime version to snapshot escalation rules")
	}
	var slaSnapshot SLATemplateResponse
	if err := json.Unmarshal(version.SLATemplateJSON, &slaSnapshot); err != nil {
		t.Fatalf("decode sla snapshot: %v", err)
	}
	if slaSnapshot.ID != sla.ID || slaSnapshot.ResponseMinutes != 5 || slaSnapshot.ResolutionMinutes != 60 {
		t.Fatalf("unexpected sla snapshot: %+v", slaSnapshot)
	}
	var ruleSnapshots []EscalationRuleResponse
	if err := json.Unmarshal(version.EscalationRulesJSON, &ruleSnapshots); err != nil {
		t.Fatalf("decode rule snapshot: %v", err)
	}
	if len(ruleSnapshots) != 1 || ruleSnapshots[0].ID != rule.ID || ruleSnapshots[0].TriggerType != "response_timeout" {
		t.Fatalf("unexpected rule snapshots: %+v", ruleSnapshots)
	}

	if err := db.Model(&SLATemplate{}).Where("id = ?", sla.ID).Updates(map[string]any{
		"response_minutes":   99,
		"resolution_minutes": 199,
	}).Error; err != nil {
		t.Fatalf("mutate sla: %v", err)
	}
	if err := db.Model(&EscalationRule{}).Where("id = ?", rule.ID).Updates(map[string]any{
		"wait_minutes": 42,
		"action_type":  "reassign",
	}).Error; err != nil {
		t.Fatalf("mutate rule: %v", err)
	}
	var reloaded ServiceDefinitionVersion
	if err := db.First(&reloaded, version.ID).Error; err != nil {
		t.Fatalf("reload runtime version: %v", err)
	}
	if strings.Contains(string(reloaded.SLATemplateJSON), "99") || strings.Contains(string(reloaded.EscalationRulesJSON), "reassign") {
		t.Fatalf("runtime version SLA snapshot drifted after live mutation: sla=%s rules=%s", reloaded.SLATemplateJSON, reloaded.EscalationRulesJSON)
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

func TestServiceDefServiceRefreshPublishHealthCheck_LLMFailureFallsBackToBasePass(t *testing.T) {
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
	if check.Status != "pass" {
		t.Fatalf("expected base pass status when llm fails, got %+v", check)
	}
	if len(check.Items) != 0 || serviceHealthHasItem(check, "health_engine", "fail") {
		t.Fatalf("expected no service-level health_engine failure, got %+v", check.Items)
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

func TestServiceDefServiceRefreshPublishHealthCheck_LLMFailureKeepsBaseWarning(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	svc.engineConfigSvc = fakePublishHealthConfigProvider{err: ErrEngineNotConfigured}
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	user := createServiceHealthUser(t, db, "operator", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-health-base-warning",
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
		t.Fatalf("expected base warning status when llm fails, got %+v", check)
	}
	if !serviceHealthHasItem(check, "intake_form", "warn") {
		t.Fatalf("expected intake_form warning to remain, got %+v", check.Items)
	}
	if serviceHealthHasItem(check, "health_engine", "fail") {
		t.Fatalf("llm failure should not overwrite base warning: %+v", check.Items)
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

func TestServiceDefServiceRefreshPublishHealthCheck_DoesNotRequireServiceAgent(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
	catSvc := newCatalogServiceForTest(t, db)
	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	user := createServiceHealthUser(t, db, "operator", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-no-service-agent",
		CatalogID:         root.ID,
		EngineType:        "smart",
		IntakeFormSchema:  serviceHealthIntakeFormSchema(),
		CollaborationSpec: "route the request to the handler",
		WorkflowJSON:      JSONField(validServiceHealthWorkflow(user.ID)),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if serviceHealthHasItem(check, "service_agent", "fail") {
		t.Fatalf("service agent should not be required, got %+v", check.Items)
	}
	if serviceHealthHasItem(check, "decision_agent", "fail") {
		t.Fatalf("decision agent should remain healthy, got %+v", check.Items)
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

func TestServiceDefServiceRefreshPublishHealthCheck_FailsWhenPositionDepartmentHasNoApprover(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	seedWorkflowParticipantOrgData(t, db, "missing")

	service, err := svc.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-health-missing-position-approver",
		CatalogID:         root.ID,
		EngineType:        "smart",
		IntakeFormSchema:  serviceHealthIntakeFormSchema(),
		CollaborationSpec: "route to ops admin",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(workflowWithPositionDepartmentParticipant("ops_admin", "it")),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "fail" {
		t.Fatalf("expected fail status, got %+v", check)
	}
	if !serviceHealthHasItem(check, "reference_path_participant", "fail") {
		t.Fatalf("expected reference_path_participant fail, got %+v", check.Items)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_FailsWhenPositionDepartmentUserInactive(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	seedWorkflowParticipantOrgData(t, db, "inactive")

	service, err := svc.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-health-inactive-position-approver",
		CatalogID:         root.ID,
		EngineType:        "smart",
		IntakeFormSchema:  serviceHealthIntakeFormSchema(),
		CollaborationSpec: "route to ops admin",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(workflowWithPositionDepartmentParticipant("ops_admin", "it")),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "fail" {
		t.Fatalf("expected fail status, got %+v", check)
	}
	if !serviceHealthHasItem(check, "reference_path_participant", "fail") {
		t.Fatalf("expected reference_path_participant fail, got %+v", check.Items)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_FiltersLLMFallbackAssigneeValidationGuess(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	operator := createServiceHealthUser(t, db, "operator", true)
	fallback := createServiceHealthUser(t, db, "fallback_owner", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-health-fallback-guess",
		CatalogID:         root.ID,
		EngineType:        "smart",
		IntakeFormSchema:  serviceHealthIntakeFormSchema(),
		CollaborationSpec: "submit request and end after processing",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(validServiceHealthWorkflow(operator.ID)),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig(), fallbackAssigneeID: fallback.ID}
	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{resp: &llm.ChatResponse{Content: `{"status":"fail","items":[{"key":"fallback_assignee","label":"缺少兜底处理人校验","status":"fail","message":"运行时配置中定义了 fallbackAssignee，但未明确校验其有效性。","location":{"kind":"runtime_config","path":"runtime.fallbackAssignee"},"recommendation":"请确保 fallbackAssignee 的值为有效用户 ID，并在运行时进行校验。","evidence":"runtime 配置中 fallbackAssignee 的值为 1，但未提供校验逻辑。"}]}`}}, nil
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "pass" {
		t.Fatalf("expected fallback validation guess to be filtered, got %+v", check)
	}
	if serviceHealthHasItem(check, "fallback_assignee", "fail") || serviceHealthHasItem(check, "health_engine", "fail") {
		t.Fatalf("expected no fallback false positive or health engine failure, got %+v", check.Items)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_WarnsWhenFallbackAssigneeInvalid(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	operator := createServiceHealthUser(t, db, "operator", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-health-invalid-fallback",
		CatalogID:         root.ID,
		EngineType:        "smart",
		IntakeFormSchema:  serviceHealthIntakeFormSchema(),
		CollaborationSpec: "submit request and end after processing",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(validServiceHealthWorkflow(operator.ID)),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig(), fallbackAssigneeID: 999999}
	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "warn" {
		t.Fatalf("expected invalid fallback to warn, got %+v", check)
	}
	if !serviceHealthHasItem(check, "fallback_assignee", "warn") {
		t.Fatalf("expected deterministic fallback warning, got %+v", check.Items)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_ServerAccessRolesPassWithValidFallback(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	fallback := createServiceHealthUser(t, db, "fallback_owner", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	seedServerAccessParticipantOrgData(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "生产服务器临时访问申请",
		Code:              "server-access",
		CatalogID:         root.ID,
		EngineType:        "smart",
		IntakeFormSchema:  serverAccessIntakeFormSchema(),
		CollaborationSpec: "用户在 IT 服务台提交生产服务器临时访问申请。服务台需要收集访问服务器、访问时段、操作目的和访问原因。应用发布、进程排障、日志排查、磁盘清理、主机巡检或生产运维操作交给 it/ops_admin；网络抓包、连通性诊断、ACL 调整、负载均衡变更或防火墙策略调整交给 it/network_admin；安全审计、入侵排查、漏洞修复验证、取证分析或合规检查交给 it/security_admin。参与者类型必须使用 position_department。",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(serverAccessRolesWorkflow()),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig(), fallbackAssigneeID: fallback.ID}
	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{resp: &llm.ChatResponse{Content: `{"status":"pass","items":[]}`}}, nil
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "pass" {
		t.Fatalf("expected valid server access role routing to pass, got %+v", check)
	}
	if serviceHealthHasItem(check, "fallback_assignee", "warn") || serviceHealthHasItem(check, "reference_path_participant", "fail") {
		t.Fatalf("expected no fallback or participant issue, got %+v", check.Items)
	}

	payload, _, err := svc.buildPublishHealthPayload(service)
	if err != nil {
		t.Fatalf("build publish health payload: %v", err)
	}
	runtimePayload, ok := payload["runtime"].(map[string]any)
	if !ok {
		t.Fatalf("expected runtime payload map, got %+v", payload["runtime"])
	}
	if _, ok := runtimePayload["auditLevel"]; ok {
		t.Fatalf("publish health llm payload should not expose raw auditLevel: %+v", runtimePayload)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_ServerAccessIgnoresInvalidLLMDiagnostics(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	fallback := createServiceHealthUser(t, db, "fallback_owner", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	seedServerAccessParticipantOrgData(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "生产服务器临时访问申请",
		Code:              "server-access-invalid-health-diagnostic",
		CatalogID:         root.ID,
		EngineType:        "smart",
		IntakeFormSchema:  serverAccessIntakeFormSchema(),
		CollaborationSpec: "用户在 IT 服务台提交生产服务器临时访问申请。服务台需要收集访问服务器、访问时段、操作目的和访问原因。应用发布、进程排障、日志排查、磁盘清理、主机巡检或生产运维操作交给 it/ops_admin；网络抓包、连通性诊断、ACL 调整、负载均衡变更或防火墙策略调整交给 it/network_admin；安全审计、入侵排查、漏洞修复验证、取证分析或合规检查交给 it/security_admin。参与者类型必须使用 position_department。",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(serverAccessRolesWorkflow()),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig(), fallbackAssigneeID: fallback.ID}
	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{
			resp: &llm.ChatResponse{
				Content: `{"status":"fail","items":[{"key":"participant_validation_missing","label":"参与者验证缺失","status":"fail","message":"未验证参与者配置是否符合协作规范要求。","location":{"kind":"collaboration_spec","path":"service.collaborationSpec"},"recommendation":"确保所有流程节点的参与者配置与协作规范一致。","evidence":"未看到参与者校验逻辑。"}]}`,
			},
		}, nil
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "pass" {
		t.Fatalf("expected valid server access workflow to pass after discarding invalid llm diagnostics, got %+v", check)
	}
	if len(check.Items) != 0 || serviceHealthHasItem(check, "health_engine", "fail") {
		t.Fatalf("expected no health_engine or llm diagnostic issue, got %+v", check.Items)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_FiltersLLMAuditRuntimeConfigGuess(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	fallback := createServiceHealthUser(t, db, "fallback_owner", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	seedServerAccessParticipantOrgData(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "生产服务器临时访问申请",
		Code:              "server-access-audit-runtime-guess",
		CatalogID:         root.ID,
		EngineType:        "smart",
		IntakeFormSchema:  serverAccessIntakeFormSchema(),
		CollaborationSpec: "用户在 IT 服务台提交生产服务器临时访问申请。服务台需要收集访问服务器、访问时段、操作目的和访问原因。应用发布、进程排障、日志排查、磁盘清理、主机巡检或生产运维操作交给 it/ops_admin；网络抓包、连通性诊断、ACL 调整、负载均衡变更或防火墙策略调整交给 it/network_admin；安全审计、入侵排查、漏洞修复验证、取证分析或合规检查交给 it/security_admin。参与者类型必须使用 position_department。",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(serverAccessRolesWorkflow()),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig(), fallbackAssigneeID: fallback.ID}
	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{
			resp: &llm.ChatResponse{
				Content: `{"status":"warn","items":[{"key":"audit_config_missing","label":"缺少审计配置","status":"warn","message":"服务运行时配置未明确审计日志存储位置，可能影响后续审计追踪。","location":{"kind":"runtime_config","path":"runtime.auditLevel"},"recommendation":"确保审计日志存储位置已配置并可用，例如指定日志存储路径或服务。","evidence":"runtime.auditLevel 配置为 full，但未提供具体存储位置或相关说明。"}]}`,
			},
		}, nil
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "pass" {
		t.Fatalf("expected audit runtime guess to be discarded, got %+v", check)
	}
	if len(check.Items) != 0 || serviceHealthHasLocationKind(check, "runtime_config") {
		t.Fatalf("expected no runtime_config llm issue, got %+v", check.Items)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_FiltersLLMParticipantValidationGuess(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	seedServerAccessParticipantOrgData(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "生产服务器临时访问申请",
		Code:              "server-access-participant-guess",
		CatalogID:         root.ID,
		EngineType:        "smart",
		IntakeFormSchema:  serverAccessIntakeFormSchema(),
		CollaborationSpec: "参与者类型必须使用 position_department，部门编码使用 it，岗位编码使用 ops_admin/network_admin/security_admin。",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(serverAccessRolesWorkflow()),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{resp: &llm.ChatResponse{Content: `{"status":"fail","items":[{"key":"participant_validation_missing","label":"参与者验证缺失","status":"fail","message":"未验证参与者配置是否符合协作规范要求。","location":{"kind":"collaboration_spec","path":"service.collaborationSpec"},"recommendation":"确保所有流程节点的参与者配置与协作规范一致，例如部门编码和岗位编码是否正确。","evidence":"协作规范要求参与者类型为 position_department，且部门编码为 it，但未验证配置是否正确。"}]}`}}, nil
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "pass" {
		t.Fatalf("expected participant validation guess to be filtered, got %+v", check)
	}
	if serviceHealthHasItem(check, "participant_validation_missing", "fail") || serviceHealthHasItem(check, "health_engine", "fail") {
		t.Fatalf("expected no participant validation false positive or health engine failure, got %+v", check.Items)
	}
}

func TestServiceDefServiceRefreshPublishHealthCheck_KeepsConcreteParticipantMismatch(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	seedServerAccessParticipantOrgData(t, db)
	service, err := svc.Create(&ServiceDefinition{
		Name:              "生产服务器临时访问申请",
		Code:              "server-access-participant-mismatch",
		CatalogID:         root.ID,
		EngineType:        "smart",
		IntakeFormSchema:  serverAccessIntakeFormSchema(),
		CollaborationSpec: "网络抓包、连通性诊断、ACL 调整、负载均衡变更或防火墙策略调整交给 it/network_admin。",
		AgentID:           &serviceAgent.ID,
		WorkflowJSON:      JSONField(serverAccessRolesWorkflow()),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc.engineConfigSvc = fakePublishHealthConfigProvider{cfg: testPublishHealthRuntimeConfig()}
	svc.llmClientFactory = func(string, string, string) (llm.Client, error) {
		return fakePublishHealthLLMClient{resp: &llm.ChatResponse{Content: `{"status":"warn","items":[{"key":"participant_mismatch","label":"参与者配置不一致","status":"warn","message":"network_process 的参与者配置与协作规范不一致。","location":{"kind":"workflow_node","path":"service.workflowJson.nodes[id=network_process].data.participants","refId":"network_process"},"recommendation":"将 network_process 的 position_code 调整为 network_admin，department_code 保持 it。","evidence":"实际 participant 为 {\"type\":\"position_department\",\"department_code\":\"it\",\"position_code\":\"ops_admin\"}；协作规范要求网络分支期望 participant 为 {\"type\":\"position_department\",\"department_code\":\"it\",\"position_code\":\"network_admin\"}。"}]}`}}, nil
	}

	check, err := svc.RefreshPublishHealthCheck(service.ID)
	if err != nil {
		t.Fatalf("refresh health check: %v", err)
	}
	if check.Status != "warn" {
		t.Fatalf("expected concrete participant mismatch to remain warn, got %+v", check)
	}
	if !serviceHealthHasItem(check, "participant_mismatch", "warn") {
		t.Fatalf("expected concrete participant mismatch item, got %+v", check.Items)
	}
}

func TestServiceDefServiceHealthCheck_RefreshesLatestSnapshot(t *testing.T) {
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
		Code:              "smart-health-refresh",
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
				Content: `{"status":"fail","items":[{"key":"workflow","label":"参考路径","status":"fail","message":"存在阻塞节点","location":{"kind":"workflow_node","path":"service.workflowJson.nodes[id=form]","refId":"form"},"recommendation":"修复阻塞节点后重新检查。","evidence":"流程验证存在阻塞风险。"}]}`,
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
	if check.Status != "pass" {
		t.Fatalf("expected pass status after discarding invalid llm diagnostics, got %+v", check)
	}
	if len(check.Items) != 0 || serviceHealthHasItem(check, "health_engine", "fail") {
		t.Fatalf("expected no service-level health_engine failure, got %+v", check.Items)
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
	if check.Status != "pass" {
		t.Fatalf("expected pass status after discarding unmapped llm location, got %+v", check)
	}
	if len(check.Items) != 0 || serviceHealthHasItem(check, "health_engine", "fail") {
		t.Fatalf("expected no service-level health_engine failure, got %+v", check.Items)
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
	if check.Status != "pass" {
		t.Fatalf("expected pass status after discarding invalid action diagnostic, got %+v", check)
	}
	if len(check.Items) != 0 || serviceHealthHasItem(check, "health_engine", "fail") {
		t.Fatalf("expected no service-level health_engine failure, got %+v", check.Items)
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

func workflowWithPositionDepartmentParticipant(positionCode, departmentCode string) string {
	return fmt.Sprintf(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"Start"}},
			{"id":"process","type":"process","data":{"label":"Ops approval","participants":[{"type":"position_department","position_code":"%s","department_code":"%s"}]}},
			{"id":"approved_end","type":"end","data":{"label":"Approved"}},
			{"id":"rejected_end","type":"end","data":{"label":"Rejected"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"process","data":{}},
			{"id":"e2","source":"process","target":"approved_end","data":{"outcome":"approved"}},
			{"id":"e3","source":"process","target":"rejected_end","data":{"outcome":"rejected"}}
		]
	}`, positionCode, departmentCode)
}

func serverAccessRolesWorkflow() string {
	return `{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"ops_process","type":"process","data":{"label":"运维管理员处理","participants":[{"type":"position_department","department_code":"it","position_code":"ops_admin"}]}},
			{"id":"network_process","type":"process","data":{"label":"网络管理员处理","participants":[{"type":"position_department","department_code":"it","position_code":"network_admin"}]}},
			{"id":"security_process","type":"process","data":{"label":"信息安全管理员处理","participants":[{"type":"position_department","department_code":"it","position_code":"security_admin"}]}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"ops_process","data":{}},
			{"id":"e2","source":"ops_process","target":"network_process","data":{"outcome":"approved"}},
			{"id":"e3","source":"ops_process","target":"end","data":{"outcome":"rejected"}},
			{"id":"e4","source":"network_process","target":"security_process","data":{"outcome":"approved"}},
			{"id":"e5","source":"network_process","target":"end","data":{"outcome":"rejected"}},
			{"id":"e6","source":"security_process","target":"end","data":{"outcome":"approved"}},
			{"id":"e7","source":"security_process","target":"end","data":{"outcome":"rejected"}}
		]
	}`
}

func seedWorkflowParticipantOrgData(t *testing.T, db *gorm.DB, mode string) {
	t.Helper()
	dept := orgdomain.Department{Name: "IT", Code: "it", IsActive: true}
	if err := db.Create(&dept).Error; err != nil {
		t.Fatalf("create department: %v", err)
	}
	pos := orgdomain.Position{Name: "Ops Admin", Code: "ops_admin", IsActive: true}
	if err := db.Create(&pos).Error; err != nil {
		t.Fatalf("create position: %v", err)
	}
	if mode == "missing" {
		return
	}
	if mode != "inactive" {
		user := createServiceHealthUser(t, db, "ops_admin_user", true)
		if err := db.Create(&orgdomain.UserPosition{
			UserID:       user.ID,
			DepartmentID: dept.ID,
			PositionID:   pos.ID,
		}).Error; err != nil {
			t.Fatalf("create user position: %v", err)
		}
		return
	}
	user := createServiceHealthUser(t, db, "ops_admin_disabled", false)
	if err := db.Model(&user).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate user: %v", err)
	}
	if err := db.Create(&orgdomain.UserPosition{
		UserID:       user.ID,
		DepartmentID: dept.ID,
		PositionID:   pos.ID,
	}).Error; err != nil {
		t.Fatalf("create inactive user position: %v", err)
	}
}

func seedServerAccessParticipantOrgData(t *testing.T, db *gorm.DB) {
	t.Helper()
	dept := orgdomain.Department{Name: "IT", Code: "it", IsActive: true}
	if err := db.Create(&dept).Error; err != nil {
		t.Fatalf("create department: %v", err)
	}
	positions := []struct {
		code     string
		username string
	}{
		{code: "ops_admin", username: "ops_admin_user"},
		{code: "network_admin", username: "network_admin_user"},
		{code: "security_admin", username: "security_admin_user"},
	}
	for _, item := range positions {
		pos := orgdomain.Position{Name: item.code, Code: item.code, IsActive: true}
		if err := db.Create(&pos).Error; err != nil {
			t.Fatalf("create position %s: %v", item.code, err)
		}
		user := createServiceHealthUser(t, db, item.username, true)
		if err := db.Create(&orgdomain.UserPosition{
			UserID:       user.ID,
			DepartmentID: dept.ID,
			PositionID:   pos.ID,
			IsPrimary:    true,
		}).Error; err != nil {
			t.Fatalf("create user position %s: %v", item.code, err)
		}
	}
}

func serviceHealthIntakeFormSchema() JSONField {
	return JSONField(`{"version":1,"fields":[{"key":"summary","type":"text","label":"Summary","required":true}]}`)
}

func serverAccessIntakeFormSchema() JSONField {
	return JSONField(`{"version":1,"fields":[{"key":"server","type":"text","label":"访问服务器","required":true},{"key":"access_time","type":"datetime_range","label":"访问时段","required":true},{"key":"purpose","type":"textarea","label":"操作目的","required":true},{"key":"reason","type":"textarea","label":"访问原因","required":true}]}`)
}

func serviceHealthHasItem(check *ServiceHealthCheck, key string, status string) bool {
	for _, item := range check.Items {
		if item.Key == key && item.Status == status {
			return true
		}
	}
	return false
}

func serviceHealthHasLocationKind(check *ServiceHealthCheck, kind string) bool {
	for _, item := range check.Items {
		if item.Location.Kind == kind {
			return true
		}
	}
	return false
}

func serviceHealthHasMessageContaining(check *ServiceHealthCheck, key string, snippet string) bool {
	for _, item := range check.Items {
		if item.Key == key && strings.Contains(item.Message, snippet) {
			return true
		}
	}
	return false
}
