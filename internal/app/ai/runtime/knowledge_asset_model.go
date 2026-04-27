package runtime

import (
	"encoding/json"
	"errors"
	"time"

	"metis/internal/model"
)

// =====================================================================
// Errors
// =====================================================================

var (
	ErrAssetNotFound          = errors.New("knowledge asset not found")
	ErrSourceNotFound         = errors.New("knowledge source not found")
	ErrChunkNotFound          = errors.New("knowledge chunk not found")
	errEmbeddingNotConfigured = errors.New("embedding not configured for this knowledge base")
	errEmbeddingEmpty         = errors.New("embedding API returned empty result")
)

// =====================================================================
// Asset category
// =====================================================================

const (
	AssetCategoryKB = "kb" // NaiveRAG knowledge base
	AssetCategoryKG = "kg" // Knowledge graph
)

// =====================================================================
// Asset types (strategies)
// =====================================================================

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

// =====================================================================
// Asset status
// =====================================================================

const (
	AssetStatusIdle     = "idle"
	AssetStatusBuilding = "building"
	AssetStatusReady    = "ready"
	AssetStatusError    = "error"
	AssetStatusStale    = "stale" // source changed, needs rebuild
)

// Legacy compile status aliases — used by existing compile/embedding services
// until they are fully migrated.
const (
	CompileStatusIdle      = AssetStatusIdle
	CompileStatusCompiling = AssetStatusBuilding
	CompileStatusCompleted = AssetStatusReady
	CompileStatusError     = AssetStatusError
)

// =====================================================================
// Source formats
// =====================================================================

const (
	SourceFormatMarkdown = "markdown"
	SourceFormatText     = "text"
	SourceFormatPDF      = "pdf"
	SourceFormatDocx     = "docx"
	SourceFormatXlsx     = "xlsx"
	SourceFormatPptx     = "pptx"
	SourceFormatURL      = "url"
)

// =====================================================================
// Source extract statuses
// =====================================================================

const (
	ExtractStatusPending   = "pending"
	ExtractStatusCompleted = "completed"
	ExtractStatusError     = "error"
)

// =====================================================================
// Build / compile stages
// =====================================================================

// RAG build stages
const (
	BuildStagePreparing = "preparing"
	BuildStageChunking  = "chunking"
	BuildStageEmbedding = "embedding"
	BuildStageIndexing  = "indexing"
	BuildStageCompleted = "completed"
	BuildStageIdle      = "idle"
)

// Graph compile stages (legacy aliases kept for existing compile service)
const (
	CompileStagePreparing            = "preparing"
	CompileStageMapping              = "mapping"
	CompileStageCallingLLM           = "calling_llm"
	CompileStageWritingNodes         = "writing_nodes"
	CompileStageGeneratingEmbeddings = "generating_embeddings"
	CompileStageCompleted            = "completed"
	CompileStageIdle                 = "idle"
)

// =====================================================================
// Knowledge node / edge types
// =====================================================================

const (
	NodeTypeConcept = "concept"
)

const (
	EdgeRelationRelated     = "related"
	EdgeRelationContradicts = "contradicts"
)

// =====================================================================
// Knowledge log actions
// =====================================================================

const (
	KnowledgeLogCompile   = "compile"
	KnowledgeLogRecompile = "recompile"
	KnowledgeLogCrawl     = "crawl"
	KnowledgeLogLint      = "lint"
)

// Legacy compile method constant — used by existing compile service.
const (
	CompileMethodKnowledgeGraph = "knowledge_graph"
)

// =====================================================================
// ProgressCounter — shared by build progress
// =====================================================================

type ProgressCounter struct {
	Total int `json:"total"`
	Done  int `json:"done"`
}

// =====================================================================
// KnowledgeAsset — unified table for both KB and KG
// =====================================================================

type KnowledgeAsset struct {
	model.BaseModel
	Name                string     `json:"name" gorm:"size:128;not null"`
	Description         string     `json:"description" gorm:"type:text"`
	Category            string     `json:"category" gorm:"size:8;not null;index"` // kb | kg
	Type                string     `json:"type" gorm:"size:32;not null"`          // naive_chunk, concept_map, …
	Status              string     `json:"status" gorm:"size:16;not null;default:idle"`
	ConfigData          string     `json:"-" gorm:"column:config;type:text"`
	CompileModelID      *uint      `json:"compileModelId" gorm:"index"`
	EmbeddingProviderID *uint      `json:"embeddingProviderId" gorm:"index"`
	EmbeddingModelID    string     `json:"embeddingModelId" gorm:"size:128"`
	AutoBuild           bool       `json:"autoBuild" gorm:"not null;default:false"`
	SourceCount         int        `json:"sourceCount" gorm:"not null;default:0"`
	BuiltAt             *time.Time `json:"builtAt"`
	ProgressData        string     `json:"-" gorm:"column:build_progress;type:text"`
}

