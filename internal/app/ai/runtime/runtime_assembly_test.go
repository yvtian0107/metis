package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"metis/internal/app"
	"metis/internal/database"
	"metis/internal/model"
)

type testRecallEngine struct{}

func (e *testRecallEngine) Build(ctx context.Context, asset *KnowledgeAsset, sources []*KnowledgeSource) error {
	return nil
}

func (e *testRecallEngine) Rebuild(ctx context.Context, asset *KnowledgeAsset, sources []*KnowledgeSource) error {
	return nil
}

func (e *testRecallEngine) Search(ctx context.Context, asset *KnowledgeAsset, query *RecallQuery) (*RecallResult, error) {
	if strings.Contains(asset.Name, "fail") {
		return nil, errors.New("search failed")
	}
	if strings.Contains(asset.Name, "empty") {
		return &RecallResult{}, nil
	}
	return &RecallResult{Items: []KnowledgeUnit{{
		ID:      "unit-1",
		AssetID: asset.ID,
		Title:   asset.Name,
		Content: "vpn reset steps from " + asset.Name,
		Score:   0.9,
	}}}, nil
}

func (e *testRecallEngine) ContentStats(ctx context.Context, asset *KnowledgeAsset) (*ContentStats, error) {
	return &ContentStats{}, nil
}

var registerTestRecallEngineOnce sync.Once

type testToolRegistry map[string]json.RawMessage

func (r testToolRegistry) HasTool(name string) bool {
	_, ok := r[name]
	return ok
}

func (r testToolRegistry) Execute(ctx context.Context, toolName string, userID uint, args json.RawMessage) (json.RawMessage, error) {
	if raw, ok := r[toolName]; ok {
		return raw, nil
	}
	return nil, errors.New("unknown tool")
}

type testRuntimeContextProvider struct {
	block       string
	acceptBlank bool
}

func (p testRuntimeContextProvider) BuildAgentRuntimeContext(ctx context.Context, agentCode string, sessionID, userID uint) (string, error) {
	if p.acceptBlank && agentCode == "" && sessionID == 99 && userID == 7 {
		return p.block, nil
	}
	if agentCode == "itsm.servicedesk" && sessionID == 99 && userID == 7 {
		return p.block, nil
	}
	return "", nil
}

func registerTestRecallEngine() {
	registerTestRecallEngineOnce.Do(func() {
		RegisterEngine(AssetCategoryKB, "test_recall", &testRecallEngine{})
		RegisterEngine(AssetCategoryKG, "test_recall", &testRecallEngine{})
	})
}

