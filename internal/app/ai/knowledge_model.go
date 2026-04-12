package ai

import (
	"encoding/json"
	"errors"
	"time"

	"metis/internal/model"
)

var (
	errEmbeddingNotConfigured = errors.New("embedding not configured for this knowledge base")
	errEmbeddingEmpty         = errors.New("embedding API returned empty result")
)

// Knowledge base compile statuses
const (
	CompileStatusIdle      = "idle"
	CompileStatusCompiling = "compiling"
	CompileStatusCompleted = "completed"
	CompileStatusError     = "error"
)

// Source formats
const (
	SourceFormatMarkdown = "markdown"
	SourceFormatText     = "text"
	SourceFormatPDF      = "pdf"
	SourceFormatDocx     = "docx"
	SourceFormatXlsx     = "xlsx"
	SourceFormatPptx     = "pptx"
	SourceFormatURL      = "url"
)

// Source extract statuses
const (
	ExtractStatusPending   = "pending"
	ExtractStatusCompleted = "completed"
	ExtractStatusError     = "error"
)

// Knowledge node types
const (
	NodeTypeIndex   = "index"
	NodeTypeConcept = "concept"
)

// Knowledge edge relation types
const (
	EdgeRelationRelated     = "related"
	EdgeRelationContradicts = "contradicts"
	EdgeRelationExtends     = "extends"
	EdgeRelationPartOf      = "part_of"
)

// Knowledge log actions
const (
	KnowledgeLogCompile   = "compile"
	KnowledgeLogRecompile = "recompile"
	KnowledgeLogCrawl     = "crawl"
	KnowledgeLogLint      = "lint"
)

// Compile methods
const (
	CompileMethodKnowledgeGraph = "knowledge_graph"
)

// Compile stages
const (
	CompileStagePreparing            = "preparing"
	CompileStageCallingLLM           = "calling_llm"
	CompileStageWritingNodes         = "writing_nodes"
	CompileStageGeneratingEmbeddings = "generating_embeddings"
	CompileStageCompleted            = "completed"
	CompileStageIdle                 = "idle"
)

// ProgressCounter tracks done/total for a specific phase
type ProgressCounter struct {
	Total int `json:"total"`
	Done  int `json:"done"`
}

// CompileProgress tracks real-time compilation progress
type CompileProgress struct {
	Stage       string          `json:"stage"`
	Sources     ProgressCounter `json:"sources"`
	Nodes       ProgressCounter `json:"nodes"`
	Embeddings  ProgressCounter `json:"embeddings"`
	CurrentItem string          `json:"currentItem"`
}

// --- KnowledgeBase ---

type KnowledgeBase struct {
	model.BaseModel
	Name                string     `json:"name" gorm:"size:128;not null"`
	Description         string     `json:"description" gorm:"type:text"`
	CompileStatus       string     `json:"compileStatus" gorm:"size:16;not null;default:idle"`
	CompileMethod       string     `json:"compileMethod" gorm:"size:64;not null;default:knowledge_graph"`
	CompileModelID      *uint      `json:"compileModelId" gorm:"index"`
	EmbeddingProviderID *uint      `json:"embeddingProviderId" gorm:"index"`
	EmbeddingModelID    string     `json:"embeddingModelId" gorm:"size:128"`
	CompiledAt          *time.Time `json:"compiledAt"`
	AutoCompile         bool       `json:"autoCompile" gorm:"not null;default:false"`
	SourceCount         int        `json:"sourceCount" gorm:"not null;default:0"`
	CompileProgressData string     `json:"-" gorm:"type:text"` // JSON stored in DB
}

// GetCompileProgress returns the parsed compile progress
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

// SetCompileProgress saves the compile progress as JSON
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

func (KnowledgeBase) TableName() string { return "ai_knowledge_bases" }

type KnowledgeBaseResponse struct {
	ID                  uint             `json:"id"`
	Name                string           `json:"name"`
	Description         string           `json:"description"`
	CompileStatus       string           `json:"compileStatus"`
	CompileMethod       string           `json:"compileMethod"`
	CompileModelID      *uint            `json:"compileModelId"`
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
	// Include compile progress if available
	if progress := kb.GetCompileProgress(); progress != nil {
		resp.CompileProgress = progress
	}
	return resp
}

// --- KnowledgeSource ---

