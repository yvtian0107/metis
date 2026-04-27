package runtime

import (
	"encoding/json"
	"errors"
	"testing"

	"metis/internal/database"
	"metis/internal/model"
)

func newToolRuntimeServiceForTest(t *testing.T, db *database.DB) *ToolRuntimeService {
	t.Helper()
	return &ToolRuntimeService{
		toolRepo:    &ToolRepo{db: db},
		modelRepo:   &ModelRepo{db: db},
		providerSvc: &ProviderService{repo: &ProviderRepo{db: db}, encKey: newTestEncryptionKey(t)},
	}
}

func seedServiceMatchRuntimeTargets(t *testing.T, db *database.DB) (Tool, AIModel) {
	t.Helper()
	provider := Provider{Name: "OpenAI", Type: ProviderTypeOpenAI, BaseURL: "https://example.test", Status: ProviderStatusActive}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	aiModel := AIModel{ProviderID: provider.ID, ModelID: "gpt-test", DisplayName: "GPT Test", Type: ModelTypeLLM, Status: ModelStatusActive}
	if err := db.Create(&aiModel).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	tool := Tool{
		Toolkit:             "itsm",
		Name:                "itsm.service_match",
		DisplayName:         "服务匹配",
		ParametersSchema:    model.JSONText("{}"),
		RuntimeConfigSchema: model.JSONText(`{"type":"object","kind":"llm"}`),
		RuntimeConfig:       model.JSONText(`{"modelId":0,"temperature":0.2,"maxTokens":1024,"timeoutSeconds":30}`),
		IsActive:            true,
	}
	if err := db.Create(&tool).Error; err != nil {
		t.Fatalf("create tool: %v", err)
	}
	return tool, aiModel
}

func TestToolRuntimeServiceUpdatesAndBuildsServiceMatchRuntime(t *testing.T) {
	rawDB := setupTestDB(t)
	db := &database.DB{DB: rawDB}
	tool, aiModel := seedServiceMatchRuntimeTargets(t, db)
	svc := newToolRuntimeServiceForTest(t, db)

	resp, err := svc.UpdateRuntimeConfig(tool.ID, json.RawMessage(`{"modelId":`+jsonNumber(aiModel.ID)+`,"temperature":0.15,"maxTokens":768,"timeoutSeconds":35}`))
	if err != nil {
		t.Fatalf("update runtime: %v", err)
	}
	if resp.RuntimeConfig == nil {
		t.Fatal("expected runtime config in response")
	}

	cfg, err := svc.LLMRuntimeConfig("itsm.service_match")
	if err != nil {
		t.Fatalf("load LLM runtime: %v", err)
	}
	if cfg.Model != "gpt-test" || cfg.Temperature != 0.15 || cfg.MaxTokens != 768 || cfg.TimeoutSeconds != 35 {
		t.Fatalf("unexpected runtime config: %+v", cfg)
	}
}

func TestToolRuntimeServiceRejectsUnconfiguredServiceMatchRuntime(t *testing.T) {
	rawDB := setupTestDB(t)
	db := &database.DB{DB: rawDB}
	_, _ = seedServiceMatchRuntimeTargets(t, db)
	svc := newToolRuntimeServiceForTest(t, db)

	if _, err := svc.LLMRuntimeConfig("itsm.service_match"); !errors.Is(err, ErrToolRuntimeNotConfigured) {
		t.Fatalf("expected ErrToolRuntimeNotConfigured, got %v", err)
	}
}

func jsonNumber(id uint) string {
	b, _ := json.Marshal(id)
	return string(b)
}