func (KnowledgeAsset) TableName() string { return "ai_knowledge_assets" }

func (a *KnowledgeAsset) GetConfig(target any) error {
	if a.ConfigData == "" {
		return nil
	}
	return json.Unmarshal([]byte(a.ConfigData), target)
}

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
	NodeCount           int             `json:"nodeCount,omitempty"`
	EdgeCount           int             `json:"edgeCount,omitempty"`
	ChunkCount          int             `json:"chunkCount,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
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
// GraphConfig — graph-specific compile config
// =====================================================================

type GraphConfig struct {
	TargetContentLength int `json:"targetContentLength"`
	MinContentLength    int `json:"minContentLength"`
	MaxChunkSize        int `json:"maxChunkSize"`
}

func DefaultGraphConfig() GraphConfig {
	return GraphConfig{
		TargetContentLength: 4000,
		MinContentLength:    200,
		MaxChunkSize:        0,
	}
}

// --- Legacy aliases for existing compile service ---

type CompileConfig = GraphConfig

func DefaultCompileConfig() CompileConfig { return DefaultGraphConfig() }

// =====================================================================
// RAGConfig — RAG-specific build config
// =====================================================================

type RAGConfig struct {
	ChunkSize    int `json:"chunkSize"`
	ChunkOverlap int `json:"chunkOverlap"`
}

func DefaultRAGConfig() RAGConfig {
	return RAGConfig{ChunkSize: 512, ChunkOverlap: 64}
}

// =====================================================================
// KnowledgeSource — independent source pool
// =====================================================================

type KnowledgeSource struct {
	model.BaseModel
	ParentID      *uint      `json:"parentId" gorm:"index"`
	Title         string     `json:"title" gorm:"size:256;not null"`
	Content       string     `json:"-" gorm:"type:text"`
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

func (KnowledgeSource) TableName() string { return "ai_knowledge_sources" }

type KnowledgeSourceResponse struct {
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
	RefCount      int        `json:"refCount"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (s *KnowledgeSource) ToResponse() KnowledgeSourceResponse {
	return KnowledgeSourceResponse{
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
// KnowledgeAssetSource — M:N association
// =====================================================================

type KnowledgeAssetSource struct {
	AssetID  uint `json:"assetId" gorm:"primaryKey"`
	SourceID uint `json:"sourceId" gorm:"primaryKey"`
}

func (KnowledgeAssetSource) TableName() string { return "ai_knowledge_asset_sources" }

// =====================================================================
// RAGChunk — chunk storage for NaiveRAG
// =====================================================================

type RAGChunk struct {
	model.BaseModel
	AssetID       uint   `json:"assetId" gorm:"not null;index"`
	SourceID      uint   `json:"sourceId" gorm:"not null;index"`
	Content       string `json:"content" gorm:"type:text;not null"`
	Summary       string `json:"summary" gorm:"type:text"`
	MetadataJSON  string `json:"-" gorm:"column:metadata;type:text"`
	ChunkIndex    int    `json:"chunkIndex" gorm:"not null;default:0"`
	ParentChunkID *uint  `json:"parentChunkId" gorm:"index"`
	EmbeddingJSON string `json:"-" gorm:"column:embedding;type:text"` // JSON-encoded []float32
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
// AgentKnowledgeGraph — M:N binding Agent ↔ KG
// =====================================================================

type AgentKnowledgeGraph struct {
	AgentID          uint `json:"agentId" gorm:"primaryKey"`
	KnowledgeGraphID uint `json:"knowledgeGraphId" gorm:"primaryKey"`
}

func (AgentKnowledgeGraph) TableName() string { return "ai_agent_knowledge_graphs" }

// =====================================================================
// KnowledgeNode — FalkorDB graph node (NOT a GORM model)
// =====================================================================

type KnowledgeNode struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Content     string `json:"content"`
	NodeType    string `json:"nodeType"`
	SourceIDs   string `json:"sourceIds"`
	Keywords    string `json:"keywords"`
	CitationMap string `json:"citationMap"`
	CompiledAt  int64  `json:"compiledAt"`
}

type KnowledgeNodeResponse struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Summary     string          `json:"summary"`
	Content     string          `json:"content,omitempty"`
	HasContent  bool            `json:"hasContent"`
	NodeType    string          `json:"nodeType"`
	SourceIDs   json.RawMessage `json:"sourceIds"`
	Keywords    json.RawMessage `json:"keywords"`
	CitationMap json.RawMessage `json:"citationMap,omitempty"`
	EdgeCount   int             `json:"edgeCount"`
	CompiledAt  int64           `json:"compiledAt"`
	Score       float64         `json:"score,omitempty"`
}

func (n *KnowledgeNode) ToResponse() KnowledgeNodeResponse {
	sourceIDs := json.RawMessage(n.SourceIDs)
	if len(sourceIDs) == 0 || string(sourceIDs) == "" {
		sourceIDs = json.RawMessage("[]")
	}
	keywords := json.RawMessage(n.Keywords)
	if len(keywords) == 0 || string(keywords) == "" {
		keywords = json.RawMessage("[]")
	}
	var citationMap json.RawMessage
	if n.CitationMap != "" {
		citationMap = json.RawMessage(n.CitationMap)
	}
	return KnowledgeNodeResponse{
		ID:          n.ID,
		Title:       n.Title,
		Summary:     n.Summary,
		Content:     n.Content,
		HasContent:  n.Content != "",
		NodeType:    n.NodeType,
		SourceIDs:   sourceIDs,
		Keywords:    keywords,
		CitationMap: citationMap,
		CompiledAt:  n.CompiledAt,
	}
}

// =====================================================================
// KnowledgeEdge — FalkorDB graph edge (NOT a GORM model)
// =====================================================================

type KnowledgeEdge struct {
	FromNodeID  string `json:"fromNodeId"`
	ToNodeID    string `json:"toNodeId"`
	Relation    string `json:"relation"`
	Description string `json:"description"`
}

type KnowledgeEdgeResponse struct {
	FromNodeID  string `json:"fromNodeId"`
	ToNodeID    string `json:"toNodeId"`
	Relation    string `json:"relation"`
	Description string `json:"description,omitempty"`
}

func (e *KnowledgeEdge) ToResponse() KnowledgeEdgeResponse {
	return KnowledgeEdgeResponse{
		FromNodeID:  e.FromNodeID,
		ToNodeID:    e.ToNodeID,
		Relation:    e.Relation,
		Description: e.Description,
	}
}

// =====================================================================
// KnowledgeLog — build/compile log
// =====================================================================

type CascadeDetail struct {
	NodeTitle    string `json:"nodeTitle"`
	UpdateType   string `json:"updateType"`
	Reason       string `json:"reason"`
	SourcesAdded []uint `json:"sourcesAdded,omitempty"`
}

type CascadeLog struct {
	PrimaryNodes   []string        `json:"primaryNodes"`
	CascadeUpdates []CascadeDetail `json:"cascadeUpdates"`
}

type KnowledgeLog struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	AssetID        uint      `json:"assetId" gorm:"not null;index"`
	Action         string    `json:"action" gorm:"size:32;not null"`
	ModelID        string    `json:"modelId" gorm:"size:128"`
	NodesCreated   int       `json:"nodesCreated"`
	NodesUpdated   int       `json:"nodesUpdated"`
	EdgesCreated   int       `json:"edgesCreated"`
	LintIssues     int       `json:"lintIssues"`
	Details        string    `json:"details" gorm:"type:text"`
	CascadeDetails string    `json:"cascadeDetails" gorm:"type:text"`
	ErrorMessage   string    `json:"errorMessage" gorm:"type:text"`
	CreatedAt      time.Time `json:"createdAt" gorm:"index"`
}

