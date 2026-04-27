package runtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/FalkorDB/falkordb-go/v2"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
)

// KnowledgeGraphRepo handles all knowledge node and edge operations via FalkorDB.
// Each KnowledgeAsset (category=kg) maps to an independent FalkorDB graph named "kg_<id>".
type KnowledgeGraphRepo struct {
	client *FalkorDBClient
}

func NewKnowledgeGraphRepo(i do.Injector) (*KnowledgeGraphRepo, error) {
	client := do.MustInvoke[*FalkorDBClient](i)
	return &KnowledgeGraphRepo{client: client}, nil
}

// Available returns true if FalkorDB is configured.
func (r *KnowledgeGraphRepo) Available() bool {
	return r.client.Available()
}

// --- Graph lifecycle ---

// DeleteGraph removes the entire graph for a knowledge base.
func (r *KnowledgeGraphRepo) DeleteGraph(kbID uint) error {
	if err := r.client.DeleteGraph(kbID); err != nil {
		// Ignore "Graph does not exist" errors
		if strings.Contains(err.Error(), "Graph does not exist") {
			return nil
		}
		return err
	}
	return nil
}

// DeleteNodesBySourceID deletes all nodes whose source_ids JSON array contains the given source ID,
// along with their connected edges (FalkorDB cascades edge deletion on node removal).
func (r *KnowledgeGraphRepo) DeleteNodesBySourceID(kbID uint, sourceID uint) (int64, error) {
	g := r.client.GraphFor(kbID)

	// source_ids is a JSON string like "[1,5,12]". Match the integer at array boundaries
	// to avoid false positives (e.g. sourceID=1 matching "12").
	idStr := fmt.Sprintf("%d", sourceID)
	query := `MATCH (n:KnowledgeNode)
WHERE n.source_ids CONTAINS $p1
   OR n.source_ids CONTAINS $p2
   OR n.source_ids CONTAINS $p3
   OR n.source_ids CONTAINS $p4
WITH n, count(n) AS cnt
DELETE n
RETURN sum(cnt) AS deleted`
	params := map[string]interface{}{
		"p1": "[" + idStr + "]", // sole element: [1]
		"p2": "[" + idStr + ",", // first element: [1,
		"p3": "," + idStr + "]", // last element: ,1]
		"p4": "," + idStr + ",", // middle element: ,1,
	}
	result, err := g.Query(query, params, nil)
	if err != nil {
		if strings.Contains(err.Error(), "Graph does not exist") {
			return 0, nil
		}
		return 0, err
	}
	if result.Next() {
		if v, ok := result.Record().Get("deleted"); ok {
			return toInt64(v), nil
		}
	}
	return 0, nil
}

// --- Node operations ---

// CreateNode creates a new KnowledgeNode in FalkorDB.
func (r *KnowledgeGraphRepo) CreateNode(kbID uint, node *KnowledgeNode) error {
	if node.ID == "" {
		node.ID = uuid.New().String()
	}
	if node.CompiledAt == 0 {
		node.CompiledAt = time.Now().Unix()
	}

	g := r.client.GraphFor(kbID)
	query := `MERGE (n:KnowledgeNode {id: $id})
SET n.title = $title, n.summary = $summary, n.content = $content,
    n.node_type = $nodeType, n.source_ids = $sourceIds, n.compiled_at = $compiledAt,
    n.keywords = $keywords, n.citation_map = $citationMap`

	params := map[string]interface{}{
		"id":          node.ID,
		"title":       node.Title,
		"summary":     node.Summary,
		"content":     node.Content,
		"nodeType":    node.NodeType,
		"sourceIds":   node.SourceIDs,
		"keywords":    node.Keywords,
		"citationMap": node.CitationMap,
		"compiledAt":  node.CompiledAt,
	}

	_, err := g.Query(query, params, nil)
	return err
}

