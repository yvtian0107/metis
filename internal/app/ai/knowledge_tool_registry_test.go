package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"metis/internal/app"
	"metis/internal/database"
)

func TestKnowledgeToolRegistry_SearchUsesSessionBoundAssets(t *testing.T) {
	registerTestRecallEngine()
	db := setupTestDB(t)
	agent := &Agent{Name: "knowledge-tool-agent", Type: AgentTypeAssistant, CreatedBy: 1}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	asset := KnowledgeAsset{Name: "vpn-tool-kb", Category: AssetCategoryKB, Type: "test_recall", Status: AssetStatusReady}
	unbound := KnowledgeAsset{Name: "unbound-kb", Category: AssetCategoryKB, Type: "test_recall", Status: AssetStatusReady}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := db.Create(&unbound).Error; err != nil {
		t.Fatalf("create unbound asset: %v", err)
	}
	agentRepo := &AgentRepo{db: &database.DB{DB: db}}
	if err := agentRepo.replaceKnowledgeBaseBindingsInTx(db, agent.ID, []uint{asset.ID}); err != nil {
		t.Fatalf("bind knowledge: %v", err)
	}
	session := AgentSession{AgentID: agent.ID, UserID: 7, Status: SessionStatusRunning}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}
	registry := &KnowledgeToolRegistry{
		agentRepo:         agentRepo,
		sessionRepo:       &SessionRepo{db: &database.DB{DB: db}},
		knowledgeSearcher: &KnowledgeSearchService{assetRepo: &KnowledgeAssetRepo{db: &database.DB{DB: db}}},
	}

	ctx := context.WithValue(context.Background(), app.SessionIDKey, session.ID)
	raw, err := registry.Execute(ctx, searchKnowledgeToolName, session.UserID, json.RawMessage(`{"query":"vpn reset","asset_ids":[999]}`))
	if err != nil {
		t.Fatalf("execute search_knowledge: %v", err)
	}
	var result searchKnowledgeResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.AssetScopeCount != 1 {
		t.Fatalf("expected one server-scoped asset, got %#v", result)
	}
	if len(result.Results) != 1 || !strings.Contains(result.Results[0].Content, "vpn-tool-kb") {
		t.Fatalf("expected bound asset result, got %#v", result.Results)
	}
	if strings.Contains(string(raw), "unbound-kb") {
		t.Fatalf("unbound knowledge leaked into result: %s", string(raw))
	}
}

func TestKnowledgeSearchService_RespectsCancelledContext(t *testing.T) {
	registerTestRecallEngine()
	db := setupTestDB(t)
	asset := KnowledgeAsset{Name: "vpn-cancel-kb", Category: AssetCategoryKB, Type: "test_recall", Status: AssetStatusReady}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	service := &KnowledgeSearchService{assetRepo: &KnowledgeAssetRepo{db: &database.DB{DB: db}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.SearchKnowledgeWithContext(ctx, []uint{asset.ID}, "vpn", 5)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
