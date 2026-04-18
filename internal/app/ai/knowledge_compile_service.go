package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/samber/do/v2"

	"metis/internal/llm"
	"metis/internal/pkg/crypto"
	"metis/internal/scheduler"
)

type KnowledgeCompileService struct {
	kbRepo       *KnowledgeBaseRepo
	sourceRepo   *KnowledgeSourceRepo
	graphRepo    *KnowledgeGraphRepo
	logRepo      *KnowledgeLogRepo
	modelRepo    *ModelRepo
	embeddingSvc *KnowledgeEmbeddingService
	encKey       crypto.EncryptionKey
	engine       *scheduler.Engine
}

func NewKnowledgeCompileService(i do.Injector) (*KnowledgeCompileService, error) {
	return &KnowledgeCompileService{
		kbRepo:       do.MustInvoke[*KnowledgeBaseRepo](i),
		sourceRepo:   do.MustInvoke[*KnowledgeSourceRepo](i),
		graphRepo:    do.MustInvoke[*KnowledgeGraphRepo](i),
		logRepo:      do.MustInvoke[*KnowledgeLogRepo](i),
		modelRepo:    do.MustInvoke[*ModelRepo](i),
		embeddingSvc: do.MustInvoke[*KnowledgeEmbeddingService](i),
		encKey:       do.MustInvoke[crypto.EncryptionKey](i),
		engine:       do.MustInvoke[*scheduler.Engine](i),
	}, nil
}

// --- Map Phase Types ---

// relationOutput represents a named relation with an optional description.
// Supports flexible unmarshaling: accepts both "string" and {"name":"...","description":"..."}.
type relationOutput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (r *relationOutput) UnmarshalJSON(data []byte) error {
	// Try string first (e.g. "Concept Name")
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.Name = s
		r.Description = ""
		return nil
	}
	// Otherwise expect object {"name": "...", "description": "..."}
	type alias relationOutput
	var obj alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*r = relationOutput(obj)
	return nil
}

type mapNodeOutput struct {
	Title      string           `json:"title"`
	Summary    string           `json:"summary"`
	Content    string           `json:"content"`
	Keywords   []string         `json:"keywords"`
	References []relationOutput `json:"references"`
}

type mapResult struct {
	Nodes []mapNodeOutput `json:"nodes"`
}

type mapSourceResult struct {
	SourceTitle string
	SourceID    uint
	Nodes       []mapNodeOutput
	Error       error
}

// --- LLM Output Schema ---

type compileOutput struct {
	Nodes        []compileNodeOutput `json:"nodes"`
	UpdatedNodes []compileNodeOutput `json:"updated_nodes"`
}

type compileNodeOutput struct {
	Title        string            `json:"title"`
	Summary      string            `json:"summary"`
	Content      string            `json:"content"`
	Keywords     []string          `json:"keywords"`
	CitationMap  map[string]string `json:"citation_map,omitempty"`
	References   []relationOutput  `json:"references"`
	Contradicts  []relationOutput  `json:"contradicts"`
	Sources      []string          `json:"sources"`
	UpdateReason string            `json:"update_reason,omitempty"`
}

type compilePayload struct {
	KbID      uint `json:"kbId"`
	Recompile bool `json:"recompile"`
}

// --- Cascade Analysis ---

// cascadeAnalysis holds the impact analysis result of new sources on existing nodes
type cascadeAnalysis struct {
	// Key concepts extracted from new sources
	NewConcepts []string `json:"newConcepts"`
	// High-impact: nodes likely to need updates (title match or directly related)
	HighImpactNodes []KnowledgeNode `json:"highImpactNodes"`
	// Medium-impact: nodes related to high-impact nodes via edges
	MediumImpactNodes []KnowledgeNode `json:"mediumImpactNodes"`
	// Reference nodes: all other existing nodes
	ReferenceNodes []KnowledgeNode `json:"referenceNodes"`
}

// cascadeMatch records why a node is considered affected
type cascadeMatch struct {
	Node      KnowledgeNode
	MatchType string // "title_similarity" | "keyword_match" | "related_to_new"
	Reason    string
}