// UpsertNodeByTitle creates or updates a node by title (used during compilation).
func (r *KnowledgeGraphRepo) UpsertNodeByTitle(kbID uint, node *KnowledgeNode) error {
	if node.ID == "" {
		node.ID = uuid.New().String()
	}
	if node.CompiledAt == 0 {
		node.CompiledAt = time.Now().Unix()
	}

	g := r.client.GraphFor(kbID)
	query := `MERGE (n:KnowledgeNode {title: $title})
ON CREATE SET n.id = $id, n.summary = $summary, n.content = $content,
              n.node_type = $nodeType, n.source_ids = $sourceIds, n.compiled_at = $compiledAt,
              n.keywords = $keywords, n.citation_map = $citationMap
ON MATCH SET  n.summary = $summary, n.content = $content,
              n.source_ids = $sourceIds, n.compiled_at = $compiledAt,
              n.keywords = $keywords, n.citation_map = $citationMap
RETURN n.id`

	params := map[string]interface{}{
		"id":          node.ID,
		"title":       node.Title,
		"summary":     node.Summary,
		"content":     node.Content,
		"nodeType":    node.NodeType,
		"sourceIds":   node.SourceIDs,
		"keywords":    node.Keywords,
		"citationMap": node.CitationMap,
		"compiledAt":  node.CompiledAt,
	}

	result, err := g.Query(query, params, nil)
	if err != nil {
		return err
	}
	// Capture the actual ID (might be existing)
	if result.Next() {
		if rec := result.Record(); rec != nil {
			if v, ok := rec.Get("n.id"); ok {
				if s, ok := v.(string); ok {
					node.ID = s
				}
			}
		}
	}
	return nil
}

// FindNodeByID returns a single node by its UUID.
func (r *KnowledgeGraphRepo) FindNodeByID(kbID uint, nodeID string) (*KnowledgeNode, error) {
	g := r.client.GraphFor(kbID)
	result, err := g.Query(
		`MATCH (n:KnowledgeNode {id: $id}) RETURN n`,
		map[string]interface{}{"id": nodeID}, nil,
	)
	if err != nil {
		return nil, err
	}
	if !result.Next() {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}
	return recordToNode(result.Record(), "n")
}

// FindNodeByTitle returns a single node by title within a KB graph.
func (r *KnowledgeGraphRepo) FindNodeByTitle(kbID uint, title string) (*KnowledgeNode, error) {
	g := r.client.GraphFor(kbID)
	result, err := g.Query(
		`MATCH (n:KnowledgeNode {title: $title}) RETURN n`,
		map[string]interface{}{"title": title}, nil,
	)
	if err != nil {
		return nil, err
	}
	if !result.Next() {
		return nil, fmt.Errorf("node not found: %s", title)
	}
	return recordToNode(result.Record(), "n")
}

// FindAllNodes returns all nodes in the KB graph.
func (r *KnowledgeGraphRepo) FindAllNodes(kbID uint) ([]KnowledgeNode, error) {
	g := r.client.GraphFor(kbID)
	result, err := g.Query(
		`MATCH (n:KnowledgeNode) RETURN n ORDER BY n.node_type ASC, n.title ASC`,
		nil, nil,
	)
	if err != nil {
		return nil, err
	}
	return collectNodes(result, "n")
}

