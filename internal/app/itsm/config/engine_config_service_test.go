package config

import (
	"strconv"
	"testing"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	aiapp "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/testutil"
	"metis/internal/app/itsm/tools"
	"metis/internal/database"
	coremodel "metis/internal/model"
	"metis/internal/pkg/crypto"
	"metis/internal/repository"
)

func newEngineConfigTestService(t *testing.T) (*EngineConfigService, *database.DB) {
	t.Helper()
	db := testutil.NewTestDB(t)
	if err := tools.SeedTools(db); err != nil {
		t.Fatalf("seed tools: %v", err)
	}
	if err := tools.SeedAgents(db); err != nil {
		t.Fatalf("seed agents: %v", err)
	}
	if err := seedEngineConfigForTest(db); err != nil {
		t.Fatalf("seed engine config: %v", err)
	}

	injector := do.New()
	do.ProvideValue(injector, &database.DB{DB: db})
	do.Provide(injector, repository.NewSysConfig)
	do.Provide(injector, aiapp.NewAgentRepo)
	do.Provide(injector, aiapp.NewAgentService)
	do.Provide(injector, aiapp.NewModelRepo)
	do.Provide(injector, aiapp.NewProviderRepo)
	do.ProvideValue(injector, crypto.EncryptionKey(crypto.DeriveKey("test-secret")))
	do.Provide(injector, aiapp.NewProviderService)
	do.Provide(injector, aiapp.NewToolRepo)
	do.Provide(injector, aiapp.NewToolRuntimeService)
	do.Provide(injector, NewEngineConfigService)
	return do.MustInvoke[*EngineConfigService](injector), &database.DB{DB: db}
}

func seedEngineConfigForTest(db *gorm.DB) error {
	defaults := map[string]string{
		SmartTicketIntakeAgentKey:       "0",
		SmartTicketDecisionAgentKey:     "0",
		SmartTicketSLAAssuranceAgentKey: "0",
		SmartTicketDecisionModeKey:      "direct_first",
		SmartTicketPathModelKey:         "0",
		SmartTicketPathTemperatureKey:   "0.3",
		SmartTicketPathMaxRetriesKey:    "1",
		SmartTicketPathTimeoutKey:       "60",
		SmartTicketGuardAuditLevelKey:   "full",
		SmartTicketGuardFallbackKey:     "0",
	}
	for key, value := range defaults {
		if err := db.FirstOrCreate(&coremodel.SystemConfig{}, coremodel.SystemConfig{Key: key, Value: value}).Error; err != nil {
			return err
		}
	}
	return nil
}

