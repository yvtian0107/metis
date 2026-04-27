package runtime

import (
	"errors"
	"testing"
)

func TestToolService_List(t *testing.T) {
	db := setupTestDB(t)
	svc := newToolServiceForTest(t, db)
	repo := svc.repo

	// Seed tools
	_ = repo.Create(&Tool{Name: "search", Toolkit: "general", DisplayName: "Search", IsActive: true})
	_ = repo.Create(&Tool{Name: "calc", Toolkit: "general", DisplayName: "Calculator", IsActive: true})

	svc.registries = []ToolHandlerRegistry{testToolRegistry{
		"search": []byte(`{"ok":true}`),
		"calc":   []byte(`{"ok":true}`),
	}}

	tools, err := svc.List()
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
	for _, tool := range tools {
		if !tool.IsExecutable || tool.AvailabilityStatus != ToolAvailabilityAvailable {
			t.Fatalf("expected executable tool, got %#v", tool)
		}
	}
}

func TestToolService_ToggleActive(t *testing.T) {
	db := setupTestDB(t)
	svc := newToolServiceForTest(t, db)
	repo := svc.repo

	tool := &Tool{Name: "search", Toolkit: "general", DisplayName: "Search", IsActive: true}
	_ = repo.Create(tool)

	updated, err := svc.ToggleActive(tool.ID, false, ToggleToolOptions{})
	if err != nil {
		t.Fatalf("toggle active: %v", err)
	}
	if updated.IsActive {
		t.Error("expected tool to be inactive")
	}
}

func TestToolService_ListMarksUnavailableTools(t *testing.T) {
	db := setupTestDB(t)
	svc := newToolServiceForTest(t, db)
	svc.registries = []ToolHandlerRegistry{testToolRegistry{
		"general.current_time": []byte(`{"ok":true}`),
		"search_knowledge":     []byte(`{"ok":true}`),
	}}
	repo := svc.repo

	tools := []Tool{
		{Name: "general.current_time", Toolkit: "general", DisplayName: "Time", IsActive: true},
		{Name: "search_knowledge", Toolkit: "knowledge", DisplayName: "Search", IsActive: true},
		{Name: "read_document", Toolkit: "knowledge", DisplayName: "Read", IsActive: true},
		{Name: "http_request", Toolkit: "network", DisplayName: "HTTP", IsActive: false},
	}
	for i := range tools {
		if err := repo.Create(&tools[i]); err != nil {
			t.Fatalf("create tool: %v", err)
		}
	}

	responses, err := svc.List()
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	byName := map[string]ToolResponse{}
	for _, tool := range responses {
		byName[tool.Name] = tool
	}
	if !byName["general.current_time"].IsExecutable {
		t.Fatalf("expected current_time executable: %#v", byName["general.current_time"])
	}
	if !byName["search_knowledge"].IsExecutable {
		t.Fatalf("expected search_knowledge executable: %#v", byName["search_knowledge"])
	}
	if byName["read_document"].AvailabilityStatus != ToolAvailabilityUnimplemented {
		t.Fatalf("expected read_document unimplemented: %#v", byName["read_document"])
	}
	if byName["http_request"].AvailabilityStatus != ToolAvailabilityRiskDisabled {
		t.Fatalf("expected http_request risk disabled: %#v", byName["http_request"])
	}
}

func TestToolService_DisableBoundToolRequiresConfirm(t *testing.T) {
	db := setupTestDB(t)
	svc := newToolServiceForTest(t, db)
	svc.registries = []ToolHandlerRegistry{testToolRegistry{
		"general.current_time": []byte(`{"ok":true}`),
	}}

	tool := &Tool{Name: "general.current_time", Toolkit: "general", DisplayName: "Time", IsActive: true}
	if err := svc.repo.Create(tool); err != nil {
		t.Fatalf("create tool: %v", err)
	}
	agent := &Agent{Name: "agent", Type: AgentTypeAssistant, CreatedBy: 1}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := db.Create(&AgentTool{AgentID: agent.ID, ToolID: tool.ID}).Error; err != nil {
		t.Fatalf("bind tool: %v", err)
	}

	_, err := svc.ToggleActive(tool.ID, false, ToggleToolOptions{})
	var impactErr *ToolImpactError
	if !errors.As(err, &impactErr) {
		t.Fatalf("expected impact confirmation error, got %v", err)
	}
	if impactErr.BoundAgentCount != 1 {
		t.Fatalf("expected 1 bound agent, got %d", impactErr.BoundAgentCount)
	}

	updated, err := svc.ToggleActive(tool.ID, false, ToggleToolOptions{ConfirmImpact: true})
	if err != nil {
		t.Fatalf("toggle with confirm: %v", err)
	}
	if updated.IsActive {
		t.Fatal("expected tool disabled after confirmed toggle")
	}
}

func TestCapabilitySetToolItemsExposeAvailability(t *testing.T) {
	db := setupTestDB(t)
	tool := Tool{Name: "read_document", Toolkit: "knowledge", DisplayName: "Read", IsActive: true}
	if err := db.Create(&tool).Error; err != nil {
		t.Fatalf("create tool: %v", err)
	}
	set := CapabilitySet{Type: CapabilityTypeTool, Name: "knowledge", IsActive: true}
	if err := db.Create(&set).Error; err != nil {
		t.Fatalf("create set: %v", err)
	}
	if err := db.Create(&CapabilitySetItem{SetID: set.ID, ItemID: tool.ID}).Error; err != nil {
		t.Fatalf("create set item: %v", err)
	}

	items, err := capabilitySetItemsForSet(db, set)
	if err != nil {
		t.Fatalf("items for set: %v", err)
	}
	if len(items) != 1 || items[0].IsExecutable || items[0].AvailabilityStatus != ToolAvailabilityUnimplemented {
		t.Fatalf("expected unimplemented capability item, got %#v", items)
	}
}