// ListNodes returns paginated nodes with optional keyword and type filter.
func (r *KnowledgeGraphRepo) ListNodes(kbID uint, keyword, nodeType string, page, pageSize int) ([]KnowledgeNode, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	g := r.client.GraphFor(kbID)

	// Build WHERE clause
	var conditions []string
	params := map[string]interface{}{}
	if nodeType != "" {
		conditions = append(conditions, "n.node_type = $nodeType")
		params["nodeType"] = nodeType
	}
	if keyword != "" {
		conditions = append(conditions, "(toLower(n.title) CONTAINS toLower($keyword) OR toLower(n.summary) CONTAINS toLower($keyword))")
		params["keyword"] = keyword
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count
	countQuery := fmt.Sprintf("MATCH (n:KnowledgeNode) %s RETURN count(n) AS total", where)
	countResult, err := g.Query(countQuery, params, nil)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if countResult.Next() {
		if v, ok := countResult.Record().Get("total"); ok {
			total = toInt64(v)
		}
	}

	// Fetch page
	offset := (page - 1) * pageSize
	params["skip"] = offset
	params["limit"] = pageSize
	dataQuery := fmt.Sprintf(
		"MATCH (n:KnowledgeNode) %s RETURN n ORDER BY n.node_type ASC, n.title ASC SKIP $skip LIMIT $limit",
		where,
	)
	dataResult, err := g.Query(dataQuery, params, nil)
	if err != nil {
		return nil, 0, err
	}
	nodes, err := collectNodes(dataResult, "n")
	return nodes, total, err
}

// CountNodes returns the total number of nodes.
func (r *KnowledgeGraphRepo) CountNodes(kbID uint) (int64, error) {
	g := r.client.GraphFor(kbID)
	result, err := g.Query(
		`MATCH (n:KnowledgeNode) RETURN count(n) AS cnt`,
		nil, nil,
	)
	if err != nil {
		// Graph might not exist yet
		if strings.Contains(err.Error(), "Graph does not exist") {
			return 0, nil
		}
		return 0, err
	}
	if result.Next() {
		if v, ok := result.Record().Get("cnt"); ok {
			return toInt64(v), nil
		}
	}
	return 0, nil
}

// CountEdges returns the number of edges.
func (r *KnowledgeGraphRepo) CountEdges(kbID uint) (int64, error) {
	g := r.client.GraphFor(kbID)
	result, err := g.Query(`MATCH ()-[r]->() RETURN count(r) AS cnt`, nil, nil)
	if err != nil {
		if strings.Contains(err.Error(), "Graph does not exist") {
			return 0, nil
		}
		return 0, err
	}
	if result.Next() {
		if v, ok := result.Record().Get("cnt"); ok {
			return toInt64(v), nil
		}
	}
	return 0, nil
}

// CountEdgesForNode returns the number of edges connected to a node.
func (r *KnowledgeGraphRepo) CountEdgesForNode(kbID uint, nodeID string) (int, error) {
	g := r.client.GraphFor(kbID)
	result, err := g.Query(
		`MATCH (n:KnowledgeNode {id: $id})-[r]-() RETURN count(r) AS cnt`,
		map[string]interface{}{"id": nodeID}, nil,
	)
	if err != nil {
		return 0, err
	}
	if result.Next() {
		if v, ok := result.Record().Get("cnt"); ok {
			return int(toInt64(v)), nil
		}
	}
	return 0, nil
}

// --- Edge operations ---

// CreateEdge creates a relationship between two nodes by title.
func (r *KnowledgeGraphRepo) CreateEdge(kbID uint, fromTitle, toTitle, relation, description string) error {
	if relation == "" {
		relation = EdgeRelationRelated
	}

	g := r.client.GraphFor(kbID)
	// Use dynamic relationship type via APOC-style — FalkorDB doesn't support variable rel types in MERGE,
	// so we use separate queries per relation type.
	query := fmt.Sprintf(
		`MATCH (a:KnowledgeNode {title: $from}), (b:KnowledgeNode {title: $to})
MERGE (a)-[r:%s]->(b)
SET r.description = $desc`, cypherRelType(relation))

	params := map[string]interface{}{
		"from": fromTitle,
		"to":   toTitle,
		"desc": description,
	}
	_, err := g.Query(query, params, nil)
	return err
}

// --- Graph traversal ---

// GetSubgraph returns nodes and edges within N hops of the given node.
func (r *KnowledgeGraphRepo) GetSubgraph(kbID uint, nodeID string, depth int) ([]KnowledgeNode, []KnowledgeEdge, error) {
	if depth < 1 {
		depth = 1
	}
	if depth > 3 {
		depth = 3
	}

	g := r.client.GraphFor(kbID)
	query := fmt.Sprintf(
		`MATCH (start:KnowledgeNode {id: $id})
CALL {
  WITH start
  MATCH path = (start)-[*1..%d]-(neighbor)
  RETURN neighbor, relationships(path) AS rels
}
RETURN DISTINCT neighbor, rels`, depth)

	result, err := g.Query(query, map[string]interface{}{"id": nodeID}, nil)
	if err != nil {
		// Fallback to simpler query if CALL subquery not supported
		return r.getSubgraphSimple(g, nodeID, depth)
	}

	nodeMap := make(map[string]*KnowledgeNode)
	edgeSet := make(map[string]*KnowledgeEdge)

	// Add the start node
	startNode, _ := r.FindNodeByID(kbID, nodeID)
	if startNode != nil {
		nodeMap[startNode.ID] = startNode
	}

	for result.Next() {
		rec := result.Record()
		if n, err := recordToNode(rec, "neighbor"); err == nil {
			nodeMap[n.ID] = n
		}
	}

	// Get all edges between collected nodes
	if len(nodeMap) > 0 {
		nodeIDs := make([]interface{}, 0, len(nodeMap))
		for id := range nodeMap {
			nodeIDs = append(nodeIDs, id)
		}
		edges, _ := r.findEdgesBetweenNodes(g, nodeIDs)
		for i := range edges {
			key := edges[i].FromNodeID + "->" + edges[i].ToNodeID + ":" + edges[i].Relation
			edgeSet[key] = &edges[i]
		}
	}

	nodes := make([]KnowledgeNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, *n)
	}
	edges := make([]KnowledgeEdge, 0, len(edgeSet))
	for _, e := range edgeSet {
		edges = append(edges, *e)
	}
	return nodes, edges, nil
}