// HandleCompile is the scheduler handler for knowledge compilation.
// It uses a Map-Reduce pipeline: Map extracts concepts per source, Reduce merges them.
func (s *KnowledgeCompileService) HandleCompile(ctx context.Context, payload json.RawMessage) error {
	var p compilePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	kb, err := s.kbRepo.FindByID(p.KbID)
	if err != nil {
		return fmt.Errorf("find kb %d: %w", p.KbID, err)
	}

	// Initialize progress tracking
	progress := &CompileProgress{
		Stage:      CompileStagePreparing,
		Sources:    ProgressCounter{Total: 0, Done: 0},
		Nodes:      ProgressCounter{Total: 0, Done: 0},
		Embeddings: ProgressCounter{Total: 0, Done: 0},
		StartedAt:  time.Now().Unix(),
	}

	// If recompile, delete the entire FalkorDB graph
	if p.Recompile {
		if err := s.graphRepo.DeleteGraph(kb.ID); err != nil {
			return fmt.Errorf("delete graph for kb %d: %w", kb.ID, err)
		}
	}

	// Get completed sources
	sources, err := s.sourceRepo.FindCompletedByKbID(kb.ID)
	if err != nil {
		return fmt.Errorf("find sources: %w", err)
	}
	if len(sources) == 0 {
		kb.CompileStatus = CompileStatusError
		if updateErr := s.kbRepo.Update(kb); updateErr != nil {
			slog.Error("failed to update kb status", "kb_id", kb.ID, "error", updateErr)
		}
		return fmt.Errorf("no completed sources to compile")
	}

	// Update progress with sources count
	progress.Sources.Total = len(sources)
	progress.CurrentItem = fmt.Sprintf("准备编译 %d 个来源", len(sources))
	if err := s.updateProgress(kb, progress); err != nil {
		slog.Error("failed to update progress", "kb_id", kb.ID, "error", err)
	}

	// Get existing nodes for incremental compilation (from FalkorDB)
	existingNodes, _ := s.graphRepo.FindAllNodes(kb.ID)

	// Perform cascade impact analysis
	var cascade *cascadeAnalysis
	if !p.Recompile && len(existingNodes) > 0 {
		slog.Info("knowledge compile: analyzing cascade impact", "kb_id", kb.ID, "existing_nodes", len(existingNodes))
		cascade = s.analyzeCascadeImpact(sources, existingNodes, kb.ID)
		slog.Info("knowledge compile: cascade analysis done",
			"high_impact", len(cascade.HighImpactNodes),
			"medium_impact", len(cascade.MediumImpactNodes),
			"reference", len(cascade.ReferenceNodes))
	}

	// Drop vector index before writing (avoid HNSW concurrent write bug)
	if err := s.graphRepo.DropVectorIndex(kb.ID); err != nil {
		slog.Warn("failed to drop vector index", "kb_id", kb.ID, "error", err)
	}

	// Resolve the LLM client from the configured model
	llmClient, modelIDStr, err := s.resolveLLMClient(kb)
	if err != nil {
		kb.CompileStatus = CompileStatusError
		if updateErr := s.kbRepo.Update(kb); updateErr != nil {
			slog.Error("failed to update kb status", "kb_id", kb.ID, "error", updateErr)
		}
		return fmt.Errorf("resolve LLM client: %w", err)
	}

	// === Phase 1: MAP — extract concepts per source ===
	mapResults, err := s.runMapPhase(ctx, llmClient, modelIDStr, sources, kb, progress)
	if err != nil {
		kb.CompileStatus = CompileStatusError
		if updateErr := s.kbRepo.Update(kb); updateErr != nil {
			slog.Error("failed to update kb status", "kb_id", kb.ID, "error", updateErr)
		}
		if logErr := s.logRepo.Create(&KnowledgeLog{
			KbID:         kb.ID,
			Action:       KnowledgeLogCompile,
			ModelID:      modelIDStr,
			ErrorMessage: err.Error(),
		}); logErr != nil {
			slog.Error("knowledge compile: failed to create log", "kb_id", kb.ID, "error", logErr)
		}
		return fmt.Errorf("map phase: %w", err)
	}

	// === Phase 2: REDUCE — merge concepts via LLM ===
	output, err := s.runReducePhase(ctx, llmClient, modelIDStr, mapResults, existingNodes, cascade, kb, progress)
	if err != nil {
		kb.CompileStatus = CompileStatusError
		if updateErr := s.kbRepo.Update(kb); updateErr != nil {
			slog.Error("failed to update kb status", "kb_id", kb.ID, "error", updateErr)
		}
		if logErr := s.logRepo.Create(&KnowledgeLog{
			KbID:         kb.ID,
			Action:       KnowledgeLogCompile,
			ModelID:      modelIDStr,
			ErrorMessage: err.Error(),
		}); logErr != nil {
			slog.Error("knowledge compile: failed to create log", "kb_id", kb.ID, "error", logErr)
		}
		return fmt.Errorf("reduce phase: %w", err)
	}

	// === Phase 3: Write (unchanged) ===

	// Update progress with total node count from LLM
	progress.Stage = CompileStageWritingNodes
	totalNodes := len(output.Nodes) + len(output.UpdatedNodes)
	progress.Nodes.Total = totalNodes
	progress.Embeddings.Total = totalNodes
	progress.CurrentItem = fmt.Sprintf("准备创建 %d 个节点", totalNodes)
	if err := s.updateProgress(kb, progress); err != nil {
		slog.Error("failed to update progress", "kb_id", kb.ID, "error", err)
	}

	// Write nodes and edges to FalkorDB with progress updates
	onNodeProgress := func() {
		if err := s.updateProgress(kb, progress); err != nil {
			slog.Error("failed to update progress", "kb_id", kb.ID, "error", err)
		}
	}
	compileCfg := kb.GetCompileConfig()
	stats, cascadeDetails, err := s.writeCompileOutput(kb.ID, output, sources, compileCfg, progress, onNodeProgress)
	if err != nil {
		kb.CompileStatus = CompileStatusError
		if updateErr := s.kbRepo.Update(kb); updateErr != nil {
			slog.Error("failed to update kb status", "kb_id", kb.ID, "error", updateErr)
		}
		slog.Error("knowledge compile: write output failed", "kb_id", kb.ID, "error", err)
		return fmt.Errorf("write output: %w", err)
	}

	// Create full-text index
	if err := s.graphRepo.CreateFullTextIndex(kb.ID); err != nil {
		slog.Warn("failed to create full-text index", "kb_id", kb.ID, "error", err)
	}

	// Update progress before generating embeddings
	progress.Stage = CompileStageGeneratingEmbeddings
	progress.CurrentItem = "正在生成向量索引..."
	if err := s.updateProgress(kb, progress); err != nil {
		slog.Error("failed to update progress", "kb_id", kb.ID, "error", err)
	}

	// Generate embeddings and rebuild vector index
	if err := s.embeddingSvc.GenerateEmbeddings(ctx, kb.ID); err != nil {
		slog.Error("failed to generate embeddings", "kb_id", kb.ID, "error", err)
	}

	// Update embeddings progress
	progress.Embeddings.Done = progress.Embeddings.Total
	if err := s.updateProgress(kb, progress); err != nil {
		slog.Error("failed to update progress", "kb_id", kb.ID, "error", err)
	}

	// Run lint
	lintIssues := s.runLint(kb.ID)

	// Mark progress as completed
	progress.Stage = CompileStageCompleted
	progress.CurrentItem = "编译完成"
	if err := s.updateProgress(kb, progress); err != nil {
		slog.Error("failed to update progress", "kb_id", kb.ID, "error", err)
	}

	// Update KB status
	now := time.Now()
	kb.CompileStatus = CompileStatusCompleted
	kb.CompiledAt = &now
	kb.SetCompileProgress(nil)
	if err := s.kbRepo.Update(kb); err != nil {
		slog.Error("failed to update kb status after compile", "kb_id", kb.ID, "error", err)
	}
	if err := s.kbRepo.UpdateSourceCount(kb.ID); err != nil {
		slog.Error("failed to update kb source count", "kb_id", kb.ID, "error", err)
	}

	// Serialize cascade details
	var cascadeDetailsJSON string
	if cascadeDetails != nil {
		b, _ := json.Marshal(cascadeDetails)
		cascadeDetailsJSON = string(b)
	}

	// Write log with cascade details
	action := KnowledgeLogCompile
	if p.Recompile {
		action = KnowledgeLogRecompile
	}
	s.logRepo.Create(&KnowledgeLog{
		KbID:           kb.ID,
		Action:         action,
		ModelID:        modelIDStr,
		NodesCreated:   stats.created,
		NodesUpdated:   stats.updated,
		EdgesCreated:   stats.edges,
		LintIssues:     lintIssues,
		CascadeDetails: cascadeDetailsJSON,
	})

	cascadeCount := 0
	if cascadeDetails != nil {
		cascadeCount = len(cascadeDetails.CascadeUpdates)
	}
	slog.Info("knowledge compile: done", "kb_id", kb.ID, "created", stats.created, "updated", stats.updated, "edges", stats.edges, "lint", lintIssues, "cascade_updates", cascadeCount)
	return nil
}