func TestEngineConfigServiceReadsAndUpdatesSmartStaffingAndEngineSettings(t *testing.T) {
	svc, db := newEngineConfigTestService(t)

	provider := aiapp.Provider{
		Name:     "OpenAI",
		Type:     aiapp.ProviderTypeOpenAI,
		Protocol: "openai",
		BaseURL:  "https://example.test",
		Status:   aiapp.ProviderStatusActive,
	}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	model := aiapp.AIModel{
		ProviderID:  provider.ID,
		ModelID:     "gpt-test",
		DisplayName: "GPT Test",
		Type:        aiapp.ModelTypeLLM,
		Status:      aiapp.ModelStatusActive,
	}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	var intakeAgent aiapp.Agent
	if err := db.Where("code = ?", "itsm.servicedesk").First(&intakeAgent).Error; err != nil {
		t.Fatalf("load intake agent: %v", err)
	}
	var decisionAgent aiapp.Agent
	if err := db.Where("code = ?", "itsm.decision").First(&decisionAgent).Error; err != nil {
		t.Fatalf("load decision agent: %v", err)
	}
	var slaAssuranceAgent aiapp.Agent
	if err := db.Where("code = ?", "itsm.sla_assurance").First(&slaAssuranceAgent).Error; err != nil {
		t.Fatalf("load SLA assurance agent: %v", err)
	}
	if err := db.Model(&aiapp.Agent{}).Where("id IN ?", []uint{intakeAgent.ID, decisionAgent.ID, slaAssuranceAgent.ID}).Update("model_id", model.ID).Error; err != nil {
		t.Fatalf("bind staffing agent models: %v", err)
	}
	if err := db.Table("ai_tools").Where("name = ?", "itsm.service_match").Update("runtime_config",
		`{"modelId":`+strconv.FormatUint(uint64(model.ID), 10)+`,"temperature":0.2,"maxTokens":1024,"timeoutSeconds":30}`).Error; err != nil {
		t.Fatalf("configure service match tool runtime: %v", err)
	}

	fallback := coremodel.User{Username: "fallback", IsActive: true}
	if err := db.Create(&fallback).Error; err != nil {
		t.Fatalf("create fallback user: %v", err)
	}

	var staffingReq UpdateSmartStaffingRequest
	staffingReq.Posts.Intake.AgentID = intakeAgent.ID
	staffingReq.Posts.Decision.AgentID = decisionAgent.ID
	staffingReq.Posts.Decision.Mode = "ai_only"
	staffingReq.Posts.SLAAssurance.AgentID = slaAssuranceAgent.ID
	if err := svc.UpdateSmartStaffingConfig(&staffingReq); err != nil {
		t.Fatalf("update smart staffing config: %v", err)
	}

	var engineReq UpdateEngineSettingsRequest
	engineReq.Runtime.PathBuilder.ModelID = model.ID
	engineReq.Runtime.PathBuilder.Temperature = 0.25
	engineReq.Runtime.PathBuilder.MaxRetries = 4
	engineReq.Runtime.PathBuilder.TimeoutSeconds = 90
	engineReq.Runtime.Guard.AuditLevel = "summary"
	engineReq.Runtime.Guard.FallbackAssignee = fallback.ID
	if err := svc.UpdateEngineSettingsConfig(&engineReq); err != nil {
		t.Fatalf("update engine settings config: %v", err)
	}

	staffing, err := svc.GetSmartStaffingConfig()
	if err != nil {
		t.Fatalf("get smart staffing config: %v", err)
	}
	if staffing.Posts.Intake.AgentID != intakeAgent.ID || staffing.Posts.Decision.AgentID != decisionAgent.ID || staffing.Posts.Decision.Mode != "ai_only" || staffing.Posts.SLAAssurance.AgentID != slaAssuranceAgent.ID {
		t.Fatalf("unexpected smart staffing posts: %+v", staffing)
	}
	if len(staffing.Health.Items) != 3 {
		t.Fatalf("expected staffing health to contain only three posts, got %+v", staffing.Health.Items)
	}

	engineSettings, err := svc.GetEngineSettingsConfig()
	if err != nil {
		t.Fatalf("get engine settings config: %v", err)
	}
	if engineSettings.Runtime.PathBuilder.ModelID != model.ID || engineSettings.Runtime.PathBuilder.ProviderID != provider.ID || engineSettings.Runtime.PathBuilder.MaxRetries != 4 || engineSettings.Runtime.PathBuilder.TimeoutSeconds != 90 {
		t.Fatalf("unexpected path config: %+v", engineSettings.Runtime.PathBuilder)
	}
	if engineSettings.Runtime.Guard.AuditLevel != "summary" || engineSettings.Runtime.Guard.FallbackAssignee != fallback.ID {
		t.Fatalf("unexpected guard config: %+v", engineSettings.Runtime.Guard)
	}

	expectedKeys := map[string]string{
		SmartTicketIntakeAgentKey:       strconv.FormatUint(uint64(intakeAgent.ID), 10),
		SmartTicketDecisionAgentKey:     strconv.FormatUint(uint64(decisionAgent.ID), 10),
		SmartTicketSLAAssuranceAgentKey: strconv.FormatUint(uint64(slaAssuranceAgent.ID), 10),
		SmartTicketDecisionModeKey:      "ai_only",
		SmartTicketPathModelKey:         strconv.FormatUint(uint64(model.ID), 10),
		SmartTicketPathTemperatureKey:   "0.25",
		SmartTicketPathMaxRetriesKey:    "4",
		SmartTicketPathTimeoutKey:       "90",
		SmartTicketGuardAuditLevelKey:   "summary",
		SmartTicketGuardFallbackKey:     strconv.FormatUint(uint64(fallback.ID), 10),
	}
	for key, value := range expectedKeys {
		var cfg coremodel.SystemConfig
		if err := db.Where("\"key\" = ?", key).First(&cfg).Error; err != nil {
			t.Fatalf("load system config %s: %v", key, err)
		}
		if cfg.Value != value {
			t.Fatalf("expected %s=%s, got %s", key, value, cfg.Value)
		}
	}
}

func TestEngineConfigServiceRejectsInvalidFallbackAssignee(t *testing.T) {
	svc, _ := newEngineConfigTestService(t)
	var req UpdateEngineSettingsRequest
	req.Runtime.PathBuilder.MaxRetries = 3
	req.Runtime.PathBuilder.TimeoutSeconds = 120
	req.Runtime.Guard.AuditLevel = "full"
	req.Runtime.Guard.FallbackAssignee = 999
	if err := svc.UpdateEngineSettingsConfig(&req); err != ErrFallbackUserNotFound {
		t.Fatalf("expected ErrFallbackUserNotFound, got %v", err)
	}
}