func (r *KnowledgeGraphRepo) getSubgraphSimple(g *falkordb.Graph, nodeID string, depth int) ([]KnowledgeNode, []KnowledgeEdge, error) {
	// Simple approach: match variable-length paths
	query := fmt.Sprintf(
		`MATCH (start:KnowledgeNode {id: $id})-[*1..%d]-(neighbor:KnowledgeNode)
RETURN DISTINCT neighbor`, depth)

	result, err := g.Query(query, map[string]interface{}{"id": nodeID}, nil)
	if err != nil {
		return nil, nil, err
	}

	nodeMap := make(map[string]*KnowledgeNode)
	for result.Next() {
		if n, err := recordToNode(result.Record(), "neighbor"); err == nil {
			nodeMap[n.ID] = n
		}
	}

	// Add start node to the set
	startResult, _ := g.Query(
		`MATCH (n:KnowledgeNode {id: $id}) RETURN n`,
		map[string]interface{}{"id": nodeID}, nil,
	)
	if startResult != nil && startResult.Next() {
		if n, err := recordToNode(startResult.Record(), "n"); err == nil {
			nodeMap[n.ID] = n
		}
	}

	// Collect edges between all found nodes
	nodeIDs := make([]interface{}, 0, len(nodeMap))
	for id := range nodeMap {
		nodeIDs = append(nodeIDs, id)
	}
	edges, _ := r.findEdgesBetweenNodes(g, nodeIDs)

	nodes := make([]KnowledgeNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, *n)
	}
	return nodes, edges, nil
}

func (r *KnowledgeGraphRepo) findEdgesBetweenNodes(g *falkordb.Graph, nodeIDs []interface{}) ([]KnowledgeEdge, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	result, err := g.Query(
		`MATCH (a:KnowledgeNode)-[r]->(b:KnowledgeNode)
WHERE a.id IN $ids AND b.id IN $ids
RETURN a.id AS from_id, b.id AS to_id, type(r) AS rel, r.description AS desc`,
		map[string]interface{}{"ids": nodeIDs}, nil,
	)
	if err != nil {
		return nil, err
	}

	var edges []KnowledgeEdge
	for result.Next() {
		rec := result.Record()
		fromID, _ := rec.Get("from_id")
		toID, _ := rec.Get("to_id")
		rel, _ := rec.Get("rel")
		desc, _ := rec.Get("desc")
		edges = append(edges, KnowledgeEdge{
			FromNodeID:  toString(fromID),
			ToNodeID:    toString(toID),
			Relation:    relationFromCypher(toString(rel)),
			Description: toString(desc),
		})
	}
	return edges, nil
}