// runMapPhase iterates each source, calls LLM to extract concepts, and returns results.
// Individual source failures are tolerated; only fails if ALL sources fail.
func (s *KnowledgeCompileService) runMapPhase(ctx context.Context, llmClient llm.Client, modelID string, sources []KnowledgeSource, kb *KnowledgeBase, progress *CompileProgress) ([]mapSourceResult, error) {
	progress.Stage = CompileStageMapping
	if err := s.updateProgress(kb, progress); err != nil {
		slog.Error("failed to update progress", "kb_id", kb.ID, "error", err)
	}

	cfg := kb.GetCompileConfig()

	// Auto-calculate maxChunkSize from model's context window if not explicitly set
	if cfg.MaxChunkSize <= 0 {
		cfg.MaxChunkSize = s.resolveAutoChunkSize(kb)
	}

	var results []mapSourceResult
	successCount := 0

	for i, src := range sources {
		progress.CurrentItem = fmt.Sprintf("分析来源 %d/%d: %s", i+1, len(sources), src.Title)
		if err := s.updateProgress(kb, progress); err != nil {
			slog.Error("failed to update progress", "kb_id", kb.ID, "error", err)
		}

		slog.Info("knowledge compile: map source", "kb_id", kb.ID, "source", src.Title, "index", i+1, "total", len(sources))

		result := s.mapSource(ctx, llmClient, modelID, src, cfg)
		results = append(results, result)

		if result.Error != nil {
			slog.Warn("knowledge compile: map source failed, skipping", "kb_id", kb.ID, "source", src.Title, "error", result.Error)
		} else {
			successCount++
			slog.Info("knowledge compile: map source done", "kb_id", kb.ID, "source", src.Title, "nodes", len(result.Nodes))
		}

		progress.Sources.Done = i + 1
		if err := s.updateProgress(kb, progress); err != nil {
			slog.Error("failed to update progress", "kb_id", kb.ID, "error", err)
		}
	}

	if successCount == 0 {
		return nil, fmt.Errorf("all %d sources failed in map phase", len(sources))
	}

	slog.Info("knowledge compile: map phase done", "kb_id", kb.ID, "success", successCount, "failed", len(sources)-successCount)
	return results, nil
}