func TestAssistantRuntimeAssembly_FiltersAndDeduplicatesResources(t *testing.T) {
	db := setupTestDB(t)
	active := Tool{Name: "runtime.active", Toolkit: "runtime", DisplayName: "Active Tool", ParametersSchema: model.JSONText("{}"), IsActive: true}
	inactive := Tool{Name: "runtime.inactive", Toolkit: "runtime", DisplayName: "Inactive Tool", ParametersSchema: model.JSONText("{}"), IsActive: true}
	unselected := Tool{Name: "runtime.unselected", Toolkit: "runtime", DisplayName: "Unselected Tool", ParametersSchema: model.JSONText("{}"), IsActive: true}
	deleted := Tool{Name: "runtime.deleted", Toolkit: "runtime", DisplayName: "Deleted Tool", ParametersSchema: model.JSONText("{}"), IsActive: true}
	unknown := Tool{Name: "runtime.unknown", Toolkit: "runtime", DisplayName: "Missing Handler", ParametersSchema: model.JSONText("{}"), IsActive: true}
	if err := db.Create(&active).Error; err != nil {
		t.Fatalf("create active tool: %v", err)
	}
	if err := db.Create(&inactive).Error; err != nil {
		t.Fatalf("create inactive tool: %v", err)
	}
	if err := db.Model(&inactive).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate tool: %v", err)
	}
	if err := db.Create(&unselected).Error; err != nil {
		t.Fatalf("create unselected tool: %v", err)
	}
	if err := db.Create(&deleted).Error; err != nil {
		t.Fatalf("create deleted tool: %v", err)
	}
	if err := db.Delete(&deleted).Error; err != nil {
		t.Fatalf("delete tool: %v", err)
	}
	if err := db.Create(&unknown).Error; err != nil {
		t.Fatalf("create unknown tool: %v", err)
	}
	agent := &Agent{Name: "runtime-filter-agent", Type: AgentTypeAssistant, ModelID: uintPtr(1), CreatedBy: 1, Strategy: AgentStrategyReact}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	setA := CapabilitySet{Type: CapabilityTypeTool, Name: "runtime-filter-a", IsActive: true}
	setB := CapabilitySet{Type: CapabilityTypeTool, Name: "runtime-filter-b", IsActive: true}
	if err := db.Create(&setA).Error; err != nil {
		t.Fatalf("create set a: %v", err)
	}
	if err := db.Create(&setB).Error; err != nil {
		t.Fatalf("create set b: %v", err)
	}
	for _, item := range []CapabilitySetItem{
		{SetID: setA.ID, ItemID: active.ID},
		{SetID: setA.ID, ItemID: inactive.ID},
		{SetID: setA.ID, ItemID: unselected.ID},
		{SetID: setA.ID, ItemID: deleted.ID},
		{SetID: setA.ID, ItemID: unknown.ID},
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
			t.Fatalf("create binding: %v", err)
		}
	}
	for _, selected := range []AgentCapabilitySetItem{
		{AgentID: agent.ID, SetID: setA.ID, ItemID: active.ID, Enabled: true},
		{AgentID: agent.ID, SetID: setA.ID, ItemID: inactive.ID, Enabled: true},
		{AgentID: agent.ID, SetID: setA.ID, ItemID: deleted.ID, Enabled: true},
		{AgentID: agent.ID, SetID: setA.ID, ItemID: unknown.ID, Enabled: true},
		{AgentID: agent.ID, SetID: setB.ID, ItemID: active.ID, Enabled: true},
	} {
		if err := db.Create(&selected).Error; err != nil {
			t.Fatalf("create selected item: %v", err)
		}
	}

	gw := &AgentGateway{
		agentRepo: &AgentRepo{db: &database.DB{DB: db}},
		toolRegistries: []ToolHandlerRegistry{testToolRegistry{
			"runtime.active":     json.RawMessage(`{"ok":true}`),
			"runtime.inactive":   json.RawMessage(`{"ok":true}`),
			"runtime.unselected": json.RawMessage(`{"ok":true}`),
			"runtime.deleted":    json.RawMessage(`{"ok":true}`),
		}},
		mcpClient: &fakeMCPRuntimeClient{},
	}
	runtime, err := gw.buildAssistantRuntime(context.Background(), agent, &AgentSession{AgentID: agent.ID, UserID: 1}, nil, "")
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	if len(runtime.Tools) != 1 {
		t.Fatalf("expected exactly one executable active tool, got %#v", runtime.Tools)
	}
	if runtime.Tools[0].Name != "runtime.active" {
		t.Fatalf("expected runtime.active, got %s", runtime.Tools[0].Name)
	}
}

func TestAssistantRuntimeAssembly_AppendsRuntimeContextProviderBlock(t *testing.T) {
	db := setupTestDB(t)
	gw := newGatewayForTest(t, db, nil)
	gw.runtimeContextProviders = []app.AgentRuntimeContextProvider{
		testRuntimeContextProvider{block: "## Runtime Context\nloaded_service_id: 5"},
	}

	code := "itsm.servicedesk"
	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, Code: &code, CreatedBy: 1}
	_ = gw.agentSvc.Create(agent)
	session := &AgentSession{BaseModel: model.BaseModel{ID: 99}, UserID: 7}

	runtime, err := gw.buildAssistantRuntime(context.Background(), agent, session, []ExecuteMessage{{Role: MessageRoleUser, Content: "继续"}}, "base")
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	if !strings.Contains(runtime.SystemPrompt, "## Runtime Context") || !strings.Contains(runtime.SystemPrompt, "loaded_service_id: 5") {
		t.Fatalf("expected runtime context block in prompt, got %q", runtime.SystemPrompt)
	}
}

