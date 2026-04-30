package config

import (
	"errors"
	"strconv"
	"testing"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	aiapp "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/prompts"
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
		SmartTicketIntakeAgentKey:              "0",
		SmartTicketDecisionAgentKey:            "0",
		SmartTicketSLAAssuranceAgentKey:        "0",
		SmartTicketDecisionModeKey:             "direct_first",
		SmartTicketPathModelKey:                "0",
		SmartTicketPathTemperatureKey:          "0.3",
		SmartTicketPathMaxRetriesKey:           "1",
		SmartTicketPathTimeoutKey:              "60",
		SmartTicketPathSystemPromptKey:         prompts.PathBuilderSystemPromptDefault,
		SmartTicketSessionTitleModelKey:        "0",
		SmartTicketSessionTitleTemperatureKey:  "0.2",
		SmartTicketSessionTitleMaxRetriesKey:   "1",
		SmartTicketSessionTitleTimeoutKey:      "30",
		SmartTicketSessionTitlePromptKey:       SessionTitleSystemPromptDefault,
		SmartTicketPublishHealthModelKey:       "0",
		SmartTicketPublishHealthTemperatureKey: "0.2",
		SmartTicketPublishHealthMaxRetriesKey:  "1",
		SmartTicketPublishHealthTimeoutKey:     "45",
		SmartTicketPublishHealthPromptKey:      prompts.PublishHealthSystemPromptDefault,
		SmartTicketGuardAuditLevelKey:          "full",
		SmartTicketGuardFallbackKey:            "0",
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
	engineReq.Runtime.PathBuilder.SystemPrompt = "path prompt"
	engineReq.Runtime.TitleBuilder.ModelID = model.ID
	engineReq.Runtime.TitleBuilder.Temperature = 0.15
	engineReq.Runtime.TitleBuilder.MaxRetries = 2
	engineReq.Runtime.TitleBuilder.TimeoutSeconds = 45
	engineReq.Runtime.TitleBuilder.SystemPrompt = "title prompt"
	engineReq.Runtime.HealthChecker.ModelID = model.ID
	engineReq.Runtime.HealthChecker.Temperature = 0.2
	engineReq.Runtime.HealthChecker.MaxRetries = 1
	engineReq.Runtime.HealthChecker.TimeoutSeconds = 55
	engineReq.Runtime.HealthChecker.SystemPrompt = "health prompt"
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
	if engineSettings.Runtime.PathBuilder.SystemPrompt != "path prompt" {
		t.Fatalf("unexpected path prompt: %q", engineSettings.Runtime.PathBuilder.SystemPrompt)
	}
	if engineSettings.Runtime.TitleBuilder.ModelID != model.ID || engineSettings.Runtime.TitleBuilder.ProviderID != provider.ID || engineSettings.Runtime.TitleBuilder.MaxRetries != 2 || engineSettings.Runtime.TitleBuilder.TimeoutSeconds != 45 {
		t.Fatalf("unexpected title builder config: %+v", engineSettings.Runtime.TitleBuilder)
	}
	if engineSettings.Runtime.TitleBuilder.SystemPrompt != "title prompt" {
		t.Fatalf("unexpected title builder prompt: %q", engineSettings.Runtime.TitleBuilder.SystemPrompt)
	}
	if engineSettings.Runtime.HealthChecker.ModelID != model.ID || engineSettings.Runtime.HealthChecker.ProviderID != provider.ID || engineSettings.Runtime.HealthChecker.MaxRetries != 1 || engineSettings.Runtime.HealthChecker.TimeoutSeconds != 55 {
		t.Fatalf("unexpected health checker config: %+v", engineSettings.Runtime.HealthChecker)
	}
	if engineSettings.Runtime.HealthChecker.SystemPrompt != "health prompt" {
		t.Fatalf("unexpected health checker prompt: %q", engineSettings.Runtime.HealthChecker.SystemPrompt)
	}
	if engineSettings.Runtime.Guard.AuditLevel != "summary" || engineSettings.Runtime.Guard.FallbackAssignee != fallback.ID {
		t.Fatalf("unexpected guard config: %+v", engineSettings.Runtime.Guard)
	}

	expectedKeys := map[string]string{
		SmartTicketIntakeAgentKey:              strconv.FormatUint(uint64(intakeAgent.ID), 10),
		SmartTicketDecisionAgentKey:            strconv.FormatUint(uint64(decisionAgent.ID), 10),
		SmartTicketSLAAssuranceAgentKey:        strconv.FormatUint(uint64(slaAssuranceAgent.ID), 10),
		SmartTicketDecisionModeKey:             "ai_only",
		SmartTicketPathModelKey:                strconv.FormatUint(uint64(model.ID), 10),
		SmartTicketPathTemperatureKey:          "0.25",
		SmartTicketPathMaxRetriesKey:           "4",
		SmartTicketPathTimeoutKey:              "90",
		SmartTicketPathSystemPromptKey:         "path prompt",
		SmartTicketSessionTitleModelKey:        strconv.FormatUint(uint64(model.ID), 10),
		SmartTicketSessionTitleTemperatureKey:  "0.15",
		SmartTicketSessionTitleMaxRetriesKey:   "2",
		SmartTicketSessionTitleTimeoutKey:      "45",
		SmartTicketSessionTitlePromptKey:       "title prompt",
		SmartTicketPublishHealthModelKey:       strconv.FormatUint(uint64(model.ID), 10),
		SmartTicketPublishHealthTemperatureKey: "0.2",
		SmartTicketPublishHealthMaxRetriesKey:  "1",
		SmartTicketPublishHealthTimeoutKey:     "55",
		SmartTicketPublishHealthPromptKey:      "health prompt",
		SmartTicketGuardAuditLevelKey:          "summary",
		SmartTicketGuardFallbackKey:            strconv.FormatUint(uint64(fallback.ID), 10),
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
	req.Runtime.PathBuilder.SystemPrompt = "path prompt"
	req.Runtime.TitleBuilder.MaxRetries = 1
	req.Runtime.TitleBuilder.TimeoutSeconds = 30
	req.Runtime.TitleBuilder.SystemPrompt = "title prompt"
	req.Runtime.HealthChecker.MaxRetries = 1
	req.Runtime.HealthChecker.TimeoutSeconds = 45
	req.Runtime.HealthChecker.SystemPrompt = "health prompt"
	req.Runtime.Guard.AuditLevel = "full"
	req.Runtime.Guard.FallbackAssignee = 999
	if err := svc.UpdateEngineSettingsConfig(&req); err != ErrFallbackUserNotFound {
		t.Fatalf("expected ErrFallbackUserNotFound, got %v", err)
	}
}

func TestEngineConfigServiceSmartStaffingUpdateRollsBackOnWriteFailure(t *testing.T) {
	svc, db := newEngineConfigTestService(t)

	var intakeAgent aiapp.Agent
	if err := db.Where("code = ?", "itsm.servicedesk").First(&intakeAgent).Error; err != nil {
		t.Fatalf("load intake agent: %v", err)
	}
	var decisionAgent aiapp.Agent
	if err := db.Where("code = ?", "itsm.decision").First(&decisionAgent).Error; err != nil {
		t.Fatalf("load decision agent: %v", err)
	}

	if err := db.Exec(`CREATE TRIGGER reject_decision_agent_config_update
BEFORE UPDATE ON system_configs
WHEN NEW.key = '` + SmartTicketDecisionAgentKey + `'
BEGIN
	SELECT RAISE(FAIL, 'reject decision agent config');
END;`).Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	var req UpdateSmartStaffingRequest
	req.Posts.Intake.AgentID = intakeAgent.ID
	req.Posts.Decision.AgentID = decisionAgent.ID
	req.Posts.Decision.Mode = "direct_first"

	err := svc.UpdateSmartStaffingConfig(&req)
	if err == nil {
		t.Fatalf("expected write failure")
	}
	if got := svc.IntakeAgentID(); got != 0 {
		t.Fatalf("expected intake agent rollback to 0, got %d", got)
	}
}

func TestEngineConfigServiceEngineSettingsUpdateRollsBackOnWriteFailure(t *testing.T) {
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

	if err := db.Exec(`CREATE TRIGGER reject_path_temperature_config_update
BEFORE UPDATE ON system_configs
WHEN NEW.key = '` + SmartTicketPathTemperatureKey + `'
BEGIN
	SELECT RAISE(FAIL, 'reject path temperature config');
END;`).Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	var req UpdateEngineSettingsRequest
	req.Runtime.PathBuilder.ModelID = model.ID
	req.Runtime.PathBuilder.Temperature = 0.25
	req.Runtime.PathBuilder.MaxRetries = 4
	req.Runtime.PathBuilder.TimeoutSeconds = 90
	req.Runtime.PathBuilder.SystemPrompt = "path prompt"
	req.Runtime.TitleBuilder.ModelID = model.ID
	req.Runtime.TitleBuilder.Temperature = 0.15
	req.Runtime.TitleBuilder.MaxRetries = 2
	req.Runtime.TitleBuilder.TimeoutSeconds = 45
	req.Runtime.TitleBuilder.SystemPrompt = "title prompt"
	req.Runtime.HealthChecker.ModelID = model.ID
	req.Runtime.HealthChecker.Temperature = 0.2
	req.Runtime.HealthChecker.MaxRetries = 1
	req.Runtime.HealthChecker.TimeoutSeconds = 55
	req.Runtime.HealthChecker.SystemPrompt = "health prompt"
	req.Runtime.Guard.AuditLevel = "summary"

	err := svc.UpdateEngineSettingsConfig(&req)
	if err == nil {
		t.Fatalf("expected write failure")
	}
	if got := svc.readPathConfig().ModelID; got != 0 {
		t.Fatalf("expected path model rollback to 0, got %d", got)
	}
}

func TestEngineConfigServiceRuntimeConfigRequiresDBPrompt(t *testing.T) {
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
	var req UpdateEngineSettingsRequest
	req.Runtime.PathBuilder.ModelID = model.ID
	req.Runtime.PathBuilder.Temperature = 0.2
	req.Runtime.PathBuilder.MaxRetries = 1
	req.Runtime.PathBuilder.TimeoutSeconds = 60
	req.Runtime.PathBuilder.SystemPrompt = "path prompt"
	req.Runtime.TitleBuilder.ModelID = model.ID
	req.Runtime.TitleBuilder.Temperature = 0.2
	req.Runtime.TitleBuilder.MaxRetries = 1
	req.Runtime.TitleBuilder.TimeoutSeconds = 30
	req.Runtime.TitleBuilder.SystemPrompt = "title prompt"
	req.Runtime.HealthChecker.ModelID = model.ID
	req.Runtime.HealthChecker.Temperature = 0.2
	req.Runtime.HealthChecker.MaxRetries = 1
	req.Runtime.HealthChecker.TimeoutSeconds = 45
	req.Runtime.HealthChecker.SystemPrompt = "health prompt"
	req.Runtime.Guard.AuditLevel = "full"
	req.Runtime.Guard.FallbackAssignee = 0
	if err := svc.UpdateEngineSettingsConfig(&req); err != nil {
		t.Fatalf("update engine settings: %v", err)
	}

	if err := db.Model(&coremodel.SystemConfig{}).Where("\"key\" = ?", SmartTicketPathSystemPromptKey).Update("value", "").Error; err != nil {
		t.Fatalf("clear path system prompt: %v", err)
	}
	_, err := svc.PathBuilderRuntimeConfig()
	if err == nil || !errors.Is(err, ErrEngineNotConfigured) {
		t.Fatalf("expected ErrEngineNotConfigured for path builder prompt, got %v", err)
	}

	if err := db.Model(&coremodel.SystemConfig{}).Where("\"key\" = ?", SmartTicketSessionTitlePromptKey).Update("value", "").Error; err != nil {
		t.Fatalf("clear title system prompt: %v", err)
	}
	_, err = svc.SessionTitleRuntimeConfig()
	if err == nil || !errors.Is(err, ErrEngineNotConfigured) {
		t.Fatalf("expected ErrEngineNotConfigured for title builder prompt, got %v", err)
	}

	if err := db.Model(&coremodel.SystemConfig{}).Where("\"key\" = ?", SmartTicketPublishHealthPromptKey).Update("value", "").Error; err != nil {
		t.Fatalf("clear health checker system prompt: %v", err)
	}
	_, err = svc.HealthCheckRuntimeConfig()
	if err == nil || !errors.Is(err, ErrEngineNotConfigured) {
		t.Fatalf("expected ErrEngineNotConfigured for health checker prompt, got %v", err)
	}
}
