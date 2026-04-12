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

// --- LLM Output Schema ---

type compileOutput struct {
	Nodes        []compileNodeOutput `json:"nodes"`
	UpdatedNodes []compileNodeOutput `json:"updated_nodes"`
}

type compileNodeOutput struct {
	Title        string            `json:"title"`
	Summary      string            `json:"summary"`
	Content      *string           `json:"content"`
	Related      []compileRelation `json:"related"`
	Sources      []string          `json:"sources"`
	UpdateReason string            `json:"update_reason,omitempty"`
}

type compileRelation struct {
	Concept  string `json:"concept"`
	Relation string `json:"relation"`
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
	var cascadeAnalysis *cascadeAnalysis
	if !p.Recompile && len(existingNodes) > 0 {
		slog.Info("knowledge compile: analyzing cascade impact", "kb_id", kb.ID, "existing_nodes", len(existingNodes))
		cascadeAnalysis = s.analyzeCascadeImpact(sources, existingNodes, kb.ID)
		slog.Info("knowledge compile: cascade analysis done",
			"high_impact", len(cascadeAnalysis.HighImpactNodes),
			"medium_impact", len(cascadeAnalysis.MediumImpactNodes),
			"reference", len(cascadeAnalysis.ReferenceNodes))
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

	// Build the compilation prompt with cascade analysis
	prompt := s.buildCompilePrompt(sources, existingNodes, cascadeAnalysis)

	// Update progress before calling LLM
	progress.Stage = CompileStageCallingLLM
	progress.Sources.Done = len(sources)
	progress.CurrentItem = "AI 正在分析文档内容..."
	if err := s.updateProgress(kb, progress); err != nil {
		slog.Error("failed to update progress", "kb_id", kb.ID, "error", err)
	}

	// Call LLM
	slog.Info("knowledge compile: calling LLM", "kb_id", kb.ID, "sources", len(sources), "existing_nodes", len(existingNodes))

	resp, err := llmClient.Chat(ctx, llm.ChatRequest{
		Model: modelIDStr,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: compileSystemPrompt},
			{Role: llm.RoleUser, Content: prompt},
		},
	})
	if err != nil {
		kb.CompileStatus = CompileStatusError
		if updateErr := s.kbRepo.Update(kb); updateErr != nil {
			slog.Error("failed to update kb status", "kb_id", kb.ID, "error", updateErr)
		}
		s.logRepo.Create(&KnowledgeLog{
			KbID:         kb.ID,
			Action:       KnowledgeLogCompile,
			ModelID:      modelIDStr,
			ErrorMessage: err.Error(),
		})
		return fmt.Errorf("LLM call: %w", err)
	}

	// Parse LLM output
	output, err := s.parseLLMOutput(resp.Content)
	if err != nil {
		kb.CompileStatus = CompileStatusError
		if updateErr := s.kbRepo.Update(kb); updateErr != nil {
			slog.Error("failed to update kb status", "kb_id", kb.ID, "error", updateErr)
		}
		s.logRepo.Create(&KnowledgeLog{
			KbID:         kb.ID,
			Action:       KnowledgeLogCompile,
			ModelID:      modelIDStr,
			ErrorMessage: "parse LLM output: " + err.Error(),
		})
		return fmt.Errorf("parse output: %w", err)
	}

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
	stats, cascadeDetails, err := s.writeCompileOutput(kb.ID, output, sources, progress, onNodeProgress)
	if err != nil {
		kb.CompileStatus = CompileStatusError
		if updateErr := s.kbRepo.Update(kb); updateErr != nil {
			slog.Error("failed to update kb status", "kb_id", kb.ID, "error", updateErr)
		}
		return fmt.Errorf("write output: %w", err)
	}

	// Generate index node
	s.generateIndexNode(kb.ID)

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

	// Update embeddings progress (embedding service doesn't report progress yet, so mark all done)
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
	// Clear progress data on successful completion
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

// resolveLLMClient looks up the model and provider configured for the KB,
// then builds an llm.Client using the provider credentials.
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