func (KnowledgeLog) TableName() string { return "ai_knowledge_logs" }

// =====================================================================
// Unified recall protocol
// =====================================================================

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

type KnowledgeRelation struct {
	FromUnitID  string  `json:"fromUnitId"`
	ToUnitID    string  `json:"toUnitId"`
	Predicate   string  `json:"predicate"`
	Description string  `json:"description,omitempty"`
	Weight      float64 `json:"weight,omitempty"`
}

type RecallResult struct {
	Items     []KnowledgeUnit     `json:"items"`
	Relations []KnowledgeRelation `json:"relations,omitempty"`
	Sources   []SourceRef         `json:"sources,omitempty"`
	Debug     *RecallDebug        `json:"debug,omitempty"`
}

type SourceRef struct {
	SourceID uint   `json:"sourceId"`
	Title    string `json:"title"`
	Format   string `json:"format"`
}

type RecallDebug struct {
	Mode        string  `json:"mode"`
	TotalHits   int     `json:"totalHits"`
	TimeTakenMs float64 `json:"timeTakenMs,omitempty"`
}

// =====================================================================
// Legacy KnowledgeBase — kept temporarily for existing compile/handler code
// Will be removed after full migration.
// =====================================================================

type KnowledgeBase struct {
	model.BaseModel
	Name                string     `json:"name" gorm:"size:128;not null"`
	Description         string     `json:"description" gorm:"type:text"`
	CompileStatus       string     `json:"compileStatus" gorm:"size:16;not null;default:idle"`
	CompileMethod       string     `json:"compileMethod" gorm:"size:64;not null;default:knowledge_graph"`
	CompileModelID      *uint      `json:"compileModelId" gorm:"index"`
	CompileConfigData   string     `json:"-" gorm:"column:compile_config;type:text"`
	EmbeddingProviderID *uint      `json:"embeddingProviderId" gorm:"index"`
	EmbeddingModelID    string     `json:"embeddingModelId" gorm:"size:128"`
	CompiledAt          *time.Time `json:"compiledAt"`
	AutoCompile         bool       `json:"autoCompile" gorm:"not null;default:false"`
	SourceCount         int        `json:"sourceCount" gorm:"not null;default:0"`
	CompileProgressData string     `json:"-" gorm:"type:text"`
}

