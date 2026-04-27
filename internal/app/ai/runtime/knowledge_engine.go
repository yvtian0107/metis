package runtime

import (
	"context"
	"fmt"
	"sync"
)

// KnowledgeEngine is the unified interface that all knowledge processing
// strategies must implement. Engines are registered by "category:type" key.
type KnowledgeEngine interface {
	// Build processes associated sources and creates indexed content
	// (chunks for RAG, nodes/edges for graphs). Incremental by default.
	Build(ctx context.Context, asset *KnowledgeAsset, sources []*KnowledgeSource) error

	// Rebuild deletes all existing content and rebuilds from scratch.
	Rebuild(ctx context.Context, asset *KnowledgeAsset, sources []*KnowledgeSource) error

	// Search queries the indexed content and returns unified results.
	Search(ctx context.Context, asset *KnowledgeAsset, query *RecallQuery) (*RecallResult, error)

	// ContentStats returns content statistics for the asset.
	ContentStats(ctx context.Context, asset *KnowledgeAsset) (*ContentStats, error)
}

// RecallQuery represents a search request to an engine.
type RecallQuery struct {
	Query    string  `json:"query"`
	Mode     string  `json:"mode"` // vector, fulltext, hybrid, graph_expand
	TopK     int     `json:"topK"`
	MinScore float64 `json:"minScore,omitempty"`
}

// ContentStats holds content statistics for a knowledge asset.
type ContentStats struct {
	NodeCount  int `json:"nodeCount,omitempty"`
	EdgeCount  int `json:"edgeCount,omitempty"`
	ChunkCount int `json:"chunkCount,omitempty"`
}

// EngineKey returns the registry key for a category+type combination.
func EngineKey(category, typ string) string {
	return category + ":" + typ
}

// --- Engine Registry ---

var (
	engineMu       sync.RWMutex
	engineRegistry = make(map[string]KnowledgeEngine)
)

// RegisterEngine registers a KnowledgeEngine implementation for the given
// category and type. It panics if a duplicate key is registered.
func RegisterEngine(category, typ string, engine KnowledgeEngine) {
	key := EngineKey(category, typ)
	engineMu.Lock()
	defer engineMu.Unlock()
	if _, exists := engineRegistry[key]; exists {
		panic(fmt.Sprintf("knowledge engine already registered: %s", key))
	}
	engineRegistry[key] = engine
}

// GetEngine looks up a registered engine by category and type.
// Returns nil if not found.
func GetEngine(category, typ string) KnowledgeEngine {
	key := EngineKey(category, typ)
	engineMu.RLock()
	defer engineMu.RUnlock()
	return engineRegistry[key]
}

// GetEngineForAsset is a convenience helper that looks up the engine for an asset.
func GetEngineForAsset(asset *KnowledgeAsset) (KnowledgeEngine, error) {
	engine := GetEngine(asset.Category, asset.Type)
	if engine == nil {
		return nil, fmt.Errorf("unsupported knowledge type: %s:%s", asset.Category, asset.Type)
	}
	return engine, nil
}

// ListEngineKeys returns all registered engine keys.
func ListEngineKeys() []string {
	engineMu.RLock()
	defer engineMu.RUnlock()
	keys := make([]string, 0, len(engineRegistry))
	for k := range engineRegistry {
		keys = append(keys, k)
	}
	return keys
}
