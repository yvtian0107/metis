//go:build dev

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/config"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/pkg/crypto"
	"metis/internal/pkg/token"
	"metis/internal/seed"
)

func newDevBootstrapTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&ai.Provider{}, &ai.AIModel{}, &ai.Agent{}, &model.SystemConfig{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}

func writeDevEnv(t *testing.T, dir string, content string) string {
	t.Helper()
	path := filepath.Join(dir, ".env.dev")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env.dev: %v", err)
	}
	return path
}

func TestLoadDevAIConfigNoEnvIsNoop(t *testing.T) {
	cfg, ok, err := loadDevAIConfig(filepath.Join(t.TempDir(), ".env.dev"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if ok {
		t.Fatalf("expected no config, got %+v", cfg)
	}
}

func TestLoadDevAIConfigRequiresNameURLAndKey(t *testing.T) {
	path := writeDevEnv(t, t.TempDir(), `
METIS_DEV_AI_PROVIDER_NAME=Dev OpenAI
METIS_DEV_AI_BASE_URL=https://api.example.com/v1
`)
	_, _, err := loadDevAIConfig(path)
	if err == nil {
		t.Fatal("expected missing api key error")
	}
}

func TestRunDevBootstrapUpsertsProviderModelAgentsAndITSMConfig(t *testing.T) {
	db := newDevBootstrapTestDB(t)
	for _, code := range []string{"itsm.servicedesk", "itsm.decision", "itsm.sla_assurance"} {
		agentCode := code
		agent := ai.Agent{
			Name:      code,
			Code:      &agentCode,
			Type:      ai.AgentTypeAssistant,
			IsActive:  true,
			CreatedBy: 1,
		}
		if err := db.Create(&agent).Error; err != nil {
			t.Fatalf("seed agent %s: %v", code, err)
		}
	}

	envPath := writeDevEnv(t, t.TempDir(), `
METIS_DEV_AI_PROVIDER_NAME=Dev OpenAI
METIS_DEV_AI_BASE_URL=https://api.example.com/v1
METIS_DEV_AI_API_KEY=sk-old
`)
	cfg := &config.MetisConfig{SecretKey: "test-secret"}
	if err := runDevBootstrap(db, cfg, envPath); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}

	if err := os.WriteFile(envPath, []byte(`
METIS_DEV_AI_PROVIDER_NAME=Dev OpenAI
METIS_DEV_AI_BASE_URL=https://proxy.example.com/v1
METIS_DEV_AI_API_KEY=sk-new
`), 0o600); err != nil {
		t.Fatalf("rewrite .env.dev: %v", err)
	}
	if err := runDevBootstrap(db, cfg, envPath); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}

	var providerCount int64
	if err := db.Model(&ai.Provider{}).Where("name = ?", "Dev OpenAI").Count(&providerCount).Error; err != nil {
		t.Fatalf("count providers: %v", err)
	}
	if providerCount != 1 {
		t.Fatalf("provider count = %d, want 1", providerCount)
	}

	var provider ai.Provider
	if err := db.Where("name = ?", "Dev OpenAI").First(&provider).Error; err != nil {
		t.Fatalf("load provider: %v", err)
	}
	if provider.BaseURL != "https://proxy.example.com/v1" || provider.Status != ai.ProviderStatusActive {
		t.Fatalf("provider not updated: %+v", provider)
	}
	plain, err := crypto.Decrypt(provider.APIKeyEncrypted, crypto.EncryptionKey(crypto.DeriveKey(cfg.SecretKey)))
	if err != nil {
		t.Fatalf("decrypt api key: %v", err)
	}
	if string(plain) != "sk-new" {
		t.Fatalf("api key = %q, want sk-new", plain)
	}

	var modelCount int64
	if err := db.Model(&ai.AIModel{}).Where("provider_id = ? AND model_id = ?", provider.ID, "gpt-4o").Count(&modelCount).Error; err != nil {
		t.Fatalf("count models: %v", err)
	}
	if modelCount != 1 {
		t.Fatalf("model count = %d, want 1", modelCount)
	}

	var llm ai.AIModel
	if err := db.Where("provider_id = ? AND model_id = ?", provider.ID, "gpt-4o").First(&llm).Error; err != nil {
		t.Fatalf("load model: %v", err)
	}
	if !llm.IsDefault || llm.Type != ai.ModelTypeLLM || llm.Status != ai.ModelStatusActive || string(llm.Capabilities) != `["tool_use"]` {
		t.Fatalf("model not initialized as expected: %+v", llm)
	}

	for _, code := range []string{"itsm.servicedesk", "itsm.decision", "itsm.sla_assurance"} {
		var agent ai.Agent
		if err := db.Where("code = ?", code).First(&agent).Error; err != nil {
			t.Fatalf("load agent %s: %v", code, err)
		}
		if agent.ModelID == nil || *agent.ModelID != llm.ID {
			t.Fatalf("agent %s model_id = %v, want %d", code, agent.ModelID, llm.ID)
		}
	}

	expectedConfig := map[string]string{
		"itsm.smart_ticket.intake.agent_id":                 "1",
		"itsm.smart_ticket.decision.agent_id":               "2",
		"itsm.smart_ticket.sla_assurance.agent_id":          "3",
		"itsm.smart_ticket.service_matcher.model_id":        "1",
		"itsm.smart_ticket.path.model_id":                   "1",
		"itsm.smart_ticket.service_matcher.temperature":     "0.2",
		"itsm.smart_ticket.service_matcher.max_tokens":      "1024",
		"itsm.smart_ticket.service_matcher.timeout_seconds": "30",
		"itsm.smart_ticket.path.temperature":                "0.3",
		"itsm.smart_ticket.path.max_retries":                "1",
		"itsm.smart_ticket.path.timeout_seconds":            "60",
		"itsm.smart_ticket.decision.mode":                   "direct_first",
	}
	for key, want := range expectedConfig {
		var cfg model.SystemConfig
		if err := db.Where("\"key\" = ?", key).First(&cfg).Error; err != nil {
			t.Fatalf("load config %s: %v", key, err)
		}
		if cfg.Value != want {
			t.Fatalf("config %s = %q, want %q", key, cfg.Value, want)
		}
	}
}

