package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/samber/do/v2"

	"metis/internal/scheduler"
)

// ConceptMapEngine implements KnowledgeEngine for the "concept_map" graph type.
// It wraps the existing Map-Reduce compile pipeline and FalkorDB search.
type ConceptMapEngine struct {
	graphRepo    *KnowledgeGraphRepo
	assetRepo    *KnowledgeAssetRepo
	sourceRepo   *KnowledgeSourceRepo
	logRepo      *KnowledgeLogRepo
	modelRepo    *ModelRepo
	embeddingSvc *KnowledgeEmbeddingService
	compileSvc   *KnowledgeCompileService
	engine       *scheduler.Engine
}

func NewConceptMapEngine(i do.Injector) (*ConceptMapEngine, error) {
	e := &ConceptMapEngine{
		graphRepo:    do.MustInvoke[*KnowledgeGraphRepo](i),
		assetRepo:    do.MustInvoke[*KnowledgeAssetRepo](i),
		sourceRepo:   do.MustInvoke[*KnowledgeSourceRepo](i),
		logRepo:      do.MustInvoke[*KnowledgeLogRepo](i),
		modelRepo:    do.MustInvoke[*ModelRepo](i),
		embeddingSvc: do.MustInvoke[*KnowledgeEmbeddingService](i),
		compileSvc:   do.MustInvoke[*KnowledgeCompileService](i),
		engine:       do.MustInvoke[*scheduler.Engine](i),
	}
	// Register this engine for kg:concept_map
	RegisterEngine(AssetCategoryKG, KGTypeConceptMap, e)
	return e, nil
}

// Build enqueues an async compile task for the asset.
func (e *ConceptMapEngine) Build(ctx context.Context, asset *KnowledgeAsset, sources []*KnowledgeSource) error {
	return e.enqueueCompile(asset.ID, false)
}

// Rebuild enqueues an async full-recompile task for the asset.
func (e *ConceptMapEngine) Rebuild(ctx context.Context, asset *KnowledgeAsset, sources []*KnowledgeSource) error {
	return e.enqueueCompile(asset.ID, true)
}

// Search performs hybrid search (vector + fulltext + graph expansion) and returns unified results.
func (e *ConceptMapEngine) Search(ctx context.Context, asset *KnowledgeAsset, query *RecallQuery) (*RecallResult, error) {
	if !e.graphRepo.Available() {
		return nil, fmt.Errorf("FalkorDB not available")
	}

	topK := query.TopK
	if topK <= 0 {
		topK = 5
	}

	// Generate query embedding if embedding is configured
	var queryVec []float32
	if asset.EmbeddingProviderID != nil && asset.EmbeddingModelID != "" {
		vec, err := e.embeddingSvc.EmbedQuery(ctx, asset, query.Query)
		if err != nil {
			slog.Warn("failed to generate query embedding, falling back to fulltext", "asset_id", asset.ID, "error", err)
		} else {
			queryVec = vec
		}
	}

	// Use the graph repo's HybridSearch (vector + fulltext + graph expand)
	result, err := e.graphRepo.HybridSearch(asset.ID, queryVec, query.Query, topK, 2)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}

	// Convert to unified RecallResult
	recall := &RecallResult{
		Debug: &RecallDebug{
			Mode:      "hybrid",
			TotalHits: len(result.Nodes),
		},
	}

	for _, node := range result.Nodes {
		unit := KnowledgeUnit{
			ID:       node.ID,
			AssetID:  asset.ID,
			UnitType: "entity",
			Title:    node.Title,
			Content:  node.Content,
			Summary:  node.Summary,
		}
		if score, ok := result.Scores[node.ID]; ok {
			unit.Score = score
		}
		// Parse source IDs from JSON
		if node.SourceIDs != "" {
			var refs []uint
			if err := json.Unmarshal([]byte(node.SourceIDs), &refs); err == nil {
				unit.SourceRefs = refs
			}
		}
		recall.Items = append(recall.Items, unit)
	}

	for _, edge := range result.Edges {
		recall.Relations = append(recall.Relations, KnowledgeRelation{
			FromUnitID:  edge.FromNodeID,
			ToUnitID:    edge.ToNodeID,
			Predicate:   edge.Relation,
			Description: edge.Description,
		})
	}

	return recall, nil
}

// ContentStats returns node and edge counts for the graph.
func (e *ConceptMapEngine) ContentStats(ctx context.Context, asset *KnowledgeAsset) (*ContentStats, error) {
	if !e.graphRepo.Available() {
		return &ContentStats{}, nil
	}
	nodeCount, _ := e.graphRepo.CountNodes(asset.ID)
	edgeCount, _ := e.graphRepo.CountEdges(asset.ID)
	return &ContentStats{
		NodeCount: int(nodeCount),
		EdgeCount: int(edgeCount),
	}, nil
}

// --- Graph-specific methods (not part of KnowledgeEngine interface) ---

// GetFullGraph returns all nodes and edges for visualization.
func (e *ConceptMapEngine) GetFullGraph(assetID uint) ([]KnowledgeNode, []KnowledgeEdge, error) {
	return e.graphRepo.GetFullGraph(assetID)
}

// ListNodes returns paginated nodes.
func (e *ConceptMapEngine) ListNodes(assetID uint, keyword, nodeType string, page, pageSize int) ([]KnowledgeNode, int64, error) {
	return e.graphRepo.ListNodes(assetID, keyword, nodeType, page, pageSize)
}

// GetNode returns a single node by ID.
func (e *ConceptMapEngine) GetNode(assetID uint, nodeID string) (*KnowledgeNode, error) {
	return e.graphRepo.FindNodeByID(assetID, nodeID)
}

// GetNodeSubgraph returns a node and its local neighborhood.
func (e *ConceptMapEngine) GetNodeSubgraph(assetID uint, nodeID string, depth int) ([]KnowledgeNode, []KnowledgeEdge, error) {
	return e.graphRepo.GetSubgraph(assetID, nodeID, depth)
}

// DeleteGraph removes the entire FalkorDB graph for an asset.
func (e *ConceptMapEngine) DeleteGraph(assetID uint) error {
	return e.graphRepo.DeleteGraph(assetID)
}

// --- Internal helpers ---

func (e *ConceptMapEngine) enqueueCompile(assetID uint, recompile bool) error {
	payload := compilePayload{KbID: assetID, Recompile: recompile}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return e.engine.Enqueue("ai-knowledge-compile", json.RawMessage(b))
}
