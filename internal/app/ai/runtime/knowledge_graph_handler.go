package runtime

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

// KnowledgeGraphHandler exposes REST endpoints for knowledge graph assets.
type KnowledgeGraphHandler struct {
	assetSvc *KnowledgeAssetService
	engine   *ConceptMapEngine
}

func NewKnowledgeGraphHandler(i do.Injector) (*KnowledgeGraphHandler, error) {
	return &KnowledgeGraphHandler{
		assetSvc: do.MustInvoke[*KnowledgeAssetService](i),
		engine:   do.MustInvoke[*ConceptMapEngine](i),
	}, nil
}

// graphAssetID parses :id and validates it is a KG asset.
func (h *KnowledgeGraphHandler) graphAssetID(c *gin.Context) (*KnowledgeAsset, bool) {
	id, ok := handler.ParseUintParam(c, "id")
	if !ok {
		return nil, false
	}
	asset, err := h.assetSvc.GetByCategory(id, AssetCategoryKG)
	if err != nil {
		handler.Fail(c, http.StatusNotFound, err.Error())
		return nil, false
	}
	return asset, true
}

// --- CRUD ---

type createGraphReq struct {
	Name                string       `json:"name" binding:"required"`
	Description         string       `json:"description"`
	Type                string       `json:"type" binding:"required"`
	Config              *GraphConfig `json:"config"`
	CompileModelID      *uint        `json:"compileModelId"`
	EmbeddingProviderID *uint        `json:"embeddingProviderId"`
	EmbeddingModelID    string       `json:"embeddingModelId"`
	AutoBuild           bool         `json:"autoBuild"`
}

func (h *KnowledgeGraphHandler) Create(c *gin.Context) {
	var req createGraphReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	// Validate type
	meta := GetAssetType(AssetCategoryKG, req.Type)
	if meta == nil {
		handler.Fail(c, http.StatusBadRequest, "unsupported graph type: "+req.Type)
		return
	}

	asset := &KnowledgeAsset{
		Name:                req.Name,
		Description:         req.Description,
		Category:            AssetCategoryKG,
		Type:                req.Type,
		CompileModelID:      req.CompileModelID,
		EmbeddingProviderID: req.EmbeddingProviderID,
		EmbeddingModelID:    req.EmbeddingModelID,
		AutoBuild:           req.AutoBuild,
	}

	cfg := req.Config
	if cfg == nil {
		dflt := DefaultGraphConfig()
		cfg = &dflt
	}
	if err := asset.SetConfig(cfg); err != nil {
		handler.Fail(c, http.StatusInternalServerError, "failed to set config: "+err.Error())
		return
	}

	if err := h.assetSvc.Create(asset); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_graph.create")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))
	c.Set("audit_summary", "Created knowledge graph: "+asset.Name)

	handler.OK(c, asset.ToResponse())
}

func (h *KnowledgeGraphHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := AssetListParams{
		Category: AssetCategoryKG,
		Type:     c.Query("type"),
		Status:   c.Query("status"),
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	}

	assets, total, err := h.assetSvc.List(params)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]KnowledgeAssetResponse, 0, len(assets))
	for i := range assets {
		resp := assets[i].ToResponse()
		stats, _ := h.engine.ContentStats(c, &assets[i])
		if stats != nil {
			resp.NodeCount = stats.NodeCount
			resp.EdgeCount = stats.EdgeCount
		}
		items = append(items, resp)
	}

	handler.OK(c, gin.H{
		"items": items,
		"total": total,
		"page":  page,
	})
}

func (h *KnowledgeGraphHandler) Get(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	resp := asset.ToResponse()
	stats, _ := h.engine.ContentStats(c, asset)
	if stats != nil {
		resp.NodeCount = stats.NodeCount
		resp.EdgeCount = stats.EdgeCount
	}

	handler.OK(c, resp)
}

type updateGraphReq struct {
	Name                string       `json:"name" binding:"required"`
	Description         string       `json:"description"`
	Config              *GraphConfig `json:"config"`
	CompileModelID      *uint        `json:"compileModelId"`
	EmbeddingProviderID *uint        `json:"embeddingProviderId"`
	EmbeddingModelID    string       `json:"embeddingModelId"`
	AutoBuild           bool         `json:"autoBuild"`
}

func (h *KnowledgeGraphHandler) Update(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	var req updateGraphReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.assetSvc.Update(
		asset.ID, req.Name, req.Description, req.Config,
		req.CompileModelID, req.EmbeddingProviderID, req.EmbeddingModelID,
		req.AutoBuild,
	)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_graph.update")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))
	c.Set("audit_summary", "Updated knowledge graph: "+updated.Name)

	handler.OK(c, updated.ToResponse())
}

func (h *KnowledgeGraphHandler) Delete(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	// Delete the FalkorDB graph data
	_ = h.engine.DeleteGraph(asset.ID)

	// Delete the asset record and associations
	if err := h.assetSvc.Delete(asset.ID); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_graph.delete")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))

	handler.OK(c, nil)
}

// --- Source association ---

type addSourcesReq struct {
	SourceIDs []uint `json:"sourceIds" binding:"required,min=1"`
}

func (h *KnowledgeGraphHandler) AddSources(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	var req addSourcesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.assetSvc.AddSources(asset.ID, req.SourceIDs); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_graph.add_sources")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))

	handler.OK(c, nil)
}