func (s *KnowledgeCompileService) writeCompileOutput(kbID uint, output *compileOutput, sources []KnowledgeSource, progress *CompileProgress, onProgress func()) (*compileStats, *CascadeLog, error) {
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
		node := &KnowledgeNode{
			Title:      n.Title,
			Summary:    n.Summary,
			Content:    n.Content,
			NodeType:   NodeTypeConcept,
			SourceIDs:  resolveSourceIDsJSON(n.Sources, sourceMap),
			CompiledAt: now,
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
		node := &KnowledgeNode{
			Title:      n.Title,
			Summary:    n.Summary,
			Content:    n.Content,
			NodeType:   NodeTypeConcept,
			SourceIDs:  resolveSourceIDsJSON(n.Sources, sourceMap),
			CompiledAt: now,
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
		// Extract source IDs from the node's sources
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
		for _, rel := range n.Related {
			// Ensure target node exists (create empty concept if not)
			if _, err := s.graphRepo.FindNodeByTitle(kbID, rel.Concept); err != nil {
				emptyNode := &KnowledgeNode{
					Title:      rel.Concept,
					NodeType:   NodeTypeConcept,
					CompiledAt: now,
				}
				if err := s.graphRepo.UpsertNodeByTitle(kbID, emptyNode); err != nil {
					continue
				}
				stats.created++
			}

			if err := s.graphRepo.CreateEdge(kbID, n.Title, rel.Concept, rel.Relation, ""); err != nil {
				slog.Error("failed to create edge", "from", n.Title, "to", rel.Concept, "error", err)
				continue
			}
			stats.edges++
		}
	}

	return stats, cascadeLog, nil
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

func (s *KnowledgeCompileService) generateIndexNode(kbID uint) {
	nodes, err := s.graphRepo.FindAllNodes(kbID)
	if err != nil {
		return
	}

	var sb strings.Builder
	sb.WriteString("# Knowledge Index\n\n")
	sb.WriteString("| Concept | Summary |\n")
	sb.WriteString("|---------|--------|\n")
	conceptCount := 0
	for _, n := range nodes {
		if n.NodeType == NodeTypeIndex {
			continue
		}
		hasContent := "✓"
		if n.Content == nil || *n.Content == "" {
			hasContent = "—"
		}
		sb.WriteString(fmt.Sprintf("| %s %s | %s |\n", n.Title, hasContent, n.Summary))
		conceptCount++
	}

	indexContent := sb.String()
	summary := fmt.Sprintf("Index of %d concepts", conceptCount)

	node := &KnowledgeNode{
		Title:      "Knowledge Index",
		Summary:    summary,
		Content:    &indexContent,
		NodeType:   NodeTypeIndex,
		CompiledAt: time.Now().Unix(),
	}
	if err := s.graphRepo.UpsertNodeByTitle(kbID, node); err != nil {
		slog.Error("failed to upsert index node", "kb_id", kbID, "error", err)
	}
}

func (s *KnowledgeCompileService) runLint(kbID uint) int {
	issues := 0

	orphans, _ := s.graphRepo.CountOrphanNodes(kbID)
	issues += int(orphans)

	sparse, _ := s.graphRepo.CountSparseNodes(kbID)
	issues += int(sparse)

	contradictions, _ := s.graphRepo.CountContradictions(kbID)
	issues += int(contradictions)

	if issues > 0 {
		slog.Info("knowledge lint", "kb_id", kbID, "orphans", orphans, "sparse", sparse, "contradictions", contradictions)
	}
	return issues
}

func (s *KnowledgeCompileService) buildCompilePrompt(sources []KnowledgeSource, existingNodes []KnowledgeNode, analysis *cascadeAnalysis) string {
	var sb strings.Builder

	sb.WriteString("## Sources to compile\n\n")
	for i, src := range sources {
		sb.WriteString(fmt.Sprintf("### Source %d: %s\n\n", i+1, src.Title))
		content := src.Content
		if len(content) > 8000 {
			content = content[:8000] + "\n\n[...truncated...]"
		}
		sb.WriteString(content)
		sb.WriteString("\n\n---\n\n")
	}

	// Impact analysis section
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

		// High-impact nodes (likely to need updates)
		if len(analysis.HighImpactNodes) > 0 {
			sb.WriteString("### High-impact existing nodes (LIKELY NEED UPDATES)\n")
			sb.WriteString("These nodes are directly related to the new sources:\n\n")
			for _, n := range analysis.HighImpactNodes {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", n.Title, n.Summary))
			}
			sb.WriteString("\n")
		}

		// Medium-impact nodes
		if len(analysis.MediumImpactNodes) > 0 {
			sb.WriteString("### Medium-impact nodes (CHECK FOR NEW RELATIONSHIPS)\n")
			sb.WriteString("These nodes may be related to new concepts:\n\n")
			for _, n := range analysis.MediumImpactNodes {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", n.Title, n.Summary))
			}
			sb.WriteString("\n")
		}

		// Reference nodes
		if len(analysis.ReferenceNodes) > 0 {
			sb.WriteString("### Reference only (unlikely affected)\n")
			sb.WriteString("For context only - these are less likely to need updates:\n\n")
			// Limit to avoid overwhelming the prompt
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
		// Fallback: simple list without analysis
		sb.WriteString("## Existing knowledge nodes\n\n")
		for _, n := range existingNodes {
			if n.NodeType == NodeTypeIndex {
				continue
			}
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", n.Title, n.Summary))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (s *KnowledgeCompileService) parseLLMOutput(content string) (*compileOutput, error) {
	jsonStr := content

	if idx := strings.Index(content, "```json"); idx != -1 {
		start := idx + 7
		end := strings.Index(content[start:], "```")
		if end != -1 {
			jsonStr = content[start : start+end]
		}
	} else if idx := strings.Index(content, "```"); idx != -1 {
		start := idx + 3
		if nl := strings.Index(content[start:], "\n"); nl != -1 {
			start = start + nl + 1
		}
		end := strings.Index(content[start:], "```")
		if end != -1 {
			jsonStr = content[start : start+end]
		}
	}

	jsonStr = strings.TrimSpace(jsonStr)

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
		if node.NodeType == NodeTypeIndex {
			continue
		}

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
		if node.NodeType == NodeTypeIndex || highImpactIDs[node.ID] {
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
		if node.NodeType == NodeTypeIndex {
			continue
		}
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

const compileSystemPrompt = `You are a knowledge compiler. Your job is to read source documents and compile them into a knowledge graph of concept nodes with relationships.

IMPORTANT RULES:
1. Organize knowledge by CONCEPTS, not by source documents
2. Multiple sources about the same concept should be merged into one node
3. If sources contradict each other, note the contradiction and mark the relationship as "contradicts"
4. Create nodes even for concepts that don't have enough content for a full article (set content to null, provide only title and summary)
5. Use name-driven references — output concept names and source titles, NOT database IDs

CASCADE UPDATE RULES (CRITICAL):
When new sources relate to existing concepts, you MUST update the existing nodes:
1. Check "High-impact existing nodes" section - these LIKELY need updates
2. Include affected existing nodes in "updated_nodes" array, not "nodes" array
3. Add new relationships if the new source connects to existing concepts
4. Merge new source content into existing nodes when they cover the same concept
5. Flag contradictions if new sources conflict with existing knowledge
6. In "update_reason" field, explain specifically what changed and why

Do NOT put existing nodes (from "High-impact" or "Medium-impact" sections) in the "nodes" array.
Only truly NEW concepts should be in "nodes".

OUTPUT FORMAT: You MUST output valid JSON with this exact structure:
{
  "nodes": [
    {
      "title": "New Concept Name",
      "summary": "One-line description",
      "content": "Full Markdown or null",
      "related": [
        {"concept": "Other Concept Name", "relation": "related|contradicts|extends|part_of"}
      ],
      "sources": ["Source Title 1"]
    }
  ],
  "updated_nodes": [
    {
      "title": "Existing Concept Name",
      "summary": "Updated summary incorporating new information",
      "content": "Updated content with merged information",
      "related": [
        {"concept": "New Concept Name", "relation": "related"},
        {"concept": "Other Existing", "relation": "extends"}
      ],
      "sources": ["Original Source", "New Source Title"],
      "update_reason": "Added pricing information from new source; linked to new 'Claude Pricing' concept"
    }
  ]
}

Relation types:
- "related": general relationship
- "contradicts": conflicting information
- "extends": builds upon or specializes
- "part_of": is a component of

If no existing nodes need updates, leave "updated_nodes" as an empty array.
Output ONLY the JSON, no other text.`
