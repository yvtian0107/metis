package ai

import (
	"encoding/json"
	"errors"
	"time"

	"metis/internal/model"
)

// ----- Errors -----

var (
	ErrAssetNotFound   = errors.New("knowledge asset not found")
	ErrSourceNotFound2 = errors.New("knowledge source not found") // renamed to avoid conflict during migration
	ErrChunkNotFound   = errors.New("knowledge chunk not found")
)

// ----- Asset category -----

const (
	AssetCategoryKB = "kb" // NaiveRAG knowledge base
	AssetCategoryKG = "kg" // Knowledge graph
)

// ----- Asset types (strategies) -----

// Knowledge base types
const (
	KBTypeNaiveChunk  = "naive_chunk"
	KBTypeParentChild = "parent_child"
	KBTypeSummary     = "summary_first"
	KBTypeQA          = "qa_extract"
)

// Knowledge graph types
const (
	KGTypeConceptMap     = "concept_map"
	KGTypeEntityRelation = "entity_relation"
	KGTypeLightGraph     = "light_graph"
	KGTypeEventGraph     = "event_graph"
)

// ----- Asset status -----

const (
	AssetStatusIdle     = "idle"
	AssetStatusBuilding = "building"
	AssetStatusReady    = "ready"
	AssetStatusError    = "error"
	AssetStatusStale    = "stale" // source changed, needs rebuild
)

// ----- Build stages -----

// RAG build stages
const (
	BuildStagePreparing = "preparing"
	BuildStageChunking  = "chunking"
	BuildStageEmbedding = "embedding"
	BuildStageIndexing  = "indexing"
	BuildStageCompleted = "completed"
	BuildStageIdle      = "idle"
)

// Graph compile stages (keep compat with existing)
const (
	GraphStagePreparing  = "preparing"
	GraphStageMapping    = "mapping"
	GraphStageCallingLLM = "calling_llm"
	GraphStageWriting    = "writing_nodes"
	GraphStageEmbedding  = "generating_embeddings"
	GraphStageCompleted  = "completed"
	GraphStageIdle       = "idle"
)

// =====================================================================
// KnowledgeAsset — unified table for both KB and KG
// =====================================================================

// KnowledgeAsset is the unified entity for knowledge bases (NaiveRAG) and
// knowledge graphs. The `Category` field determines which engine processes it.
type KnowledgeAsset struct {
	model.BaseModel
	Name                string     `json:"name" gorm:"size:128;not null"`
	Description         string     `json:"description" gorm:"type:text"`
	Category            string     `json:"category" gorm:"size:8;not null;index"` // kb | kg
	Type                string     `json:"type" gorm:"size:32;not null"`          // naive_chunk, concept_map, …
	Status              string     `json:"status" gorm:"size:16;not null;default:idle"`
	ConfigData          string     `json:"-" gorm:"column:config;type:text"` // JSON, type-specific
	CompileModelID      *uint      `json:"compileModelId" gorm:"index"`      // LLM model (kg only)
	EmbeddingProviderID *uint      `json:"embeddingProviderId" gorm:"index"`
	EmbeddingModelID    string     `json:"embeddingModelId" gorm:"size:128"`
	AutoBuild           bool       `json:"autoBuild" gorm:"not null;default:false"`
	SourceCount         int        `json:"sourceCount" gorm:"not null;default:0"`
	BuiltAt             *time.Time `json:"builtAt"`
	ProgressData        string     `json:"-" gorm:"column:build_progress;type:text"` // JSON
}

func (KnowledgeAsset) TableName() string { return "ai_knowledge_assets" }

// ----- Config helpers -----

// GetConfig unmarshals the JSON config into the given target.
func (a *KnowledgeAsset) GetConfig(target any) error {
	if a.ConfigData == "" {
		return nil
	}
	return json.Unmarshal([]byte(a.ConfigData), target)
}

