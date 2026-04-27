package runtime

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/pkg/crypto"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.AutoMigrate(
		// Provider & Model
		&Provider{}, &AIModel{}, &AILog{},
		// Knowledge (new unified model)
		&KnowledgeAsset{}, &KnowledgeSource{}, &KnowledgeAssetSource{},
		&RAGChunk{}, &KnowledgeLog{},
		// Legacy knowledge table
		&KnowledgeBase{},
		// Tool registry
		&Tool{}, &MCPServer{}, &Skill{},
		&CapabilitySet{}, &CapabilitySetItem{},
		// Agent bindings
		&AgentTool{}, &AgentMCPServer{}, &AgentSkill{}, &AgentKnowledgeBase{}, &AgentKnowledgeGraph{},
		&AgentCapabilitySet{}, &AgentCapabilitySetItem{},
		// Agent runtime
		&Agent{}, &AgentTemplate{}, &AgentSession{}, &SessionMessage{}, &AgentMemory{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return db
}

func newTestEncryptionKey(t *testing.T) crypto.EncryptionKey {
	t.Helper()
	// Deterministic 32-byte AES key for tests
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return crypto.EncryptionKey(key)
}

func newProviderServiceForTest(t *testing.T, db *gorm.DB) *ProviderService {
	t.Helper()
	return &ProviderService{
		repo:   &ProviderRepo{db: &database.DB{DB: db}},
		encKey: newTestEncryptionKey(t),
	}
}

func newModelServiceForTest(t *testing.T, db *gorm.DB) *ModelService {
	t.Helper()
	return &ModelService{
		repo:         &ModelRepo{db: &database.DB{DB: db}},
		providerRepo: &ProviderRepo{db: &database.DB{DB: db}},
		encKey:       newTestEncryptionKey(t),
	}
}

func newToolServiceForTest(t *testing.T, db *gorm.DB) *ToolService {
	t.Helper()
	return &ToolService{
		repo: &ToolRepo{db: &database.DB{DB: db}},
	}
}

func newMCPServerServiceForTest(t *testing.T, db *gorm.DB) *MCPServerService {
	t.Helper()
	return &MCPServerService{
		repo:      &MCPServerRepo{db: &database.DB{DB: db}},
		encKey:    newTestEncryptionKey(t),
		mcpClient: &fakeMCPRuntimeClient{},
	}
}

func newSkillServiceForTest(t *testing.T, db *gorm.DB) *SkillService {
	t.Helper()
	return &SkillService{
		repo:   &SkillRepo{db: &database.DB{DB: db}},
		encKey: newTestEncryptionKey(t),
	}
}

func newAgentServiceForTest(t *testing.T, db *gorm.DB) *AgentService {
	t.Helper()
	return &AgentService{
		repo: &AgentRepo{db: &database.DB{DB: db}},
	}
}

func newSessionServiceForTest(t *testing.T, db *gorm.DB) *SessionService {
	t.Helper()
	return &SessionService{
		repo:     &SessionRepo{db: &database.DB{DB: db}},
		agentSvc: newAgentServiceForTest(t, db),
	}
}

func newKnowledgeSourceServiceForTest(t *testing.T, db *gorm.DB) *KnowledgeSourceService {
	t.Helper()
	return &KnowledgeSourceService{
		sourceRepo: &KnowledgeSourceRepo{db: &database.DB{DB: db}},
		assetRepo:  &KnowledgeAssetRepo{db: &database.DB{DB: db}},
	}
}

func seedAgentBindingTargets(t *testing.T, db *gorm.DB) (toolIDs, skillIDs, mcpIDs, kbIDs, kgIDs []uint) {
	t.Helper()

	tools := []Tool{
		{Name: "tool_a", DisplayName: "Tool A", ParametersSchema: model.JSONText("{}"), IsActive: true},
		{Name: "tool_b", DisplayName: "Tool B", ParametersSchema: model.JSONText("{}"), IsActive: true},
	}
	for i := range tools {
		if err := db.Create(&tools[i]).Error; err != nil {
			t.Fatalf("seed tool: %v", err)
		}
		toolIDs = append(toolIDs, tools[i].ID)
	}

	skills := []Skill{
		{Name: "skill_a", DisplayName: "Skill A", SourceType: SkillSourceUpload, IsActive: true},
	}
	for i := range skills {
		if err := db.Create(&skills[i]).Error; err != nil {
			t.Fatalf("seed skill: %v", err)
		}
		skillIDs = append(skillIDs, skills[i].ID)
	}

	mcps := []MCPServer{
		{Name: "mcp_a", Transport: MCPTransportSSE, URL: "https://example.com/sse", AuthType: AuthTypeNone, IsActive: true},
	}
	for i := range mcps {
		if err := db.Create(&mcps[i]).Error; err != nil {
			t.Fatalf("seed mcp: %v", err)
		}
		mcpIDs = append(mcpIDs, mcps[i].ID)
	}

	assets := []KnowledgeAsset{
		{Name: "kb_a", Category: AssetCategoryKB, Type: KBTypeNaiveChunk, Status: AssetStatusIdle},
		{Name: "kg_a", Category: AssetCategoryKG, Type: KGTypeConceptMap, Status: AssetStatusIdle},
	}
	for i := range assets {
		if err := db.Create(&assets[i]).Error; err != nil {
			t.Fatalf("seed knowledge asset: %v", err)
		}
		if assets[i].Category == AssetCategoryKB {
			kbIDs = append(kbIDs, assets[i].ID)
		} else {
			kgIDs = append(kgIDs, assets[i].ID)
		}
	}

	return toolIDs, skillIDs, mcpIDs, kbIDs, kgIDs
}

// stubKnowledgeGraphRepo is a minimal stub for KnowledgeGraphRepo used in tests.
type stubKnowledgeGraphRepo struct {
	deleteGraphCalls    []uint
	deleteNodesBySource []struct{ kbID, sourceID uint }
}

func (s *stubKnowledgeGraphRepo) DeleteGraph(kbID uint) error {
	s.deleteGraphCalls = append(s.deleteGraphCalls, kbID)
	return nil
}

func (s *stubKnowledgeGraphRepo) DeleteNodesBySourceID(kbID uint, sourceID uint) (int64, error) {
	s.deleteNodesBySource = append(s.deleteNodesBySource, struct{ kbID, sourceID uint }{kbID, sourceID})
	return 1, nil
}