func (KnowledgeBase) TableName() string { return "ai_knowledge_bases" }

// CompileProgress tracks real-time compilation progress (legacy).
type CompileProgress struct {
	Stage       string          `json:"stage"`
	Sources     ProgressCounter `json:"sources"`
	Nodes       ProgressCounter `json:"nodes"`
	Embeddings  ProgressCounter `json:"embeddings"`
	CurrentItem string          `json:"currentItem"`
	StartedAt   int64           `json:"startedAt"`
}

func (kb *KnowledgeBase) GetCompileConfig() CompileConfig {
	if kb.CompileConfigData == "" {
		return DefaultCompileConfig()
	}
	var cfg CompileConfig
	if err := json.Unmarshal([]byte(kb.CompileConfigData), &cfg); err != nil {
		return DefaultCompileConfig()
	}
	if cfg.TargetContentLength <= 0 {
		cfg.TargetContentLength = 4000
	}
	if cfg.MinContentLength <= 0 {
		cfg.MinContentLength = 200
	}
	return cfg
}

func (kb *KnowledgeBase) SetCompileConfig(cfg *CompileConfig) error {
	if cfg == nil {
		kb.CompileConfigData = ""
		return nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	kb.CompileConfigData = string(data)
	return nil
}

func (kb *KnowledgeBase) GetCompileProgress() *CompileProgress {
	if kb.CompileProgressData == "" {
		return nil
	}
	var progress CompileProgress
	if err := json.Unmarshal([]byte(kb.CompileProgressData), &progress); err != nil {
		return nil
	}
	return &progress
}

func (kb *KnowledgeBase) SetCompileProgress(progress *CompileProgress) error {
	if progress == nil {
		kb.CompileProgressData = ""
		return nil
	}
	data, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	kb.CompileProgressData = string(data)
	return nil
}

type KnowledgeBaseResponse struct {
	ID                  uint             `json:"id"`
	Name                string           `json:"name"`
	Description         string           `json:"description"`
	CompileStatus       string           `json:"compileStatus"`
	CompileMethod       string           `json:"compileMethod"`
	CompileModelID      *uint            `json:"compileModelId"`
	CompileConfig       *CompileConfig   `json:"compileConfig,omitempty"`
	EmbeddingProviderID *uint            `json:"embeddingProviderId"`
	EmbeddingModelID    string           `json:"embeddingModelId"`
	CompiledAt          *time.Time       `json:"compiledAt"`
	AutoCompile         bool             `json:"autoCompile"`
	SourceCount         int              `json:"sourceCount"`
	NodeCount           int              `json:"nodeCount"`
	EdgeCount           int              `json:"edgeCount"`
	CompileProgress     *CompileProgress `json:"compileProgress,omitempty"`
	CreatedAt           time.Time        `json:"createdAt"`
	UpdatedAt           time.Time        `json:"updatedAt"`
}

func (kb *KnowledgeBase) ToResponse() KnowledgeBaseResponse {
	resp := KnowledgeBaseResponse{
		ID:                  kb.ID,
		Name:                kb.Name,
		Description:         kb.Description,
		CompileStatus:       kb.CompileStatus,
		CompileMethod:       kb.CompileMethod,
		CompileModelID:      kb.CompileModelID,
		EmbeddingProviderID: kb.EmbeddingProviderID,
		EmbeddingModelID:    kb.EmbeddingModelID,
		CompiledAt:          kb.CompiledAt,
		AutoCompile:         kb.AutoCompile,
		SourceCount:         kb.SourceCount,
		CreatedAt:           kb.CreatedAt,
		UpdatedAt:           kb.UpdatedAt,
	}
	if kb.CompileConfigData != "" {
		cfg := kb.GetCompileConfig()
		resp.CompileConfig = &cfg
	}
	if progress := kb.GetCompileProgress(); progress != nil {
		resp.CompileProgress = progress
	}
	return resp
}