// mapSource calls LLM to extract concepts from a single source.
// If the source exceeds maxChunkSize, it delegates to the long-doc pipeline.
func (s *KnowledgeCompileService) mapSource(ctx context.Context, llmClient llm.Client, modelID string, src KnowledgeSource, cfg CompileConfig) mapSourceResult {
	maxSize := cfg.MaxChunkSize
	if maxSize <= 0 {
		maxSize = 12000
	}

	// Long-doc path: source exceeds maxChunkSize → three-phase pipeline
	if len(src.Content) > maxSize {
		slog.Info("knowledge compile: source exceeds maxChunkSize, using long-doc pipeline", "source", src.Title, "content_len", len(src.Content), "max_chunk", maxSize)
		return s.mapSourceLongDoc(ctx, llmClient, modelID, src, cfg)
	}

	// Fast path: source fits in one chunk
	prompt := fmt.Sprintf("## Source: %s\n\n%s", src.Title, src.Content)
	systemPrompt := fmt.Sprintf(mapSystemPrompt, cfg.MinContentLength, cfg.TargetContentLength)

	resp, err := llmClient.Chat(ctx, llm.ChatRequest{
		Model:     modelID,
		MaxTokens: 16384,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: prompt},
		},
	})
	if err != nil {
		return mapSourceResult{SourceTitle: src.Title, SourceID: src.ID, Error: fmt.Errorf("LLM call: %w", err)}
	}

	jsonStr := llm.ExtractJSON(resp.Content)
	var mr mapResult
	if err := json.Unmarshal([]byte(jsonStr), &mr); err != nil {
		return mapSourceResult{SourceTitle: src.Title, SourceID: src.ID, Error: fmt.Errorf("parse JSON: %w (preview: %.200s)", err, jsonStr)}
	}

	return mapSourceResult{
		SourceTitle: src.Title,
		SourceID:    src.ID,
		Nodes:       mr.Nodes,
	}
}

