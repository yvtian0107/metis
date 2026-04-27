package runtime

import (
	"encoding/json"
	"sync"
)

// AssetTypeMeta describes a knowledge asset type for the frontend.
type AssetTypeMeta struct {
	Category      string          `json:"category"`    // kb | kg
	Type          string          `json:"type"`        // naive_chunk, concept_map, …
	DisplayName   string          `json:"displayName"` // user-facing name
	Description   string          `json:"description"` // short description
	Icon          string          `json:"icon,omitempty"`
	DefaultConfig json.RawMessage `json:"defaultConfig,omitempty"` // default config JSON schema/values
}

var (
	typeMu       sync.RWMutex
	typeRegistry []AssetTypeMeta
)

// RegisterAssetType registers a type's metadata for frontend discovery.
func RegisterAssetType(meta AssetTypeMeta) {
	typeMu.Lock()
	defer typeMu.Unlock()
	typeRegistry = append(typeRegistry, meta)
}

// ListAssetTypes returns all registered types, optionally filtered by category.
func ListAssetTypes(category string) []AssetTypeMeta {
	typeMu.RLock()
	defer typeMu.RUnlock()
	if category == "" {
		result := make([]AssetTypeMeta, len(typeRegistry))
		copy(result, typeRegistry)
		return result
	}
	var result []AssetTypeMeta
	for _, m := range typeRegistry {
		if m.Category == category {
			result = append(result, m)
		}
	}
	return result
}

// GetAssetType looks up a registered type by category and type. Returns nil if not found.
func GetAssetType(category, typ string) *AssetTypeMeta {
	typeMu.RLock()
	defer typeMu.RUnlock()
	for i := range typeRegistry {
		if typeRegistry[i].Category == category && typeRegistry[i].Type == typ {
			return &typeRegistry[i]
		}
	}
	return nil
}

// registerBuiltinTypes registers all built-in knowledge asset types.
// Called during package init.
func registerBuiltinTypes() {
	// ----- Knowledge Base (RAG) types -----

	ragDefault, _ := json.Marshal(DefaultRAGConfig())

	RegisterAssetType(AssetTypeMeta{
		Category:      AssetCategoryKB,
		Type:          KBTypeNaiveChunk,
		DisplayName:   "标准检索",
		Description:   "按段落或固定长度切块，向量检索。适合大多数文档",
		Icon:          "file-text",
		DefaultConfig: ragDefault,
	})
	RegisterAssetType(AssetTypeMeta{
		Category:    AssetCategoryKB,
		Type:        KBTypeParentChild,
		DisplayName: "父子检索",
		Description: "小块定位、大块返回，适合长篇技术文档",
		Icon:        "layers",
	})
	RegisterAssetType(AssetTypeMeta{
		Category:    AssetCategoryKB,
		Type:        KBTypeSummary,
		DisplayName: "摘要检索",
		Description: "先提炼要点再索引，适合政策、规章类文档",
		Icon:        "list",
	})
	RegisterAssetType(AssetTypeMeta{
		Category:    AssetCategoryKB,
		Type:        KBTypeQA,
		DisplayName: "QA 检索",
		Description: "自动抽取问答对，适合 FAQ、客服知识",
		Icon:        "message-circle",
	})

	// ----- Knowledge Graph types -----

	graphDefault, _ := json.Marshal(DefaultGraphConfig())

	RegisterAssetType(AssetTypeMeta{
		Category:      AssetCategoryKG,
		Type:          KGTypeConceptMap,
		DisplayName:   "概念图谱",
		Description:   "抽取概念及其关系，Map-Reduce 合并。适合通用知识整理",
		Icon:          "git-branch",
		DefaultConfig: graphDefault,
	})
	RegisterAssetType(AssetTypeMeta{
		Category:    AssetCategoryKG,
		Type:        KGTypeEntityRelation,
		DisplayName: "实体图谱",
		Description: "识别人、组织、产品等实体关系，适合业务知识",
		Icon:        "users",
	})
	RegisterAssetType(AssetTypeMeta{
		Category:    AssetCategoryKG,
		Type:        KGTypeLightGraph,
		DisplayName: "轻量图谱",
		Description: "快速构建，成本低，适合大批量文档",
		Icon:        "zap",
	})
	RegisterAssetType(AssetTypeMeta{
		Category:    AssetCategoryKG,
		Type:        KGTypeEventGraph,
		DisplayName: "事件图谱",
		Description: "抽取事件和因果链，适合流程和历史类知识",
		Icon:        "clock",
	})
}

func init() {
	registerBuiltinTypes()
}