func (h *KnowledgeGraphHandler) RemoveSource(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	sourceID, ok := handler.ParseUintParam(c, "sourceId")
	if !ok {
		return
	}

	if err := h.assetSvc.RemoveSource(asset.ID, sourceID); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_graph.remove_source")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))

	handler.OK(c, nil)
}

func (h *KnowledgeGraphHandler) ListSources(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	sources, err := h.assetSvc.ListSources(asset.ID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]KnowledgeSourceResponse, 0, len(sources))
	for i := range sources {
		items = append(items, sources[i].ToResponse())
	}

	handler.OK(c, items)
}

// --- Build / Rebuild ---

func (h *KnowledgeGraphHandler) Build(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	sources, err := h.assetSvc.ListCompletedSources(asset.ID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	srcPtrs := make([]*KnowledgeSource, len(sources))
	for i := range sources {
		srcPtrs[i] = &sources[i]
	}

	if err := h.engine.Build(context.Background(), asset, srcPtrs); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_graph.build")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))
	c.Set("audit_summary", "Started graph build: "+asset.Name)

	handler.OK(c, gin.H{"message": "build enqueued"})
}

func (h *KnowledgeGraphHandler) Rebuild(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	sources, err := h.assetSvc.ListCompletedSources(asset.ID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	srcPtrs := make([]*KnowledgeSource, len(sources))
	for i := range sources {
		srcPtrs[i] = &sources[i]
	}

	if err := h.engine.Rebuild(context.Background(), asset, srcPtrs); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_graph.rebuild")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))
	c.Set("audit_summary", "Started graph rebuild: "+asset.Name)

	handler.OK(c, gin.H{"message": "rebuild enqueued"})
}

// --- Progress ---

func (h *KnowledgeGraphHandler) GetProgress(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	progress := asset.GetBuildProgress()
	if progress == nil {
		handler.OK(c, gin.H{"stage": "idle"})
		return
	}

	handler.OK(c, progress)
}

// --- Logs ---

func (h *KnowledgeGraphHandler) ListLogs(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	logs, err := h.assetSvc.ListLogs(asset.ID, limit)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, logs)
}

// --- Graph nodes ---

func (h *KnowledgeGraphHandler) ListNodes(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	keyword := c.Query("keyword")
	nodeType := c.Query("nodeType")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	nodes, total, err := h.engine.ListNodes(asset.ID, keyword, nodeType, page, pageSize)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]KnowledgeNodeResponse, 0, len(nodes))
	for i := range nodes {
		items = append(items, nodes[i].ToResponse())
	}

	handler.OK(c, gin.H{
		"items": items,
		"total": total,
		"page":  page,
	})
}

func (h *KnowledgeGraphHandler) GetNode(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	nodeID := c.Param("nodeId")
	if nodeID == "" {
		handler.Fail(c, http.StatusBadRequest, "nodeId is required")
		return
	}

	node, err := h.engine.GetNode(asset.ID, nodeID)
	if err != nil {
		handler.Fail(c, http.StatusNotFound, err.Error())
		return
	}

	handler.OK(c, node.ToResponse())
}

func (h *KnowledgeGraphHandler) GetNodeSubgraph(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	nodeID := c.Param("nodeId")
	if nodeID == "" {
		handler.Fail(c, http.StatusBadRequest, "nodeId is required")
		return
	}

	depth, _ := strconv.Atoi(c.DefaultQuery("depth", "2"))
	if depth < 1 {
		depth = 1
	}
	if depth > 5 {
		depth = 5
	}

	nodes, edges, err := h.engine.GetNodeSubgraph(asset.ID, nodeID, depth)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	nodeResps := make([]KnowledgeNodeResponse, 0, len(nodes))
	for i := range nodes {
		nodeResps = append(nodeResps, nodes[i].ToResponse())
	}
	edgeResps := make([]KnowledgeEdgeResponse, 0, len(edges))
	for i := range edges {
		edgeResps = append(edgeResps, edges[i].ToResponse())
	}

	handler.OK(c, gin.H{
		"nodes": nodeResps,
		"edges": edgeResps,
	})
}

// --- Full graph visualization ---

func (h *KnowledgeGraphHandler) GetFullGraph(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	nodes, edges, err := h.engine.GetFullGraph(asset.ID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	nodeResps := make([]KnowledgeNodeResponse, 0, len(nodes))
	for i := range nodes {
		nodeResps = append(nodeResps, nodes[i].ToResponse())
	}
	edgeResps := make([]KnowledgeEdgeResponse, 0, len(edges))
	for i := range edges {
		edgeResps = append(edgeResps, edges[i].ToResponse())
	}

	handler.OK(c, gin.H{
		"nodes": nodeResps,
		"edges": edgeResps,
	})
}

// --- Search ---

type graphSearchReq struct {
	Query string `json:"query" binding:"required"`
	TopK  int    `json:"topK"`
	Mode  string `json:"mode"`
}

func (h *KnowledgeGraphHandler) Search(c *gin.Context) {
	asset, ok := h.graphAssetID(c)
	if !ok {
		return
	}

	var req graphSearchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	query := &RecallQuery{
		Query: req.Query,
		TopK:  req.TopK,
		Mode:  req.Mode,
	}

	result, err := h.engine.Search(context.Background(), asset, query)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	handler.OK(c, result)
}