func TestRunSeedDevRequiresEnvDev(t *testing.T) {
	t.Chdir(t.TempDir())
	err := runSeedDev("config.yml", ".env.dev")
	if err == nil {
		t.Fatal("expected missing .env.dev error")
	}
}

func TestRunSeedDevInstallsSystemAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeDevEnv(t, dir, `
METIS_DEV_AI_PROVIDER_NAME=Dev OpenAI
METIS_DEV_AI_BASE_URL=https://api.example.com/v1
METIS_DEV_AI_API_KEY=sk-dev
`)

	if err := runSeedDev("config.yml", ".env.dev"); err != nil {
		t.Fatalf("first seed-dev: %v", err)
	}
	if err := runSeedDev("config.yml", ".env.dev"); err != nil {
		t.Fatalf("second seed-dev: %v", err)
	}

	cfg, err := config.Load("config.yml")
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	db, err := database.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		t.Fatalf("open generated db: %v", err)
	}
	defer db.Shutdown()

	if !seed.IsInstalled(db.DB) {
		t.Fatal("expected app.installed=true")
	}

	var admin model.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}
	if !admin.IsActive || !token.CheckPassword(admin.Password, "password") {
		t.Fatalf("admin not active or password mismatch: active=%v", admin.IsActive)
	}
	var adminCount int64
	if err := db.Model(&model.User{}).Where("username = ?", "admin").Count(&adminCount).Error; err != nil {
		t.Fatalf("count admin: %v", err)
	}
	if adminCount != 1 {
		t.Fatalf("admin count = %d, want 1", adminCount)
	}
	var fallbackCfg model.SystemConfig
	if err := db.Where("\"key\" = ?", "itsm.smart_ticket.guard.fallback_assignee").First(&fallbackCfg).Error; err != nil {
		t.Fatalf("load ITSM fallback assignee config: %v", err)
	}
	if fallbackCfg.Value != strconv.FormatUint(uint64(admin.ID), 10) {
		t.Fatalf("fallback assignee = %q, want admin id %d", fallbackCfg.Value, admin.ID)
	}

	for _, key := range []string{
		"itsm.smart_ticket.intake.agent_id",
		"itsm.smart_ticket.decision.agent_id",
		"itsm.smart_ticket.sla_assurance.agent_id",
		"itsm.smart_ticket.service_matcher.model_id",
		"itsm.smart_ticket.path.model_id",
	} {
		var cfg model.SystemConfig
		if err := db.Where("\"key\" = ?", key).First(&cfg).Error; err != nil {
			t.Fatalf("load config %s: %v", key, err)
		}
		if cfg.Value == "" || cfg.Value == "0" {
			t.Fatalf("config %s was not initialized: %q", key, cfg.Value)
		}
	}

	var providerCount int64
	if err := db.Model(&ai.Provider{}).Where("name = ?", "Dev OpenAI").Count(&providerCount).Error; err != nil {
		t.Fatalf("count dev providers: %v", err)
	}
	if providerCount != 1 {
		t.Fatalf("provider count = %d, want 1", providerCount)
	}

	var positionCount int64
	if err := db.Table("user_positions").Where("user_id = ?", admin.ID).Count(&positionCount).Error; err != nil {
		t.Fatalf("count admin positions: %v", err)
	}
	if positionCount < 1 {
		t.Fatal("expected admin org identities to be assigned")
	}
}