// SetConfig marshals the given config to JSON and stores it.
func (a *KnowledgeAsset) SetConfig(cfg any) error {
	if cfg == nil {
		a.ConfigData = ""
		return nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	a.ConfigData = string(data)
	return nil
}

// ----- Progress helpers -----

// BuildProgress tracks real-time build/compile progress.
type BuildProgress struct {
	Stage       string          `json:"stage"`
	Sources     ProgressCounter `json:"sources"`
	Items       ProgressCounter `json:"items"` // chunks or nodes
	Embeddings  ProgressCounter `json:"embeddings"`
	CurrentItem string          `json:"currentItem"`
	StartedAt   int64           `json:"startedAt"`
}

func (a *KnowledgeAsset) GetBuildProgress() *BuildProgress {
	if a.ProgressData == "" {
		return nil
	}
	var p BuildProgress
	if err := json.Unmarshal([]byte(a.ProgressData), &p); err != nil {
		return nil
	}
	return &p
}

func (a *KnowledgeAsset) SetBuildProgress(p *BuildProgress) error {
	if p == nil {
		a.ProgressData = ""
		return nil
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	a.ProgressData = string(data)
	return nil
}

// ----- Graph-specific config (backward compat with CompileConfig) -----

// GraphConfig holds configurable parameters for knowledge graph compilation.
type GraphConfig struct {
	TargetContentLength int `json:"targetContentLength"` // default 4000
	MinContentLength    int `json:"minContentLength"`    // default 200
	MaxChunkSize        int `json:"maxChunkSize"`        // 0 = auto
}

func DefaultGraphConfig() GraphConfig {
	return GraphConfig{
		TargetContentLength: 4000,
		MinContentLength:    200,
		MaxChunkSize:        0,
	}
}

// ----- RAG-specific config -----

// RAGConfig holds configurable parameters for RAG knowledge base building.
type RAGConfig struct {
	ChunkSize    int `json:"chunkSize"`    // target chunk size in chars, default 512
	ChunkOverlap int `json:"chunkOverlap"` // overlap between chunks, default 64
}

func DefaultRAGConfig() RAGConfig {
	return RAGConfig{
		ChunkSize:    512,
		ChunkOverlap: 64,
	}
}

// ----- Response DTO -----

type KnowledgeAssetResponse struct {
	ID                  uint            `json:"id"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	Category            string          `json:"category"`
	Type                string          `json:"type"`
	Status              string          `json:"status"`
	Config              json.RawMessage `json:"config,omitempty"`
	CompileModelID      *uint           `json:"compileModelId"`
	EmbeddingProviderID *uint           `json:"embeddingProviderId"`
	EmbeddingModelID    string          `json:"embeddingModelId"`
	AutoBuild           bool            `json:"autoBuild"`
	SourceCount         int             `json:"sourceCount"`
	BuiltAt             *time.Time      `json:"builtAt"`
	BuildProgress       *BuildProgress  `json:"buildProgress,omitempty"`
	// Dynamic counts (filled by handler)
	NodeCount  int `json:"nodeCount,omitempty"`
	EdgeCount  int `json:"edgeCount,omitempty"`
	ChunkCount int `json:"chunkCount,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (a *KnowledgeAsset) ToResponse() KnowledgeAssetResponse {
	resp := KnowledgeAssetResponse{
		ID:                  a.ID,
		Name:                a.Name,
		Description:         a.Description,
		Category:            a.Category,
		Type:                a.Type,
		Status:              a.Status,
		CompileModelID:      a.CompileModelID,
		EmbeddingProviderID: a.EmbeddingProviderID,
		EmbeddingModelID:    a.EmbeddingModelID,
		AutoBuild:           a.AutoBuild,
		SourceCount:         a.SourceCount,
		BuiltAt:             a.BuiltAt,
		CreatedAt:           a.CreatedAt,
		UpdatedAt:           a.UpdatedAt,
	}
	if a.ConfigData != "" {
		resp.Config = json.RawMessage(a.ConfigData)
	}
	if p := a.GetBuildProgress(); p != nil {
		resp.BuildProgress = p
	}
	return resp
}

// =====================================================================
// KnowledgeSource — independent source pool (no kb_id foreign key)
// =====================================================================

// KnowledgeSource2 is the new independent source entity.
// Named with suffix "2" to avoid conflict during migration; will be renamed
// after old KnowledgeSource is removed.
type KnowledgeSource2 struct {
	model.BaseModel
	ParentID      *uint      `json:"parentId" gorm:"index"`
	Title         string     `json:"title" gorm:"size:256;not null"`
	Content       string     `json:"-" gorm:"type:longtext"`
	Format        string     `json:"format" gorm:"size:16;not null"`
	SourceURL     string     `json:"sourceUrl" gorm:"size:1024"`
	CrawlDepth    int        `json:"crawlDepth" gorm:"not null;default:0"`
	URLPattern    string     `json:"urlPattern" gorm:"size:512"`
	CrawlEnabled  bool       `json:"crawlEnabled" gorm:"not null;default:false"`
	CrawlSchedule string     `json:"crawlSchedule" gorm:"size:64"`
	LastCrawledAt *time.Time `json:"lastCrawledAt"`
	FileName      string     `json:"fileName" gorm:"size:256"`
	ByteSize      int64      `json:"byteSize"`
	ExtractStatus string     `json:"extractStatus" gorm:"size:16;not null;default:pending"`
	ContentHash   string     `json:"-" gorm:"size:64"`
	ErrorMessage  string     `json:"errorMessage" gorm:"type:text"`
}

func (KnowledgeSource2) TableName() string { return "ai_knowledge_sources_v2" }

type KnowledgeSource2Response struct {
	ID            uint       `json:"id"`
	ParentID      *uint      `json:"parentId"`
	Title         string     `json:"title"`
	Format        string     `json:"format"`
	SourceURL     string     `json:"sourceUrl,omitempty"`
	CrawlDepth    int        `json:"crawlDepth"`
	CrawlEnabled  bool       `json:"crawlEnabled"`
	CrawlSchedule string     `json:"crawlSchedule,omitempty"`
	LastCrawledAt *time.Time `json:"lastCrawledAt,omitempty"`
	FileName      string     `json:"fileName,omitempty"`
	ByteSize      int64      `json:"byteSize"`
	ExtractStatus string     `json:"extractStatus"`
	ErrorMessage  string     `json:"errorMessage,omitempty"`
	RefCount      int        `json:"refCount"` // how many assets reference this source
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (s *KnowledgeSource2) ToResponse() KnowledgeSource2Response {
	return KnowledgeSource2Response{
		ID:            s.ID,
		ParentID:      s.ParentID,
		Title:         s.Title,
		Format:        s.Format,
		SourceURL:     s.SourceURL,
		CrawlDepth:    s.CrawlDepth,
		CrawlEnabled:  s.CrawlEnabled,
		CrawlSchedule: s.CrawlSchedule,
		LastCrawledAt: s.LastCrawledAt,
		FileName:      s.FileName,
		ByteSize:      s.ByteSize,
		ExtractStatus: s.ExtractStatus,
		ErrorMessage:  s.ErrorMessage,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
}

// =====================================================================
// KnowledgeAssetSource — M:N association between assets and sources
// =====================================================================

type KnowledgeAssetSource struct {
	AssetID  uint `json:"assetId" gorm:"primaryKey"`
	SourceID uint `json:"sourceId" gorm:"primaryKey"`
}

func (KnowledgeAssetSource) TableName() string { return "ai_knowledge_asset_sources" }

// =====================================================================
// RAGChunk — chunk storage for NaiveRAG knowledge bases
// =====================================================================

// RAGChunk stores a document chunk produced by a RAG knowledge base build.
type RAGChunk struct {
	model.BaseModel
	AssetID       uint   `json:"assetId" gorm:"not null;index"`
	SourceID      uint   `json:"sourceId" gorm:"not null;index"`
	Content       string `json:"content" gorm:"type:longtext;not null"`
	Summary       string `json:"summary" gorm:"type:text"`
	MetadataJSON  string `json:"-" gorm:"column:metadata;type:text"` // JSON
	ChunkIndex    int    `json:"chunkIndex" gorm:"not null;default:0"`
	ParentChunkID *uint  `json:"parentChunkId" gorm:"index"`
	// Embedding stored via pgvector or similar; column managed by migration
}

func (RAGChunk) TableName() string { return "ai_rag_chunks" }

type RAGChunkResponse struct {
	ID            uint            `json:"id"`
	AssetID       uint            `json:"assetId"`
	SourceID      uint            `json:"sourceId"`
	Content       string          `json:"content"`
	Summary       string          `json:"summary,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	ChunkIndex    int             `json:"chunkIndex"`
	ParentChunkID *uint           `json:"parentChunkId,omitempty"`
	Score         float64         `json:"score,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
}

func (c *RAGChunk) ToResponse() RAGChunkResponse {
	resp := RAGChunkResponse{
		ID:            c.ID,
		AssetID:       c.AssetID,
		SourceID:      c.SourceID,
		Content:       c.Content,
		Summary:       c.Summary,
		ChunkIndex:    c.ChunkIndex,
		ParentChunkID: c.ParentChunkID,
		CreatedAt:     c.CreatedAt,
	}
	if c.MetadataJSON != "" {
		resp.Metadata = json.RawMessage(c.MetadataJSON)
	}
	return resp
}

// =====================================================================
// AgentKnowledgeGraph — M:N binding between Agent and KG assets
// =====================================================================

type AgentKnowledgeGraph struct {
	AgentID          uint `json:"agentId" gorm:"primaryKey"`
	KnowledgeGraphID uint `json:"knowledgeGraphId" gorm:"primaryKey"`
}

func (AgentKnowledgeGraph) TableName() string { return "ai_agent_knowledge_graphs" }

// =====================================================================
// KnowledgeLog2 — compile/build log for new asset model
// =====================================================================

type KnowledgeLog2 struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	AssetID        uint      `json:"assetId" gorm:"not null;index"`
	Action         string    `json:"action" gorm:"size:32;not null"`
	ModelID        string    `json:"modelId" gorm:"size:128"`
	ItemsCreated   int       `json:"itemsCreated"` // nodes or chunks
	ItemsUpdated   int       `json:"itemsUpdated"`
	EdgesCreated   int       `json:"edgesCreated"`
	LintIssues     int       `json:"lintIssues"`
	Details        string    `json:"details" gorm:"type:text"`
	CascadeDetails string    `json:"cascadeDetails" gorm:"type:text"`
	ErrorMessage   string    `json:"errorMessage" gorm:"type:text"`
	CreatedAt      time.Time `json:"createdAt" gorm:"index"`
}

func (KnowledgeLog2) TableName() string { return "ai_knowledge_logs_v2" }

// =====================================================================
// Unified recall protocol
// =====================================================================

// KnowledgeUnit is the standard output unit returned by all engines.
type KnowledgeUnit struct {
	ID         string          `json:"id"`
	AssetID    uint            `json:"assetId"`
	UnitType   string          `json:"unitType"` // document_chunk, fact, entity, relation_summary
	Title      string          `json:"title"`
	Content    string          `json:"content"`
	Summary    string          `json:"summary,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	SourceRefs []uint          `json:"sourceRefs,omitempty"`
	Score      float64         `json:"score,omitempty"`
}

// KnowledgeRelation is an optional relation returned by graph engines.
type KnowledgeRelation struct {
	FromUnitID  string  `json:"fromUnitId"`
	ToUnitID    string  `json:"toUnitId"`
	Predicate   string  `json:"predicate"`
	Description string  `json:"description,omitempty"`
	Weight      float64 `json:"weight,omitempty"`
}

// RecallResult is the unified search response from any engine.
type RecallResult struct {
	Items     []KnowledgeUnit     `json:"items"`
	Relations []KnowledgeRelation `json:"relations,omitempty"`
	Sources   []SourceRef         `json:"sources,omitempty"`
	Debug     *RecallDebug        `json:"debug,omitempty"`
}

// SourceRef references an original source document.
type SourceRef struct {
	SourceID uint   `json:"sourceId"`
	Title    string `json:"title"`
	Format   string `json:"format"`
}

// RecallDebug contains optional debug information for search results.
type RecallDebug struct {
	Mode        string  `json:"mode"` // vector, fulltext, hybrid, graph_expand
	TotalHits   int     `json:"totalHits"`
	TimeTakenMs float64 `json:"timeTakenMs,omitempty"`
}