func TestAssistantRuntimeAssembly_AppendsRuntimeContextForCodeLessAgent(t *testing.T) {
	db := setupTestDB(t)
	gw := newGatewayForTest(t, db, nil)
	gw.runtimeContextProviders = []app.AgentRuntimeContextProvider{
		testRuntimeContextProvider{block: "## Runtime Context\nloaded_service_id: 8", acceptBlank: true},
	}

	modelID := uint(1)
	agent := &Agent{Name: "Custom Intake", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = gw.agentSvc.Create(agent)
	session := &AgentSession{BaseModel: model.BaseModel{ID: 99}, UserID: 7}

	runtime, err := gw.buildAssistantRuntime(context.Background(), agent, session, []ExecuteMessage{{Role: MessageRoleUser, Content: "继续"}}, "base")
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	if !strings.Contains(runtime.SystemPrompt, "loaded_service_id: 8") {
		t.Fatalf("expected runtime context block for codeless agent, got %q", runtime.SystemPrompt)
	}
}

func TestAssistantRuntimeAssembly_KnowledgeContext(t *testing.T) {
	registerTestRecallEngine()
	db := setupTestDB(t)
	agent := &Agent{Name: "knowledge-agent", Type: AgentTypeAssistant, ModelID: uintPtr(1), CreatedBy: 1, Strategy: AgentStrategyReact}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	assets := []KnowledgeAsset{
		{Name: "vpn-kb", Category: AssetCategoryKB, Type: "test_recall", Status: AssetStatusReady},
		{Name: "empty-kb", Category: AssetCategoryKB, Type: "test_recall", Status: AssetStatusReady},
		{Name: "fail-kb", Category: AssetCategoryKB, Type: "test_recall", Status: AssetStatusReady},
	}
	for i := range assets {
		if err := db.Create(&assets[i]).Error; err != nil {
			t.Fatalf("create asset: %v", err)
		}
	}
	repo := &AgentRepo{db: &database.DB{DB: db}}
	if err := repo.replaceKnowledgeBaseBindingsInTx(db, agent.ID, []uint{assets[0].ID, assets[1].ID, assets[2].ID}); err != nil {
		t.Fatalf("bind knowledge: %v", err)
	}
	gw := &AgentGateway{
		agentRepo:         repo,
		toolRegistries:    []ToolHandlerRegistry{NewGeneralToolRegistry(nil, nil)},
		knowledgeSearcher: &KnowledgeSearchService{assetRepo: &KnowledgeAssetRepo{db: &database.DB{DB: db}}},
	}
	runtime, err := gw.buildAssistantRuntime(context.Background(), agent, &AgentSession{AgentID: agent.ID, UserID: 1}, []ExecuteMessage{{Role: MessageRoleUser, Content: "vpn reset"}}, "base")
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	if !strings.Contains(runtime.SystemPrompt, "## Knowledge Context") {
		t.Fatalf("expected knowledge context in prompt: %s", runtime.SystemPrompt)
	}
	if !strings.Contains(runtime.SystemPrompt, "vpn reset steps from vpn-kb") {
		t.Fatalf("expected recalled content, got: %s", runtime.SystemPrompt)
	}
	if strings.Contains(runtime.SystemPrompt, "fail-kb") {
		t.Fatalf("failed asset should not appear in prompt: %s", runtime.SystemPrompt)
	}
}

func TestAssistantRuntimeAssembly_NoSelectedKnowledgeKeepsBasePrompt(t *testing.T) {
	registerTestRecallEngine()
	db := setupTestDB(t)
	agent := &Agent{Name: "no-knowledge-agent", Type: AgentTypeAssistant, ModelID: uintPtr(1), CreatedBy: 1, Strategy: AgentStrategyReact}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	gw := &AgentGateway{
		agentRepo:         &AgentRepo{db: &database.DB{DB: db}},
		knowledgeSearcher: &KnowledgeSearchService{assetRepo: &KnowledgeAssetRepo{db: &database.DB{DB: db}}},
	}

	runtime, err := gw.buildAssistantRuntime(context.Background(), agent, &AgentSession{AgentID: agent.ID, UserID: 1}, []ExecuteMessage{{Role: MessageRoleUser, Content: "vpn reset"}}, "base")
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	if runtime.SystemPrompt != "base" {
		t.Fatalf("expected base prompt only, got %q", runtime.SystemPrompt)
	}
}

func TestAssistantRuntimeAssembly_SkillPromptAndEndpointTools(t *testing.T) {
	db := setupTestDB(t)
	agent := &Agent{Name: "skill-agent", Type: AgentTypeAssistant, ModelID: uintPtr(1), CreatedBy: 1, Strategy: AgentStrategyReact}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "input": body})
	}))
	defer server.Close()

	promptOnly := Skill{Name: "prompt_skill", DisplayName: "Prompt Skill", SourceType: SkillSourceUpload, Instructions: "Always cite policy.", IsActive: true}
	endpoint := Skill{
		Name:         "endpoint_skill",
		DisplayName:  "Endpoint Skill",
		SourceType:   SkillSourceUpload,
		Instructions: "Use endpoint if needed.",
		ToolsSchema: model.JSONText(fmtJSON([]map[string]any{{
			"name":        "lookup",
			"description": "Lookup something",
			"parameters":  map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}},
			"endpoint":    map[string]any{"url": server.URL, "method": "POST"},
		}})),
		IsActive: true,
	}
	malformed := Skill{Name: "bad_skill", DisplayName: "Bad Skill", SourceType: SkillSourceUpload, ToolsSchema: model.JSONText(`not-json`), IsActive: true}
	inactive := Skill{Name: "inactive_skill", DisplayName: "Inactive Skill", SourceType: SkillSourceUpload, Instructions: "do not include", IsActive: true}
	for _, skill := range []*Skill{&promptOnly, &endpoint, &malformed, &inactive} {
		if err := db.Create(skill).Error; err != nil {
			t.Fatalf("create skill: %v", err)
		}
	}
	if err := db.Model(&inactive).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate skill: %v", err)
	}
	repo := &AgentRepo{db: &database.DB{DB: db}}
	if err := repo.replaceSkillBindingsInTx(db, agent.ID, []uint{promptOnly.ID, endpoint.ID, malformed.ID, inactive.ID}); err != nil {
		t.Fatalf("bind skills: %v", err)
	}
	gw := &AgentGateway{agentRepo: repo, toolRegistries: []ToolHandlerRegistry{NewGeneralToolRegistry(nil, nil)}}
	runtime, err := gw.buildAssistantRuntime(context.Background(), agent, &AgentSession{AgentID: agent.ID, UserID: 1}, nil, "base")
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	if !strings.Contains(runtime.SystemPrompt, "Always cite policy.") || !strings.Contains(runtime.SystemPrompt, "Use endpoint if needed.") {
		t.Fatalf("expected skill instructions in prompt: %s", runtime.SystemPrompt)
	}
	if strings.Contains(runtime.SystemPrompt, "do not include") {
		t.Fatalf("inactive skill instructions should be omitted: %s", runtime.SystemPrompt)
	}
	if len(runtime.Tools) != 1 || runtime.Tools[0].Name != "skill__endpoint_skill__lookup" {
		t.Fatalf("expected one endpoint skill tool, got %#v", runtime.Tools)
	}
	var skillReg ToolHandlerRegistry
	for _, reg := range runtime.ToolRegistries {
		if reg.HasTool("skill__endpoint_skill__lookup") {
			skillReg = reg
			break
		}
	}
	if skillReg == nil {
		t.Fatal("expected executable skill registry")
	}
	raw, err := skillReg.Execute(context.Background(), "skill__endpoint_skill__lookup", 1, json.RawMessage(`{"q":"vpn"}`))
	if err != nil {
		t.Fatalf("execute skill tool: %v", err)
	}
	if !strings.Contains(string(raw), `"ok":true`) {
		t.Fatalf("expected skill endpoint response, got %s", raw)
	}
}