// GetFullGraph returns all nodes and edges for a KB.
func (r *KnowledgeGraphRepo) GetFullGraph(kbID uint) ([]KnowledgeNode, []KnowledgeEdge, error) {
	g := r.client.GraphFor(kbID)

	// Get all nodes
	nodeResult, err := g.Query(
		`MATCH (n:KnowledgeNode) RETURN n ORDER BY n.node_type ASC, n.title ASC`,
		nil, nil,
	)
	if err != nil {
		if strings.Contains(err.Error(), "Graph does not exist") {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	nodes, err := collectNodes(nodeResult, "n")
	if err != nil {
		return nil, nil, err
	}

	// Get all edges
	edgeResult, err := g.Query(
		`MATCH (a:KnowledgeNode)-[r]->(b:KnowledgeNode)
RETURN a.id AS from_id, b.id AS to_id, type(r) AS rel, r.description AS desc`,
		nil, nil,
	)
	if err != nil {
		return nodes, nil, nil
	}

	var edges []KnowledgeEdge
	for edgeResult.Next() {
		rec := edgeResult.Record()
		fromID, _ := rec.Get("from_id")
		toID, _ := rec.Get("to_id")
		rel, _ := rec.Get("rel")
		desc, _ := rec.Get("desc")
		edges = append(edges, KnowledgeEdge{
			FromNodeID:  toString(fromID),
			ToNodeID:    toString(toID),
			Relation:    relationFromCypher(toString(rel)),
			Description: toString(desc),
		})
	}

	return nodes, edges, nil
}

// --- Vector search ---

// VectorSearch returns the top-K nodes by cosine similarity.
func (r *KnowledgeGraphRepo) VectorSearch(kbID uint, queryVec []float32, topK int) ([]KnowledgeNode, []float64, error) {
	g := r.client.GraphFor(kbID)
	vec := make([]interface{}, len(queryVec))
	for i, v := range queryVec {
		vec[i] = float64(v)
	}
	result, err := g.Query(
		`CALL db.idx.vector.queryNodes('KnowledgeNode', 'embedding', $topK, vecf32($vec))
YIELD node, score
RETURN node, score`,
		map[string]interface{}{"topK": topK, "vec": vec}, nil,
	)
	if err != nil {
		return nil, nil, err
	}

	var nodes []KnowledgeNode
	var scores []float64
	for result.Next() {
		rec := result.Record()
		n, err := recordToNode(rec, "node")
		if err != nil {
			continue
		}
		nodes = append(nodes, *n)
		if v, ok := rec.Get("score"); ok {
			scores = append(scores, toFloat64(v))
		}
	}
	return nodes, scores, nil
}

// VectorSearchWithExpand performs vector search then expands via graph traversal.
func (r *KnowledgeGraphRepo) VectorSearchWithExpand(kbID uint, queryVec []float32, topK, hops int) ([]KnowledgeNode, []KnowledgeEdge, []float64, error) {
	seeds, scores, err := r.VectorSearch(kbID, queryVec, topK)
	if err != nil {
		return nil, nil, nil, err
	}

	if hops < 1 || len(seeds) == 0 {
		return seeds, nil, scores, nil
	}

	allNodes, edges := r.expandSeeds(kbID, seeds, hops)
	return allNodes, edges, scores, nil
}

// expandSeeds performs N-hop graph traversal from seed nodes and collects all edges between them.
func (r *KnowledgeGraphRepo) expandSeeds(kbID uint, seeds []KnowledgeNode, hops int) ([]KnowledgeNode, []KnowledgeEdge) {
	nodeMap := make(map[string]*KnowledgeNode)
	for i := range seeds {
		nodeMap[seeds[i].ID] = &seeds[i]
	}

	g := r.client.GraphFor(kbID)
	for _, seed := range seeds {
		query := fmt.Sprintf(
			`MATCH (start:KnowledgeNode {id: $id})-[*1..%d]-(neighbor:KnowledgeNode)
RETURN DISTINCT neighbor`, hops)
		result, err := g.Query(query, map[string]interface{}{"id": seed.ID}, nil)
		if err != nil {
			continue
		}
		for result.Next() {
			if n, err := recordToNode(result.Record(), "neighbor"); err == nil {
				if _, exists := nodeMap[n.ID]; !exists {
					nodeMap[n.ID] = n
				}
			}
		}
	}

	nodeIDs := make([]interface{}, 0, len(nodeMap))
	for id := range nodeMap {
		nodeIDs = append(nodeIDs, id)
	}
	edges, _ := r.findEdgesBetweenNodes(g, nodeIDs)

	allNodes := make([]KnowledgeNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		allNodes = append(allNodes, *n)
	}
	return allNodes, edges
}

// HybridSearchResult holds the merged results from hybrid search.
type HybridSearchResult struct {
	Nodes  []KnowledgeNode
	Edges  []KnowledgeEdge
	Scores map[string]float64 // nodeID -> RRF score (seed nodes only)
}

// HybridSearch runs vector + fulltext concurrently, merges via RRF, then expands via graph.
// queryVec may be nil if embedding is not configured (degrades to fulltext-only).
func (r *KnowledgeGraphRepo) HybridSearch(kbID uint, queryVec []float32, queryText string, topK, hops int) (*HybridSearchResult, error) {
	const rrfK = 60
	fetchK := topK * 3

	type rankedResult struct {
		nodes  []KnowledgeNode
		scores []float64
		err    error
	}

	var vecRes, ftRes rankedResult
	var wg sync.WaitGroup

	// Concurrent vector search
	if queryVec != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nodes, scores, err := r.VectorSearch(kbID, queryVec, fetchK)
			vecRes = rankedResult{nodes, scores, err}
		}()
	}

	// Concurrent fulltext search
	wg.Add(1)
	go func() {
		defer wg.Done()
		nodes, scores, err := r.SearchFullText(kbID, queryText, fetchK)
		ftRes = rankedResult{nodes, scores, err}
	}()

	wg.Wait()

	// If both failed, return error
	if (queryVec == nil || vecRes.err != nil) && ftRes.err != nil {
		errMsg := "fulltext search failed"
		if vecRes.err != nil {
			errMsg = fmt.Sprintf("vector: %v, fulltext: %v", vecRes.err, ftRes.err)
		}
		return nil, fmt.Errorf("hybrid search: %s", errMsg)
	}

	// RRF merge
	rrfScores := make(map[string]float64)
	nodeByID := make(map[string]*KnowledgeNode)

	if vecRes.err == nil {
		for i, n := range vecRes.nodes {
			rrfScores[n.ID] += 1.0 / float64(rrfK+i+1)
			if _, exists := nodeByID[n.ID]; !exists {
				cp := n
				nodeByID[n.ID] = &cp
			}
		}
	}

	if ftRes.err == nil {
		for i, n := range ftRes.nodes {
			rrfScores[n.ID] += 1.0 / float64(rrfK+i+1)
			if _, exists := nodeByID[n.ID]; !exists {
				cp := n
				nodeByID[n.ID] = &cp
			}
		}
	}

	// Sort by RRF score descending, take topK
	type scored struct {
		id    string
		score float64
	}
	ranked := make([]scored, 0, len(rrfScores))
	for id, s := range rrfScores {
		ranked = append(ranked, scored{id, s})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})
	if len(ranked) > topK {
		ranked = ranked[:topK]
	}

	seeds := make([]KnowledgeNode, 0, len(ranked))
	seedScores := make(map[string]float64, len(ranked))
	for _, r := range ranked {
		if n, ok := nodeByID[r.id]; ok {
			seeds = append(seeds, *n)
			seedScores[r.id] = r.score
		}
	}

	if hops < 1 || len(seeds) == 0 {
		return &HybridSearchResult{Nodes: seeds, Scores: seedScores}, nil
	}

	allNodes, edges := r.expandSeeds(kbID, seeds, hops)
	return &HybridSearchResult{Nodes: allNodes, Edges: edges, Scores: seedScores}, nil
}

