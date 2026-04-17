package ai

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"metis/internal/database"
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
		// Knowledge
		&KnowledgeBase{}, &KnowledgeSource{}, &KnowledgeLog{},
		// Tool registry
		&Tool{}, &MCPServer{}, &Skill{},
		// Agent bindings
		&AgentTool{}, &AgentMCPServer{}, &AgentSkill{}, &AgentKnowledgeBase{},
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
		repo:   &MCPServerRepo{db: &database.DB{DB: db}},
		encKey: newTestEncryptionKey(t),
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

func newKnowledgeBaseServiceForTest(t *testing.T, db *gorm.DB, graphRepo *stubKnowledgeGraphRepo) *KnowledgeBaseService {
	t.Helper()
	return &KnowledgeBaseService{
		repo:       &KnowledgeBaseRepo{db: &database.DB{DB: db}},
		sourceRepo: &KnowledgeSourceRepo{db: &database.DB{DB: db}},
		graphRepo:  graphRepo,
	}
}

func newKnowledgeSourceServiceForTest(t *testing.T, db *gorm.DB, graphRepo *stubKnowledgeGraphRepo) *KnowledgeSourceService {
	t.Helper()
	return &KnowledgeSourceService{
		repo:      &KnowledgeSourceRepo{db: &database.DB{DB: db}},
		kbRepo:    &KnowledgeBaseRepo{db: &database.DB{DB: db}},
		graphRepo: graphRepo,
	}
}

// stubKnowledgeGraphRepo is a minimal stub for KnowledgeGraphRepo used in tests.
type stubKnowledgeGraphRepo struct {
	deleteGraphCalls     []uint
	deleteNodesBySource  []struct{ kbID, sourceID uint }
}

func (s *stubKnowledgeGraphRepo) DeleteGraph(kbID uint) error {
	s.deleteGraphCalls = append(s.deleteGraphCalls, kbID)
	return nil
}

func (s *stubKnowledgeGraphRepo) DeleteNodesBySourceID(kbID uint, sourceID uint) (int64, error) {
	s.deleteNodesBySource = append(s.deleteNodesBySource, struct{ kbID, sourceID uint }{kbID, sourceID})
	return 1, nil
}
