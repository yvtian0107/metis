package tools

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	aiapp "metis/internal/app/ai/runtime"
)

func TestSeedAgentsUpdatesExistingPresetAgentPrompt(t *testing.T) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&aiapp.Agent{}, &aiapp.Tool{}, &aiapp.AgentTool{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	code := "itsm.servicedesk"
	agent := aiapp.Agent{
		Name:         "旧服务台",
		Code:         &code,
		Type:         aiapp.AgentTypeAssistant,
		Visibility:   aiapp.AgentVisibilityTeam,
		SystemPrompt: "stale prompt",
		Temperature:  1,
		IsActive:     false,
	}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create stale service desk agent: %v", err)
	}

	if err := SeedAgents(db); err != nil {
		t.Fatalf("seed agents: %v", err)
	}

	var got aiapp.Agent
	if err := db.Where("code = ?", code).First(&got).Error; err != nil {
		t.Fatalf("load service desk agent: %v", err)
	}
	if got.SystemPrompt == "stale prompt" ||
		!strings.Contains(got.SystemPrompt, "prefill_suggestions") ||
		!strings.Contains(got.SystemPrompt, "itsm.current_request_context") {
		t.Fatalf("expected service desk prompt to be refreshed")
	}
	if got.Name != "IT 服务台智能体" || !got.IsActive {
		t.Fatalf("expected service desk metadata to be refreshed, got name=%q active=%v", got.Name, got.IsActive)
	}

	var decision aiapp.Agent
	if err := db.Where("code = ?", "itsm.decision").First(&decision).Error; err != nil {
		t.Fatalf("load decision agent: %v", err)
	}
	for _, needle := range []string{"证据优先", "一次只决策当前下一步", "低置信"} {
		if !strings.Contains(decision.SystemPrompt, needle) {
			t.Fatalf("expected decision prompt to contain %q", needle)
		}
	}
}

func TestSeedToolsIncludesCurrentRequestContext(t *testing.T) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&aiapp.Tool{}, &aiapp.AgentTool{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	if err := SeedTools(db); err != nil {
		t.Fatalf("seed tools: %v", err)
	}

	var tool aiapp.Tool
	if err := db.Where("name = ?", "itsm.current_request_context").First(&tool).Error; err != nil {
		t.Fatalf("expected current request context tool to be seeded: %v", err)
	}
	if !strings.Contains(tool.Description, "多轮继续") {
		t.Fatalf("expected context-aware tool description, got %q", tool.Description)
	}
}