func TestRunSeedDevUsesExplicitEnvPath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	envPath := filepath.Join(dir, "custom.dev.env")
	if err := os.WriteFile(envPath, []byte(`
METIS_DEV_AI_PROVIDER_NAME=Custom Dev Provider
METIS_DEV_AI_BASE_URL=https://api.example.com/v1
METIS_DEV_AI_API_KEY=sk-custom-dev
`), 0600); err != nil {
		t.Fatalf("write custom env: %v", err)
	}

	if err := runSeedDev("config.yml", envPath); err != nil {
		t.Fatalf("seed-dev with explicit env: %v", err)
	}

	cfg, err := config.Load("config.yml")
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	db, err := database.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		t.Fatalf("open generated db: %v", err)
	}
	defer db.Shutdown()

	var providerCount int64
	if err := db.Model(&ai.Provider{}).Where("name = ?", "Custom Dev Provider").Count(&providerCount).Error; err != nil {
		t.Fatalf("count custom dev providers: %v", err)
	}
	if providerCount != 1 {
		t.Fatalf("provider count = %d, want 1", providerCount)
	}
}

func TestMaybeRunSeedDevAutoInstallsWhenEnvDevExists(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeDevEnv(t, dir, `
METIS_DEV_AI_PROVIDER_NAME=Dev OpenAI
METIS_DEV_AI_BASE_URL=https://api.example.com/v1
METIS_DEV_AI_API_KEY=sk-dev
`)

	ran, err := maybeRunSeedDev("config.yml", ".env.dev", nil)
	if err != nil {
		t.Fatalf("maybe seed-dev: %v", err)
	}
	if !ran {
		t.Fatal("expected seed-dev to run when .env.dev exists and config is missing")
	}

	cfg, err := config.Load("config.yml")
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	db, err := database.Open(cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		t.Fatalf("open generated db: %v", err)
	}
	defer db.Shutdown()
	if !seed.IsInstalled(db.DB) {
		t.Fatal("expected auto seed-dev to mark system installed")
	}
}