func TestAssistantRuntimeAssembly_MCPDiscoveryAndDispatch(t *testing.T) {
	db := setupTestDB(t)
	agent := &Agent{Name: "mcp-agent", Type: AgentTypeAssistant, ModelID: uintPtr(1), CreatedBy: 1, Strategy: AgentStrategyReact}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	active := MCPServer{Name: "main-server", Transport: MCPTransportSSE, URL: "https://example.com/sse", AuthType: AuthTypeNone, IsActive: true}
	inactive := MCPServer{Name: "inactive-server", Transport: MCPTransportSSE, URL: "https://example.com/sse", AuthType: AuthTypeNone, IsActive: true}
	if err := db.Create(&active).Error; err != nil {
		t.Fatalf("create active mcp: %v", err)
	}
	if err := db.Create(&inactive).Error; err != nil {
		t.Fatalf("create inactive mcp: %v", err)
	}
	if err := db.Model(&inactive).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate mcp: %v", err)
	}
	repo := &AgentRepo{db: &database.DB{DB: db}}
	if err := repo.replaceMCPServerBindingsInTx(db, agent.ID, []uint{active.ID, inactive.ID}); err != nil {
		t.Fatalf("bind mcp: %v", err)
	}
	client := &fakeMCPRuntimeClient{
		toolsByServer: map[uint][]MCPRuntimeTool{
			active.ID:   {{Name: "lookup", Description: "Lookup", Parameters: []byte(`{"type":"object"}`)}},
			inactive.ID: {{Name: "hidden", Description: "Hidden", Parameters: []byte(`{"type":"object"}`)}},
		},
		results: map[string]json.RawMessage{"main-server:lookup": json.RawMessage(`{"result":"ok"}`)},
	}
	gw := &AgentGateway{agentRepo: repo, toolRegistries: []ToolHandlerRegistry{NewGeneralToolRegistry(nil, nil)}, mcpClient: client}
	runtime, err := gw.buildAssistantRuntime(context.Background(), agent, &AgentSession{AgentID: agent.ID, UserID: 1}, nil, "")
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	if len(runtime.Tools) != 1 || runtime.Tools[0].Name != "mcp__main-server__lookup" {
		t.Fatalf("expected one active MCP tool, got %#v", runtime.Tools)
	}
	var mcpReg ToolHandlerRegistry
	for _, reg := range runtime.ToolRegistries {
		if reg.HasTool("mcp__main-server__lookup") {
			mcpReg = reg
			break
		}
	}
	if mcpReg == nil {
		t.Fatal("expected executable MCP registry")
	}
	raw, err := mcpReg.Execute(context.Background(), "mcp__main-server__lookup", 1, json.RawMessage(`{"q":"vpn"}`))
	if err != nil {
		t.Fatalf("execute mcp tool: %v", err)
	}
	if string(raw) != `{"result":"ok"}` {
		t.Fatalf("expected MCP result, got %s", raw)
	}
	client.callErr = errors.New("mcp down")
	if _, err := mcpReg.Execute(context.Background(), "mcp__main-server__lookup", 1, json.RawMessage(`{"q":"vpn"}`)); err == nil || !strings.Contains(err.Error(), "mcp down") {
		t.Fatalf("expected MCP call failure, got %v", err)
	}
	if _, err := mcpReg.Execute(context.Background(), "mcp__main-server__missing", 1, nil); err == nil {
		t.Fatal("expected unknown MCP tool error")
	}
}

