package runtime

import (
	"reflect"
	"sort"
	"testing"

	"metis/internal/database"
	"metis/internal/model"
)

func TestCapabilitySetMigrationPreservesEffectiveBindings(t *testing.T) {
	db := setupTestDB(t)
	toolIDs, skillIDs, mcpIDs, kbIDs, kgIDs := seedAgentBindingTargets(t, db)

	agent := &Agent{Name: "legacy-agent", Type: AgentTypeAssistant, ModelID: uintPtr(1), CreatedBy: 1, Strategy: AgentStrategyReact}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	repo := &AgentRepo{db: &database.DB{DB: db}}
	if err := repo.replaceToolBindingsInTx(db, agent.ID, toolIDs); err != nil {
		t.Fatalf("legacy tool bind: %v", err)
	}
	if err := repo.replaceSkillBindingsInTx(db, agent.ID, skillIDs); err != nil {
		t.Fatalf("legacy skill bind: %v", err)
	}
	if err := repo.replaceMCPServerBindingsInTx(db, agent.ID, mcpIDs); err != nil {
		t.Fatalf("legacy mcp bind: %v", err)
	}
	if err := repo.replaceKnowledgeBaseBindingsInTx(db, agent.ID, kbIDs); err != nil {
		t.Fatalf("legacy kb bind: %v", err)
	}
	if err := repo.replaceKnowledgeGraphBindingsInTx(db, agent.ID, kgIDs); err != nil {
		t.Fatalf("legacy kg bind: %v", err)
	}

	if err := migrateAgentCapabilitySetBindings(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	assertIDs(t, "tools", toolIDs, mustIDs(repo.GetToolIDs(agent.ID)))
	assertIDs(t, "skills", skillIDs, mustIDs(repo.GetSkillIDs(agent.ID)))
	assertIDs(t, "mcp", mcpIDs, mustIDs(repo.GetMCPServerIDs(agent.ID)))
	assertIDs(t, "kb", kbIDs, mustIDs(repo.GetKnowledgeBaseIDs(agent.ID)))
	assertIDs(t, "kg", kgIDs, mustIDs(repo.GetKnowledgeGraphIDs(agent.ID)))

	setBindings, err := repo.GetCapabilitySetBindings(agent.ID)
	if err != nil {
		t.Fatalf("get set bindings: %v", err)
	}
	if len(setBindings) == 0 {
		t.Fatal("expected migrated capability set bindings")
	}
}

func TestAgentServiceCapabilitySetBindings(t *testing.T) {
	db := setupTestDB(t)
	toolIDs, _, _, _, _ := seedAgentBindingTargets(t, db)
	if err := ensureDefaultCapabilitySets(db); err != nil {
		t.Fatalf("ensure sets: %v", err)
	}
	setID, err := firstCapabilitySetIDForItem(db, CapabilityTypeTool, toolIDs[0])
	if err != nil {
		t.Fatalf("find set: %v", err)
	}

	svc := newAgentServiceForTest(t, db)
	agent := &Agent{Name: "set-agent", Type: AgentTypeAssistant, ModelID: uintPtr(1), CreatedBy: 1, Strategy: AgentStrategyReact}
	err = svc.CreateWithBindings(agent, AgentBindings{
		CapabilitySets: []AgentCapabilitySetBinding{{SetID: setID, ItemIDs: []uint{toolIDs[0]}}},
	})
	if err != nil {
		t.Fatalf("create with set bindings: %v", err)
	}

	repo := &AgentRepo{db: &database.DB{DB: db}}
	assertIDs(t, "selected tools", []uint{toolIDs[0]}, mustIDs(repo.GetToolIDs(agent.ID)))
	assertIDs(t, "legacy-synced tools", []uint{toolIDs[0]}, mustIDs(repo.getLegacyToolIDs(agent.ID)))

	invalid := &Agent{Name: "invalid-set-agent", Type: AgentTypeAssistant, ModelID: uintPtr(1), CreatedBy: 1, Strategy: AgentStrategyReact}
	err = svc.CreateWithBindings(invalid, AgentBindings{
		CapabilitySets: []AgentCapabilitySetBinding{{SetID: setID, ItemIDs: []uint{99999}}},
	})
	if err != ErrInvalidBinding {
		t.Fatalf("expected ErrInvalidBinding, got %v", err)
	}
}

func TestGatewayCapabilitySetRuntimeResolution(t *testing.T) {
	db := setupTestDB(t)
	active := Tool{Name: "general.current_time", Toolkit: "runtime", DisplayName: "Active Tool", Description: "active", ParametersSchema: model.JSONText("{}"), IsActive: true}
	inactive := Tool{Name: "system.current_user_profile", Toolkit: "runtime", DisplayName: "Inactive Tool", Description: "inactive", ParametersSchema: model.JSONText("{}"), IsActive: false}
	if err := db.Create(&active).Error; err != nil {
		t.Fatalf("create active tool: %v", err)
	}
	if err := db.Create(&inactive).Error; err != nil {
		t.Fatalf("create inactive tool: %v", err)
	}
	if err := db.Model(&inactive).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate tool: %v", err)
	}
	agent := &Agent{Name: "runtime-agent", Type: AgentTypeAssistant, ModelID: uintPtr(1), CreatedBy: 1, Strategy: AgentStrategyReact}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	setA := CapabilitySet{Type: CapabilityTypeTool, Name: "runtime-a", IsActive: true}
	setB := CapabilitySet{Type: CapabilityTypeTool, Name: "runtime-b", IsActive: true}
	if err := db.Create(&setA).Error; err != nil {
		t.Fatalf("create set a: %v", err)
	}
	if err := db.Create(&setB).Error; err != nil {
		t.Fatalf("create set b: %v", err)
	}
	for _, item := range []CapabilitySetItem{
		{SetID: setA.ID, ItemID: active.ID},
		{SetID: setA.ID, ItemID: inactive.ID},
		{SetID: setB.ID, ItemID: active.ID},
	} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create set item: %v", err)
		}
	}
	for _, binding := range []AgentCapabilitySet{
		{AgentID: agent.ID, SetID: setA.ID},
		{AgentID: agent.ID, SetID: setB.ID},
	} {
		if err := db.Create(&binding).Error; err != nil {
			t.Fatalf("create agent set binding: %v", err)
		}
	}
	for _, selected := range []AgentCapabilitySetItem{
		{AgentID: agent.ID, SetID: setA.ID, ItemID: active.ID, Enabled: true},
		{AgentID: agent.ID, SetID: setA.ID, ItemID: inactive.ID, Enabled: true},
		{AgentID: agent.ID, SetID: setB.ID, ItemID: active.ID, Enabled: true},
	} {
		if err := db.Create(&selected).Error; err != nil {
			t.Fatalf("create selected item: %v", err)
		}
	}

	gw := &AgentGateway{
		agentRepo:      &AgentRepo{db: &database.DB{DB: db}},
		toolRegistries: []ToolHandlerRegistry{NewGeneralToolRegistry(nil, nil)},
	}
	defs, err := gw.buildToolDefinitions(agent.ID)
	if err != nil {
		t.Fatalf("build defs: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected exactly one runtime tool, got %d: %#v", len(defs), defs)
	}
	if defs[0].Name != active.Name {
		t.Fatalf("expected active tool, got %s", defs[0].Name)
	}
}

func uintPtr(v uint) *uint {
	return &v
}

func mustIDs(ids []uint, err error) []uint {
	if err != nil {
		panic(err)
	}
	return ids
}

func assertIDs(t *testing.T, label string, want, got []uint) {
	t.Helper()
	want = append([]uint(nil), want...)
	got = append([]uint(nil), got...)
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s ids mismatch: want %v got %v", label, want, got)
	}
}