type KnowledgeSource struct {
	model.BaseModel
	KbID          uint    `json:"kbId" gorm:"not null;index"`
	ParentID      *uint   `json:"parentId" gorm:"index"`
	Title         string  `json:"title" gorm:"size:256;not null"`
	Content       string  `json:"content" gorm:"type:text"`
	Format        string  `json:"format" gorm:"size:16;not null"`
	SourceURL     string  `json:"sourceUrl" gorm:"size:1024"`
	CrawlDepth    int     `json:"crawlDepth" gorm:"not null;default:0"`
	URLPattern    string     `json:"urlPattern" gorm:"size:512"`
	CrawlEnabled  bool       `json:"crawlEnabled" gorm:"not null;default:false"`
	CrawlSchedule string     `json:"crawlSchedule" gorm:"size:64"`
	LastCrawledAt *time.Time `json:"lastCrawledAt"`
	FileName      string     `json:"fileName" gorm:"size:256"`
	ByteSize      int64   `json:"byteSize"`
	ExtractStatus string  `json:"extractStatus" gorm:"size:16;not null;default:pending"`
	ContentHash   string  `json:"contentHash" gorm:"size:64"`
	ErrorMessage  string  `json:"errorMessage" gorm:"type:text"`
}

func (KnowledgeSource) TableName() string { return "ai_knowledge_sources" }

type KnowledgeSourceResponse struct {
	ID            uint       `json:"id"`
	KbID          uint       `json:"kbId"`
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
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (s *KnowledgeSource) ToResponse() KnowledgeSourceResponse {
	return KnowledgeSourceResponse{
		ID:            s.ID,
		KbID:          s.KbID,
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

// --- KnowledgeNode (FalkorDB) ---
// KnowledgeNode is a pure data struct representing a node in FalkorDB.
// It is NOT a GORM model — nodes are stored exclusively in FalkorDB graphs.

type KnowledgeNode struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	Content    *string   `json:"content"`
	NodeType   string    `json:"nodeType"`
	SourceIDs  string    `json:"sourceIds"`
	CompiledAt int64     `json:"compiledAt"`
}

type KnowledgeNodeResponse struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Summary    string          `json:"summary"`
	Content    *string         `json:"content,omitempty"`
	HasContent bool            `json:"hasContent"`
	NodeType   string          `json:"nodeType"`
	SourceIDs  json.RawMessage `json:"sourceIds"`
	EdgeCount  int             `json:"edgeCount"`
	CompiledAt int64           `json:"compiledAt"`
	Score      float64         `json:"score,omitempty"`
}

func (n *KnowledgeNode) ToResponse() KnowledgeNodeResponse {
	sourceIDs := json.RawMessage(n.SourceIDs)
	if len(sourceIDs) == 0 || string(sourceIDs) == "" {
		sourceIDs = json.RawMessage("[]")
	}
	return KnowledgeNodeResponse{
		ID:         n.ID,
		Title:      n.Title,
		Summary:    n.Summary,
		Content:    n.Content,
		HasContent: n.Content != nil && *n.Content != "",
		NodeType:   n.NodeType,
		SourceIDs:  sourceIDs,
		CompiledAt: n.CompiledAt,
	}
}

// --- KnowledgeEdge (FalkorDB) ---
// KnowledgeEdge is a pure data struct representing an edge in FalkorDB.

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

// --- KnowledgeLog ---

// CascadeDetail 记录单个节点的级联更新详情
type CascadeDetail struct {
	NodeTitle    string `json:"nodeTitle"`
	UpdateType   string `json:"updateType"` // "content" | "relationship" | "contradiction" | "merge"
	Reason       string `json:"reason"`
	SourcesAdded []uint `json:"sourcesAdded,omitempty"`
}

// CascadeLog 记录整个编译过程的级联更新情况
type CascadeLog struct {
	PrimaryNodes   []string        `json:"primaryNodes"`   // 本次编译主要涉及的新节点
	CascadeUpdates []CascadeDetail `json:"cascadeUpdates"` // 被级联更新的已有节点
}

type KnowledgeLog struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	KbID           uint      `json:"kbId" gorm:"not null;index"`
	Action         string    `json:"action" gorm:"size:32;not null"`
	ModelID        string    `json:"modelId" gorm:"size:128"`
	NodesCreated   int       `json:"nodesCreated"`
	NodesUpdated   int       `json:"nodesUpdated"`
	EdgesCreated   int       `json:"edgesCreated"`
	LintIssues     int       `json:"lintIssues"`
	Details        string    `json:"details" gorm:"type:text"`
	CascadeDetails string    `json:"cascadeDetails" gorm:"type:text"` // JSON 存储 CascadeLog
	ErrorMessage   string    `json:"errorMessage" gorm:"type:text"`
	CreatedAt      time.Time `json:"createdAt" gorm:"index"`
}

func (KnowledgeLog) TableName() string { return "ai_knowledge_logs" }