// --- Full-text search ---

// SearchFullText searches nodes via FalkorDB full-text index.
func (r *KnowledgeGraphRepo) SearchFullText(kbID uint, keyword string, limit int) ([]KnowledgeNode, []float64, error) {
	if limit < 1 {
		limit = 20
	}
	g := r.client.GraphFor(kbID)
	result, err := g.Query(
		`CALL db.idx.fulltext.queryNodes('KnowledgeNode', $keyword)
YIELD node, score
RETURN node, score LIMIT $limit`,
		map[string]interface{}{"keyword": keyword, "limit": limit}, nil,
	)
	if err != nil {
		// Fallback to CONTAINS if full-text index doesn't exist
		return r.searchContains(kbID, keyword, limit)
	}
	var nodes []KnowledgeNode
	var scores []float64
	for result.Next() {
		rec := result.Record()
		n, err := recordToNode(rec, "node")
		if err != nil {
			continue
		}
		nodes = append(nodes, *n)
		if v, ok := rec.Get("score"); ok {
			scores = append(scores, toFloat64(v))
		}
	}
	return nodes, scores, nil
}

// searchContains is a fallback when full-text index is not available.
func (r *KnowledgeGraphRepo) searchContains(kbID uint, keyword string, limit int) ([]KnowledgeNode, []float64, error) {
	g := r.client.GraphFor(kbID)
	result, err := g.Query(
		`MATCH (n:KnowledgeNode)
WHERE toLower(n.title) CONTAINS toLower($keyword) OR toLower(n.summary) CONTAINS toLower($keyword)
RETURN n LIMIT $limit`,
		map[string]interface{}{"keyword": keyword, "limit": limit}, nil,
	)
	if err != nil {
		return nil, nil, err
	}
	nodes, err := collectNodes(result, "n")
	if err != nil {
		return nil, nil, err
	}
	scores := make([]float64, len(nodes))
	for i := range scores {
		scores[i] = 1.0 - float64(i)*0.01
	}
	return nodes, scores, nil
}