// runReducePhase merges all Map results via LLM and returns the final compileOutput.
func (s *KnowledgeCompileService) runReducePhase(ctx context.Context, llmClient llm.Client, modelID string, mapResults []mapSourceResult, existingNodes []KnowledgeNode, cascade *cascadeAnalysis, kb *KnowledgeBase, progress *CompileProgress) (*compileOutput, error) {
	progress.Stage = CompileStageCallingLLM
	progress.CurrentItem = "AI 正在合并知识图谱..."
	if err := s.updateProgress(kb, progress); err != nil {
		slog.Error("failed to update progress", "kb_id", kb.ID, "error", err)
	}

	cfg := kb.GetCompileConfig()
	prompt := s.buildReducePrompt(mapResults, existingNodes, cascade)

	slog.Info("knowledge compile: reduce phase calling LLM", "kb_id", kb.ID, "map_results", len(mapResults), "existing_nodes", len(existingNodes))

	systemPrompt := fmt.Sprintf(compileSystemPrompt, cfg.TargetContentLength, cfg.MinContentLength)

	resp, err := llmClient.Chat(ctx, llm.ChatRequest{
		Model:     modelID,
		MaxTokens: 16384,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: prompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call: %w", err)
	}

	output, err := s.parseLLMOutput(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parse output: %w", err)
	}

	return output, nil
}

// buildReducePrompt builds the Reduce phase prompt from Map results and existing graph state.
func (s *KnowledgeCompileService) buildReducePrompt(mapResults []mapSourceResult, existingNodes []KnowledgeNode, analysis *cascadeAnalysis) string {
	var sb strings.Builder

	// Source index for citation markers
	sb.WriteString("## Source Index\n")
	idx := 1
	for _, mr := range mapResults {
		if mr.Error != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("[S%d] = \"%s\"\n", idx, mr.SourceTitle))
		idx++
	}
	sb.WriteString("\n")

	sb.WriteString("## Extracted concepts from sources\n\n")
	for _, mr := range mapResults {
		if mr.Error != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("### From \"%s\":\n", mr.SourceTitle))
		for _, n := range mr.Nodes {
			sb.WriteString(fmt.Sprintf("- **%s**: %s", n.Title, n.Summary))
			if len(n.References) > 0 {
				names := make([]string, len(n.References))
				for i, r := range n.References {
					names[i] = r.Name
				}
				sb.WriteString(fmt.Sprintf(" [references: %s]", strings.Join(names, ", ")))
			}
			if len(n.Keywords) > 0 {
				sb.WriteString(fmt.Sprintf(" [keywords: %s]", strings.Join(n.Keywords, ", ")))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Impact analysis section (same logic as old buildCompilePrompt)
	if analysis != nil && len(existingNodes) > 0 {
		sb.WriteString("## Impact Analysis\n\n")

		if len(analysis.NewConcepts) > 0 {
			sb.WriteString("Key concepts from new sources: ")
			for i, concept := range analysis.NewConcepts {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("\"%s\"", concept))
			}
			sb.WriteString("\n\n")
		}

		if len(analysis.HighImpactNodes) > 0 {
			sb.WriteString("### High-impact existing nodes (LIKELY NEED UPDATES)\n")
			sb.WriteString("These nodes are directly related to the new sources:\n\n")
			for _, n := range analysis.HighImpactNodes {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", n.Title, n.Summary))
			}
			sb.WriteString("\n")
		}

		if len(analysis.MediumImpactNodes) > 0 {
			sb.WriteString("### Medium-impact nodes (CHECK FOR NEW RELATIONSHIPS)\n")
			sb.WriteString("These nodes may be related to new concepts:\n\n")
			for _, n := range analysis.MediumImpactNodes {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", n.Title, n.Summary))
			}
			sb.WriteString("\n")
		}

		if len(analysis.ReferenceNodes) > 0 {
			sb.WriteString("### Reference only (unlikely affected)\n")
			sb.WriteString("For context only - these are less likely to need updates:\n\n")
			limit := len(analysis.ReferenceNodes)
			if limit > 20 {
				limit = 20
			}
			for i := 0; i < limit; i++ {
				n := analysis.ReferenceNodes[i]
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", n.Title, n.Summary))
			}
			if len(analysis.ReferenceNodes) > 20 {
				sb.WriteString(fmt.Sprintf("- ... and %d more nodes\n", len(analysis.ReferenceNodes)-20))
			}
			sb.WriteString("\n")
		}
	} else if len(existingNodes) > 0 {
		sb.WriteString("## Existing knowledge nodes\n\n")
		for _, n := range existingNodes {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", n.Title, n.Summary))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// resolveLLMClient looks up the model and provider configured for the KB,
// then builds an llm.Client using the provider credentials.
// resolveAutoChunkSize calculates maxChunkSize from the model's context window.
// A lower threshold means more documents use the long-doc pipeline (chunked processing),
// which produces better coverage than a single LLM call on a large input.
func (s *KnowledgeCompileService) resolveAutoChunkSize(kb *KnowledgeBase) int {
	const defaultChunkSize = 12000

	var m *AIModel
	var err error
	if kb.CompileModelID != nil {
		m, err = s.modelRepo.FindByID(*kb.CompileModelID)
	} else {
		m, err = s.modelRepo.FindDefaultByType(ModelTypeLLM)
	}
	if err != nil || m == nil || m.ContextWindow <= 0 {
		return defaultChunkSize
	}

	// Use 10% of context window as chunk size (conservative to avoid "lost in the middle").
	// Floor: 8K chars, Ceiling: 15K chars.
	autoSize := int(float64(m.ContextWindow) * 0.1)
	if autoSize < 8000 {
		autoSize = 8000
	}
	if autoSize > 15000 {
		autoSize = 15000
	}
	slog.Info("knowledge compile: auto chunk size from model context window", "model", m.ModelID, "context_window", m.ContextWindow, "chunk_size", autoSize)
	return autoSize
}

func (s *KnowledgeCompileService) resolveLLMClient(kb *KnowledgeBase) (llm.Client, string, error) {
	var m *AIModel
	var err error

	if kb.CompileModelID != nil {
		m, err = s.modelRepo.FindByID(*kb.CompileModelID)
	} else {
		m, err = s.modelRepo.FindDefaultByType(ModelTypeLLM)
	}
	if err != nil {
		return nil, "", fmt.Errorf("find LLM model: %w", err)
	}
	if m.Provider == nil {
		return nil, "", fmt.Errorf("model has no provider loaded")
	}

	apiKey, err := decryptAPIKey(m.Provider.APIKeyEncrypted, s.encKey)
	if err != nil {
		return nil, "", fmt.Errorf("decrypt api key: %w", err)
	}

	client, err := llm.NewClient(m.Provider.Protocol, m.Provider.BaseURL, apiKey)
	if err != nil {
		return nil, "", fmt.Errorf("create LLM client: %w", err)
	}

	return client, m.ModelID, nil
}

type compileStats struct {
	created int
	updated int
	edges   int
}

// updateProgress saves the current compile progress to the database
func (s *KnowledgeCompileService) updateProgress(kb *KnowledgeBase, progress *CompileProgress) error {
	if err := kb.SetCompileProgress(progress); err != nil {
		return err
	}
	return s.kbRepo.Update(kb)
}

func (s *KnowledgeCompileService) writeCompileOutput(kbID uint, output *compileOutput, sources []KnowledgeSource, cfg CompileConfig, progress *CompileProgress, onProgress func()) (*compileStats, *CascadeLog, error) {
	stats := &compileStats{}
	cascadeLog := &CascadeLog{
		PrimaryNodes:   make([]string, 0),
		CascadeUpdates: make([]CascadeDetail, 0),
	}
	now := time.Now().Unix()

	// Build source title → ID map
	sourceMap := make(map[string]uint)
	for _, src := range sources {
		sourceMap[src.Title] = src.ID
	}

	// Process new nodes — upsert by title into FalkorDB
	for i, n := range output.Nodes {
		// Discard nodes with insufficient content
		if len(n.Content) < cfg.MinContentLength {
			slog.Warn("discarding node with insufficient content", "title", n.Title, "content_len", len(n.Content), "min", cfg.MinContentLength)
			continue
		}
		node := &KnowledgeNode{
			Title:       n.Title,
			Summary:     n.Summary,
			Content:     n.Content,
			NodeType:    NodeTypeConcept,
			SourceIDs:   resolveSourceIDsJSON(n.Sources, sourceMap),
			Keywords:    toJSONString(n.Keywords),
			CitationMap: toJSONString(n.CitationMap),
			CompiledAt:  now,
		}
		if err := s.graphRepo.UpsertNodeByTitle(kbID, node); err != nil {
			slog.Error("failed to upsert node", "title", n.Title, "error", err)
			continue
		}
		stats.created++
		cascadeLog.PrimaryNodes = append(cascadeLog.PrimaryNodes, n.Title)
		progress.Nodes.Done = stats.created + stats.updated
		progress.CurrentItem = fmt.Sprintf("正在创建节点: %s", n.Title)
		if i%5 == 0 || i == len(output.Nodes)-1 {
			onProgress()
		}
	}

	// Process updated nodes
	for i, n := range output.UpdatedNodes {
		if len(n.Content) < cfg.MinContentLength {
			slog.Warn("discarding updated node with insufficient content", "title", n.Title, "content_len", len(n.Content), "min", cfg.MinContentLength)
			continue
		}
		node := &KnowledgeNode{
			Title:       n.Title,
			Summary:     n.Summary,
			Content:     n.Content,
			NodeType:    NodeTypeConcept,
			SourceIDs:   resolveSourceIDsJSON(n.Sources, sourceMap),
			Keywords:    toJSONString(n.Keywords),
			CitationMap: toJSONString(n.CitationMap),
			CompiledAt:  now,
		}
		if err := s.graphRepo.UpsertNodeByTitle(kbID, node); err != nil {
			slog.Error("failed to upsert updated node", "title", n.Title, "error", err)
			continue
		}
		stats.updated++

		// Record cascade update detail
		detail := CascadeDetail{
			NodeTitle:  n.Title,
			UpdateType: detectUpdateType(n.UpdateReason),
			Reason:     n.UpdateReason,
		}
		for _, srcTitle := range n.Sources {
			if srcID, ok := sourceMap[srcTitle]; ok {
				detail.SourcesAdded = append(detail.SourcesAdded, srcID)
			}
		}
		cascadeLog.CascadeUpdates = append(cascadeLog.CascadeUpdates, detail)

		progress.Nodes.Done = stats.created + stats.updated
		progress.CurrentItem = fmt.Sprintf("正在更新节点: %s", n.Title)
		if i%5 == 0 || i == len(output.UpdatedNodes)-1 {
			onProgress()
		}
	}

	// Resolve edges from all nodes (new + updated)
	allNodeOutputs := append(output.Nodes, output.UpdatedNodes...)
	for _, n := range allNodeOutputs {
		// Create "related" edges from references
		for _, ref := range n.References {
			// Only create edge if target node exists — no ghost nodes
			if _, err := s.graphRepo.FindNodeByTitle(kbID, ref.Name); err != nil {
				continue
			}
			if err := s.graphRepo.CreateEdge(kbID, n.Title, ref.Name, EdgeRelationRelated, ref.Description); err != nil {
				slog.Error("failed to create edge", "from", n.Title, "to", ref.Name, "error", err)
				continue
			}
			stats.edges++
		}
		// Create "contradicts" edges
		for _, contra := range n.Contradicts {
			if _, err := s.graphRepo.FindNodeByTitle(kbID, contra.Name); err != nil {
				continue
			}
			if err := s.graphRepo.CreateEdge(kbID, n.Title, contra.Name, EdgeRelationContradicts, contra.Description); err != nil {
				slog.Error("failed to create contradicts edge", "from", n.Title, "to", contra.Name, "error", err)
				continue
			}
			stats.edges++
		}
	}

	return stats, cascadeLog, nil
}

// toJSONString marshals any value to a JSON string. Returns "" on nil/empty input.
func toJSONString(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return ""
	}
	return string(b)
}

// detectUpdateType infers the type of update from the reason text
func detectUpdateType(reason string) string {
	reasonLower := strings.ToLower(reason)
	if strings.Contains(reasonLower, "contradict") {
		return "contradiction"
	}
	if strings.Contains(reasonLower, "relationship") || strings.Contains(reasonLower, "related") || strings.Contains(reasonLower, "link") {
		return "relationship"
	}
	if strings.Contains(reasonLower, "merge") {
		return "merge"
	}
	return "content"
}

func (s *KnowledgeCompileService) runLint(kbID uint) int {
	issues := 0

	orphans, _ := s.graphRepo.CountOrphanNodes(kbID)
	issues += int(orphans)

	contradictions, _ := s.graphRepo.CountContradictions(kbID)
	issues += int(contradictions)

	if issues > 0 {
		slog.Info("knowledge lint", "kb_id", kbID, "orphans", orphans, "contradictions", contradictions)
	}
	return issues
}

func (s *KnowledgeCompileService) parseLLMOutput(content string) (*compileOutput, error) {
	jsonStr := llm.ExtractJSON(content)
	var output compileOutput
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w (content preview: %.200s)", err, jsonStr)
	}
	return &output, nil
}

// analyzeCascadeImpact analyzes how new sources might affect existing nodes
func (s *KnowledgeCompileService) analyzeCascadeImpact(sources []KnowledgeSource, existingNodes []KnowledgeNode, kbID uint) *cascadeAnalysis {
	analysis := &cascadeAnalysis{
		NewConcepts:       extractKeyConcepts(sources),
		HighImpactNodes:   []KnowledgeNode{},
		MediumImpactNodes: []KnowledgeNode{},
		ReferenceNodes:    []KnowledgeNode{},
	}

	if len(existingNodes) == 0 || len(sources) == 0 {
		return analysis
	}

	// Track which nodes are high/medium impact
	highImpactIDs := make(map[string]bool)
	mediumImpactIDs := make(map[string]bool)

	// Check each existing node against new sources
	for _, node := range existingNodes {
		match := s.checkNodeImpact(node, sources, analysis.NewConcepts)
		if match != nil {
			if match.MatchType == "title_similarity" || match.MatchType == "keyword_match" {
				analysis.HighImpactNodes = append(analysis.HighImpactNodes, node)
				highImpactIDs[node.ID] = true
			}
		}
	}

	// For high-impact nodes, find related nodes via graph (would need graphRepo query)
	// For now, use simple heuristics: nodes that share keywords in summaries
	for _, node := range existingNodes {
		if highImpactIDs[node.ID] {
			continue
		}

		// Check if summary contains any of the new concepts
		for _, concept := range analysis.NewConcepts {
			if strings.Contains(strings.ToLower(node.Summary), strings.ToLower(concept)) {
				if !highImpactIDs[node.ID] && !mediumImpactIDs[node.ID] {
					analysis.MediumImpactNodes = append(analysis.MediumImpactNodes, node)
					mediumImpactIDs[node.ID] = true
					break
				}
			}
		}
	}

	// Remaining nodes are reference-only
	for _, node := range existingNodes {
		if !highImpactIDs[node.ID] && !mediumImpactIDs[node.ID] {
			analysis.ReferenceNodes = append(analysis.ReferenceNodes, node)
		}
	}

	return analysis
}

// checkNodeImpact checks if a node is likely affected by new sources
func (s *KnowledgeCompileService) checkNodeImpact(node KnowledgeNode, sources []KnowledgeSource, newConcepts []string) *cascadeMatch {
	nodeTitleLower := strings.ToLower(node.Title)

	// Check 1: Direct title match with source titles
	for _, src := range sources {
		srcTitleLower := strings.ToLower(src.Title)
		// Check if source title contains node title or vice versa
		if strings.Contains(srcTitleLower, nodeTitleLower) || strings.Contains(nodeTitleLower, srcTitleLower) {
			return &cascadeMatch{
				Node:      node,
				MatchType: "title_similarity",
				Reason:    fmt.Sprintf("Source title '%s' related to node title", src.Title),
			}
		}
	}

	// Check 2: Node title matches extracted concepts
	for _, concept := range newConcepts {
		conceptLower := strings.ToLower(concept)
		if strings.Contains(nodeTitleLower, conceptLower) || strings.Contains(conceptLower, nodeTitleLower) {
			return &cascadeMatch{
				Node:      node,
				MatchType: "keyword_match",
				Reason:    fmt.Sprintf("Node title matches concept '%s' from new sources", concept),
			}
		}
	}

	return nil
}

// extractKeyConcepts extracts key concepts from source content
// Simple implementation: extracts capitalized phrases and quoted terms
func extractKeyConcepts(sources []KnowledgeSource) []string {
	conceptSet := make(map[string]bool)

	for _, src := range sources {
		// Extract from title first (higher priority)
		extractFromText(src.Title, conceptSet)

		// Extract from content (first 2000 chars only for performance)
		content := src.Content
		if len(content) > 2000 {
			content = content[:2000]
		}
		extractFromText(content, conceptSet)
	}

	// Convert to slice
	concepts := make([]string, 0, len(conceptSet))
	for c := range conceptSet {
		if len(c) > 2 { // Filter out very short matches
			concepts = append(concepts, c)
		}
	}

	// Limit to top 20 concepts
	if len(concepts) > 20 {
		concepts = concepts[:20]
	}

	return concepts
}

// extractFromText extracts potential concept phrases from text
func extractFromText(text string, concepts map[string]bool) {
	// Pattern 1: Capitalized phrases (2-4 words)
	words := strings.Fields(text)
	for i := 0; i < len(words); i++ {
		// Check for capitalized word sequences
		if len(words[i]) > 0 && words[i][0] >= 'A' && words[i][0] <= 'Z' {
			phrase := words[i]
			for j := i + 1; j < len(words) && j < i+4; j++ {
				if len(words[j]) > 0 {
					phrase += " " + words[j]
					if j-i >= 1 {
						concepts[phrase] = true
					}
				}
			}
		}

		// Pattern 2: Words in quotes or backticks
		w := words[i]
		if strings.HasPrefix(w, "`") || strings.HasPrefix(w, "'") || strings.HasPrefix(w, "\"") {
			cleaned := strings.Trim(w, "`'\"" + ".,;:!?")
			if len(cleaned) > 2 {
				concepts[cleaned] = true
			}
		}
	}
}

// EnqueueCompile enqueues a knowledge base compilation task.
func (s *KnowledgeCompileService) EnqueueCompile(kbID uint, recompile bool) error {
	payload := compilePayload{KbID: kbID, Recompile: recompile}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.engine.Enqueue("ai-knowledge-compile", json.RawMessage(b))
}

func (s *KnowledgeCompileService) TaskDefs() []scheduler.TaskDef {
	return []scheduler.TaskDef{
		{
			Name:        "ai-knowledge-compile",
			Type:        scheduler.TypeAsync,
			Description: "Compile knowledge sources into knowledge graph using LLM",
			Timeout:     300 * time.Second,
			MaxRetries:  1,
			Handler:     s.HandleCompile,
		},
	}
}

const compileSystemPrompt = `You are a knowledge compiler. Merge extracted concepts into a knowledge graph of wiki-article nodes.

TASK: Given concept outlines from multiple sources and existing graph nodes, produce final wiki articles. Merge duplicate concepts, detect contradictions, and update existing nodes when new information is available.

OUTPUT SCHEMA (strict JSON):
{
  "nodes": [                              // NEW concepts only
    {
      "title": "Concept Name",
      "summary": "2-3 sentence description",
      "content": "Full Markdown article with [S1][S2] citation markers (REQUIRED, never empty)",
      "keywords": ["domain-specific", "search", "terms"],          // 3-8 keywords
      "citation_map": {"S1": "Source Title 1", "S2": "Source Title 2"},  // maps [S1] markers to source titles
      "references": [                                              // creates "related" edges
        {"name": "Other Concept", "description": "Why these concepts are related"}
      ],
      "contradicts": [],                                           // creates "contradicts" edges
      "sources": ["Source Title 1", "Source Title 2"]              // which source documents contributed
    }
  ],
  "updated_nodes": [                      // EXISTING concepts that need updates
    {
      "title": "Existing Concept Name",   // must match an existing node title exactly
      "summary": "Updated summary",
      "content": "Rewritten article incorporating new information, with [S1][S2] citations",
      "keywords": ["updated", "keywords"],
      "citation_map": {"S1": "Original Source", "S2": "New Source"},
      "references": [{"name": "Related Concept", "description": "Relationship explanation"}],
      "contradicts": [{"name": "Conflicting Concept", "description": "Nature of contradiction"}],
      "sources": ["Original Source", "New Source"],
      "update_reason": "What changed and why"
    }
  ]
}

FIELD RULES:
- "content": Target %d chars, minimum %d chars. Must be a complete, self-contained wiki article.
- "citation_map": Every [S1]-style marker in content MUST have a corresponding entry. Use the Source Index provided in the user message.
- "references"/"contradicts": Each entry MUST be {"name": "...", "description": "..."}. The description explains WHY the relationship exists.
- "keywords": Domain-specific terms useful for search. NOT generic words like "overview" or "important".

CASCADE RULES:
When the user message includes "High-impact existing nodes", those nodes LIKELY need updates:
1. Put updated existing nodes in "updated_nodes", NOT in "nodes"
2. Merge new source information into the existing article
3. Add contradictions if new sources conflict with existing knowledge
4. Explain what changed in "update_reason"

Only truly NEW concepts (not matching any existing node) go in "nodes".
If no updates are needed, leave "updated_nodes" as [].

BAD OUTPUT (will cause errors):
- "references": ["Concept A", "Concept B"]           ← WRONG: must be objects with name+description
- "content": ""                                        ← WRONG: content must never be empty
- "citation_map": {}  but content has [S1] markers    ← WRONG: every marker needs a map entry
- Putting an existing node title in "nodes" instead of "updated_nodes"

Output ONLY the JSON object, no markdown fences, no explanation.`

const mapSystemPrompt = `You are a knowledge extractor. Read a source document and extract distinct concept nodes.

TASK: For each concept with enough material, produce a self-contained wiki article.
IMPORTANT: You MUST process the ENTIRE document from beginning to end. Do NOT stop after the first section — extract concepts from ALL parts of the source.

OUTPUT SCHEMA (strict JSON):
{
  "nodes": [
    {
      "title": "Canonical Concept Name",   // descriptive, e.g. "React Hooks" not "hooks"
      "summary": "2-3 sentence description that captures the concept's essence and scope",
      "content": "Full wiki article in Markdown (REQUIRED, never empty or short)",
      "keywords": ["domain-specific", "search", "terms"],  // 3-8 keywords
      "references": ["Other Concept Name"]                  // names of related concepts mentioned in article
    }
  ]
}

RULES:
1. Only create nodes for concepts with enough material for at least %d characters of content.
2. Target article length: %d characters. Write substantive, detailed articles.
3. The "summary" field is critical — it must capture WHAT the concept is, WHY it matters, and its SCOPE. A good summary lets someone decide whether to read the full article.
4. "keywords" must be specific and domain-relevant (not generic words like "important" or "system").
5. "references" is a flat list of concept name strings — just the names, nothing else.
6. Scan the ENTIRE source — concepts from the middle and end of the document are just as important as those at the beginning.

DO NOT:
- Create nodes for concepts only briefly mentioned
- Output empty or near-empty content
- Use database IDs anywhere — use human-readable names only
- Skip sections of the source document

Output ONLY the JSON object, no markdown fences, no explanation.`

const scanSystemPrompt = `You are a knowledge scanner. Quickly identify distinct concepts in a text chunk.

OUTPUT SCHEMA (strict JSON):
{
  "concepts": [
    {
      "title": "Canonical Concept Name",
      "summary": "2-3 sentence description capturing essence and scope",
      "keywords": ["domain-specific", "terms"]   // 3-8 keywords for deduplication
    }
  ]
}

RULES:
1. Only identify concepts with enough material for a substantive article.
2. Use descriptive, canonical titles (e.g., "React Server Components" not "server components").
3. The summary is critical for downstream merging — be specific about scope and boundaries.
4. Skip trivially mentioned concepts.

Output ONLY the JSON object, no markdown fences, no explanation.`