func TestAssistantRuntimeAssembly_MCPDiscoveryFailureSkipsServer(t *testing.T) {
	db := setupTestDB(t)
	agent := &Agent{Name: "mcp-fail-agent", Type: AgentTypeAssistant, ModelID: uintPtr(1), CreatedBy: 1, Strategy: AgentStrategyReact}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	server := MCPServer{Name: "fail-server", Transport: MCPTransportSSE, URL: "https://example.com/sse", AuthType: AuthTypeNone, IsActive: true}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("create mcp: %v", err)
	}
	repo := &AgentRepo{db: &database.DB{DB: db}}
	if err := repo.replaceMCPServerBindingsInTx(db, agent.ID, []uint{server.ID}); err != nil {
		t.Fatalf("bind mcp: %v", err)
	}
	gw := &AgentGateway{
		agentRepo: repo,
		mcpClient: &fakeMCPRuntimeClient{discoverErr: errors.New("discovery down")},
	}
	runtime, err := gw.buildAssistantRuntime(context.Background(), agent, &AgentSession{AgentID: agent.ID, UserID: 1}, nil, "")
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	if len(runtime.Tools) != 0 {
		t.Fatalf("expected discovery failure to skip MCP tools, got %#v", runtime.Tools)
	}
}

func fmtJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

var _ app.AIKnowledgeSearcher = (*KnowledgeSearchService)(nil)