// --- Index management ---

// CreateFullTextIndex creates a full-text index on title and summary.
func (r *KnowledgeGraphRepo) CreateFullTextIndex(kbID uint) error {
	g := r.client.GraphFor(kbID)
	_, err := g.Query(
		`CREATE FULLTEXT INDEX FOR (n:KnowledgeNode) ON (n.title, n.summary)`,
		nil, nil,
	)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
}

// DropFullTextIndex drops the full-text index.
func (r *KnowledgeGraphRepo) DropFullTextIndex(kbID uint) error {
	g := r.client.GraphFor(kbID)
	_, err := g.Query(`DROP INDEX FOR (n:KnowledgeNode) ON (n.title, n.summary)`, nil, nil)
	if err != nil && !strings.Contains(err.Error(), "does not exist") && !strings.Contains(err.Error(), "Unable to drop") {
		return err
	}
	return nil
}

// CreateVectorIndex creates an HNSW vector index on the embedding property.
func (r *KnowledgeGraphRepo) CreateVectorIndex(kbID uint, dimension int) error {
	g := r.client.GraphFor(kbID)
	query := fmt.Sprintf(
		`CREATE VECTOR INDEX FOR (n:KnowledgeNode) ON (n.embedding) OPTIONS {dimension: %d, similarityFunction: 'cosine'}`,
		dimension,
	)
	_, err := g.Query(query, nil, nil)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
}

// DropVectorIndex drops the HNSW vector index.
func (r *KnowledgeGraphRepo) DropVectorIndex(kbID uint) error {
	g := r.client.GraphFor(kbID)
	_, err := g.Query(`DROP INDEX FOR (n:KnowledgeNode) ON (n.embedding)`, nil, nil)
	if err != nil && !strings.Contains(err.Error(), "does not exist") && !strings.Contains(err.Error(), "Unable to drop") {
		return err
	}
	return nil
}

// SetNodeEmbedding writes the embedding vector to a node.
func (r *KnowledgeGraphRepo) SetNodeEmbedding(kbID uint, nodeID string, embedding []float32) error {
	g := r.client.GraphFor(kbID)
	// Convert []float32 to []interface{} with float64 values, because
	// the FalkorDB Go client only supports []interface{} in params.
	vec := make([]interface{}, len(embedding))
	for i, v := range embedding {
		vec[i] = float64(v)
	}
	_, err := g.Query(
		`MATCH (n:KnowledgeNode {id: $id}) SET n.embedding = vecf32($vec)`,
		map[string]interface{}{"id": nodeID, "vec": vec}, nil,
	)
	return err
}

// --- Lint queries ---

