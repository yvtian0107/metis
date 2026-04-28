package tools

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	aiapp "metis/internal/app/ai/runtime"
	"metis/internal/model"
)

func setupSeedAgentsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&aiapp.Provider{},
		&aiapp.AIModel{},
		&aiapp.Agent{},
		&aiapp.Tool{},
		&aiapp.AgentTool{},
	); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}

func TestSeedAgentsCreatesPresetAgentsWithDefaultModelAndBindings(t *testing.T) {
	db := setupSeedAgentsTestDB(t)

	provider := aiapp.Provider{Name: "Test Provider", Type: "openai", Protocol: "openai", BaseURL: "https://example.com", Status: "active"}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	defaultLLM := aiapp.AIModel{
		ModelID:     "gpt-test",
		DisplayName: "GPT Test",
		ProviderID:  provider.ID,
		Type:        "llm",
		IsDefault:   true,
		Status:      "active",
	}
	if err := db.Create(&defaultLLM).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	for _, toolName := range []string{"itsm.service_match", "general.current_time"} {
		if err := db.Create(&aiapp.Tool{Name: toolName, DisplayName: toolName, ParametersSchema: model.JSONText("{}")}).Error; err != nil {
			t.Fatalf("create tool %s: %v", toolName, err)
		}
	}

	if err := SeedAgents(db); err != nil {
		t.Fatalf("seed agents: %v", err)
	}

	var got aiapp.Agent
	if err := db.Where("code = ?", "itsm.servicedesk").First(&got).Error; err != nil {
		t.Fatalf("load service desk agent: %v", err)
	}
	if got.ModelID == nil || *got.ModelID != defaultLLM.ID {
		t.Fatalf("expected default model %d, got %v", defaultLLM.ID, got.ModelID)
	}
	if got.SystemPrompt == "" {
		t.Fatal("expected default system prompt to be seeded")
	}

	var toolCount int64
	if err := db.Table("ai_agent_tools").Where("agent_id = ?", got.ID).Count(&toolCount).Error; err != nil {
		t.Fatalf("count tool bindings: %v", err)
	}
	if toolCount != 2 {
		t.Fatalf("expected 2 seeded tool bindings, got %d", toolCount)
	}
}

func TestSeedAgentsKeepsExistingPresetAgentConfiguration(t *testing.T) {
	db := setupSeedAgentsTestDB(t)

	provider := aiapp.Provider{Name: "Test Provider", Type: "openai", Protocol: "openai", BaseURL: "https://example.com", Status: "active"}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	defaultModel := aiapp.AIModel{
		ModelID:     "gpt-default",
		DisplayName: "GPT Default",
		ProviderID:  provider.ID,
		Type:        "llm",
		IsDefault:   true,
		Status:      "active",
	}
	if err := db.Create(&defaultModel).Error; err != nil {
		t.Fatalf("create default model: %v", err)
	}
	userModel := aiapp.AIModel{
		ModelID:     "gpt-user",
		DisplayName: "GPT User",
		ProviderID:  provider.ID,
		Type:        "llm",
		Status:      "active",
	}
	if err := db.Create(&userModel).Error; err != nil {
		t.Fatalf("create user model: %v", err)
	}

	customTool := aiapp.Tool{Name: "custom.tool", DisplayName: "Custom Tool", ParametersSchema: model.JSONText("{}")}
	defaultTool := aiapp.Tool{Name: "itsm.service_match", DisplayName: "itsm.service_match", ParametersSchema: model.JSONText("{}")}
	if err := db.Create(&customTool).Error; err != nil {
		t.Fatalf("create custom tool: %v", err)
	}
	if err := db.Create(&defaultTool).Error; err != nil {
		t.Fatalf("create default tool: %v", err)
	}

	code := "itsm.servicedesk"
	userModelID := userModel.ID
	agent := aiapp.Agent{
		Name:         "IT éˆå¶…å§Ÿé™ç‰ˆæ«¤é‘³æˆ’ç¶‹",
		Code:         &code,
		Type:         aiapp.AgentTypeAssistant,
		Visibility:   aiapp.AgentVisibilityPublic,
		SystemPrompt: "user customized prompt",
		Temperature:  0.91,
		MaxTokens:    777,
		MaxTurns:     5,
		ModelID:      &userModelID,
		IsActive:     false,
		CreatedBy:    9,
	}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create preset agent: %v", err)
	}
	if err := db.Model(&agent).Update("is_active", false).Error; err != nil {
		t.Fatalf("mark preset agent inactive: %v", err)
	}
	if err := db.Create(&aiapp.AgentTool{AgentID: agent.ID, ToolID: customTool.ID}).Error; err != nil {
		t.Fatalf("create custom binding: %v", err)
	}

	if err := SeedAgents(db); err != nil {
		t.Fatalf("seed agents: %v", err)
	}

	var got aiapp.Agent
	if err := db.Where("code = ?", code).First(&got).Error; err != nil {
		t.Fatalf("reload preset agent: %v", err)
	}
	if got.ModelID == nil || *got.ModelID != userModel.ID {
		t.Fatalf("expected model to stay %d, got %v", userModel.ID, got.ModelID)
	}
	if got.SystemPrompt != "user customized prompt" {
		t.Fatalf("expected prompt to remain customized, got %q", got.SystemPrompt)
	}
	if got.Temperature != 0.91 || got.MaxTokens != 777 || got.MaxTurns != 5 {
		t.Fatalf("expected numeric config to remain customized, got temp=%v maxTokens=%d maxTurns=%d", got.Temperature, got.MaxTokens, got.MaxTurns)
	}
	if got.IsActive {
		t.Fatal("expected active flag to remain user-configured")
	}

	var bindings []aiapp.AgentTool
	if err := db.Where("agent_id = ?", agent.ID).Find(&bindings).Error; err != nil {
		t.Fatalf("load bindings: %v", err)
	}
	if len(bindings) != 1 || bindings[0].ToolID != customTool.ID {
		t.Fatalf("expected custom tool binding to remain untouched, got %+v", bindings)
	}
}