// CountOrphanNodes returns concept nodes with no edges.
func (r *KnowledgeGraphRepo) CountOrphanNodes(kbID uint) (int64, error) {
	g := r.client.GraphFor(kbID)
	result, err := g.Query(
		`MATCH (n:KnowledgeNode {node_type: $nt})
WHERE NOT (n)-[]-()
RETURN count(n) AS cnt`,
		map[string]interface{}{"nt": NodeTypeConcept}, nil,
	)
	if err != nil {
		return 0, err
	}
	if result.Next() {
		if v, ok := result.Record().Get("cnt"); ok {
			return toInt64(v), nil
		}
	}
	return 0, nil
}

// CountSparseNodes returns concept nodes without content but with 3+ edges.
func (r *KnowledgeGraphRepo) CountSparseNodes(kbID uint) (int64, error) {
	g := r.client.GraphFor(kbID)
	result, err := g.Query(
		`MATCH (n:KnowledgeNode {node_type: $nt})
WHERE (n.content IS NULL OR n.content = '')
WITH n, size([(n)-[]-() | 1]) AS deg
WHERE deg >= 3
RETURN count(n) AS cnt`,
		map[string]interface{}{"nt": NodeTypeConcept}, nil,
	)
	if err != nil {
		return 0, err
	}
	if result.Next() {
		if v, ok := result.Record().Get("cnt"); ok {
			return toInt64(v), nil
		}
	}
	return 0, nil
}

// CountContradictions returns edges with CONTRADICTS relation.
func (r *KnowledgeGraphRepo) CountContradictions(kbID uint) (int64, error) {
	g := r.client.GraphFor(kbID)
	result, err := g.Query(`MATCH ()-[r:CONTRADICTS]->() RETURN count(r) AS cnt`, nil, nil)
	if err != nil {
		return 0, err
	}
	if result.Next() {
		if v, ok := result.Record().Get("cnt"); ok {
			return toInt64(v), nil
		}
	}
	return 0, nil
}

// --- Helpers ---

// recordToNode extracts a KnowledgeNode from a FalkorDB record.
func recordToNode(rec *falkordb.Record, alias string) (*KnowledgeNode, error) {
	v, ok := rec.Get(alias)
	if !ok {
		return nil, fmt.Errorf("alias %q not found in record", alias)
	}
	fNode, ok := v.(*falkordb.Node)
	if !ok {
		return nil, fmt.Errorf("expected *falkordb.Node, got %T", v)
	}

	node := &KnowledgeNode{
		ID:          toString(fNode.GetProperty("id")),
		Title:       toString(fNode.GetProperty("title")),
		Summary:     toString(fNode.GetProperty("summary")),
		Content:     toString(fNode.GetProperty("content")),
		NodeType:    toString(fNode.GetProperty("node_type")),
		SourceIDs:   toString(fNode.GetProperty("source_ids")),
		Keywords:    toString(fNode.GetProperty("keywords")),
		CitationMap: toString(fNode.GetProperty("citation_map")),
		CompiledAt:  toInt64(fNode.GetProperty("compiled_at")),
	}

	return node, nil
}

func collectNodes(result *falkordb.QueryResult, alias string) ([]KnowledgeNode, error) {
	var nodes []KnowledgeNode
	for result.Next() {
		n, err := recordToNode(result.Record(), alias)
		if err != nil {
			slog.Debug("skip node parse", "error", err)
			continue
		}
		nodes = append(nodes, *n)
	}
	return nodes, nil
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case float32:
		return float64(n)
	default:
		return 0
	}
}

// cypherRelType converts an edge relation string to a Cypher relationship type.
func cypherRelType(relation string) string {
	switch relation {
	case EdgeRelationContradicts:
		return "CONTRADICTS"
	default:
		return "RELATED_TO"
	}
}

// relationFromCypher converts a Cypher relationship type back to our relation string.
// Legacy types (EXTENDS, PART_OF) are mapped to "related".
func relationFromCypher(rel string) string {
	switch rel {
	case "CONTRADICTS":
		return EdgeRelationContradicts
	default:
		return EdgeRelationRelated
	}
}

// resolveSourceIDsJSON converts source title list to JSON array of source IDs.
func resolveSourceIDsJSON(titles []string, sourceMap map[string]uint) string {
	var ids []uint
	for _, title := range titles {
		if id, ok := sourceMap[title]; ok {
			ids = append(ids, id)
		}
	}
	b, _ := json.Marshal(ids)
	return string(b)
}
