package runtime

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

// KnowledgeKBHandler exposes REST endpoints for knowledge base (RAG) assets.
type KnowledgeKBHandler struct {
	assetSvc  *KnowledgeAssetService
	engine    *NaiveChunkEngine
	chunkRepo *RAGChunkRepo
}

func NewKnowledgeKBHandler(i do.Injector) (*KnowledgeKBHandler, error) {
	return &KnowledgeKBHandler{
		assetSvc:  do.MustInvoke[*KnowledgeAssetService](i),
		engine:    do.MustInvoke[*NaiveChunkEngine](i),
		chunkRepo: do.MustInvoke[*RAGChunkRepo](i),
	}, nil
}

// kbAssetID parses :id and validates it is a KB asset.
func (h *KnowledgeKBHandler) kbAssetID(c *gin.Context) (*KnowledgeAsset, bool) {
	id, ok := handler.ParseUintParam(c, "id")
	if !ok {
		return nil, false
	}
	asset, err := h.assetSvc.GetByCategory(id, AssetCategoryKB)
	if err != nil {
		handler.Fail(c, http.StatusNotFound, err.Error())
		return nil, false
	}
	return asset, true
}

// --- CRUD ---

type createKBReq struct {
	Name                string     `json:"name" binding:"required"`
	Description         string     `json:"description"`
	Type                string     `json:"type" binding:"required"`
	Config              *RAGConfig `json:"config"`
	EmbeddingProviderID *uint      `json:"embeddingProviderId"`
	EmbeddingModelID    string     `json:"embeddingModelId"`
	AutoBuild           bool       `json:"autoBuild"`
}

func (h *KnowledgeKBHandler) Create(c *gin.Context) {
	var req createKBReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	meta := GetAssetType(AssetCategoryKB, req.Type)
	if meta == nil {
		handler.Fail(c, http.StatusBadRequest, "unsupported knowledge base type: "+req.Type)
		return
	}

	asset := &KnowledgeAsset{
		Name:                req.Name,
		Description:         req.Description,
		Category:            AssetCategoryKB,
		Type:                req.Type,
		EmbeddingProviderID: req.EmbeddingProviderID,
		EmbeddingModelID:    req.EmbeddingModelID,
		AutoBuild:           req.AutoBuild,
	}

	cfg := req.Config
	if cfg == nil {
		dflt := DefaultRAGConfig()
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

	c.Set("audit_action", "knowledge_base.create")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))
	c.Set("audit_summary", "Created knowledge base: "+asset.Name)

	handler.OK(c, asset.ToResponse())
}

func (h *KnowledgeKBHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := AssetListParams{
		Category: AssetCategoryKB,
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
			resp.ChunkCount = stats.ChunkCount
		}
		items = append(items, resp)
	}

	handler.OK(c, gin.H{
		"items": items,
		"total": total,
		"page":  page,
	})
}

func (h *KnowledgeKBHandler) Get(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
	if !ok {
		return
	}

	resp := asset.ToResponse()
	stats, _ := h.engine.ContentStats(c, asset)
	if stats != nil {
		resp.ChunkCount = stats.ChunkCount
	}

	handler.OK(c, resp)
}

type updateKBReq struct {
	Name                string     `json:"name" binding:"required"`
	Description         string     `json:"description"`
	Config              *RAGConfig `json:"config"`
	EmbeddingProviderID *uint      `json:"embeddingProviderId"`
	EmbeddingModelID    string     `json:"embeddingModelId"`
	AutoBuild           bool       `json:"autoBuild"`
}

func (h *KnowledgeKBHandler) Update(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
	if !ok {
		return
	}

	var req updateKBReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.assetSvc.Update(
		asset.ID, req.Name, req.Description, req.Config,
		nil, req.EmbeddingProviderID, req.EmbeddingModelID,
		req.AutoBuild,
	)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_base.update")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))
	c.Set("audit_summary", "Updated knowledge base: "+updated.Name)

	handler.OK(c, updated.ToResponse())
}

func (h *KnowledgeKBHandler) Delete(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
	if !ok {
		return
	}

	// Delete all chunks
	_ = h.chunkRepo.DeleteByAsset(asset.ID)

	if err := h.assetSvc.Delete(asset.ID); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_base.delete")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))

	handler.OK(c, nil)
}

// --- Source association ---

type addKBSourcesReq struct {
	SourceIDs []uint `json:"sourceIds" binding:"required,min=1"`
}

func (h *KnowledgeKBHandler) AddSources(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
	if !ok {
		return
	}

	var req addKBSourcesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.assetSvc.AddSources(asset.ID, req.SourceIDs); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_base.add_sources")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))

	handler.OK(c, nil)
}

func (h *KnowledgeKBHandler) RemoveSource(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
	if !ok {
		return
	}

	sourceID, ok := handler.ParseUintParam(c, "sourceId")
	if !ok {
		return
	}

	// Also delete chunks from this source
	_ = h.chunkRepo.DeleteBySource(asset.ID, sourceID)

	if err := h.assetSvc.RemoveSource(asset.ID, sourceID); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_base.remove_source")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))

	handler.OK(c, nil)
}

func (h *KnowledgeKBHandler) ListSources(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
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

func (h *KnowledgeKBHandler) Build(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
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

	// Build synchronously (chunking is fast; embedding will be async in Phase 9)
	if err := h.engine.Build(context.Background(), asset, srcPtrs); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledge_base.build")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))

	handler.OK(c, gin.H{"message": "build completed"})
}

func (h *KnowledgeKBHandler) Rebuild(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
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

	c.Set("audit_action", "knowledge_base.rebuild")
	c.Set("audit_resource", "ai_knowledge_asset")
	c.Set("audit_resource_id", strconv.Itoa(int(asset.ID)))

	handler.OK(c, gin.H{"message": "rebuild completed"})
}

// --- Progress ---

func (h *KnowledgeKBHandler) GetProgress(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
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

// --- Chunks ---

func (h *KnowledgeKBHandler) ListChunks(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	chunks, total, err := h.chunkRepo.ListByAsset(asset.ID, page, pageSize)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]RAGChunkResponse, 0, len(chunks))
	for i := range chunks {
		items = append(items, chunks[i].ToResponse())
	}

	handler.OK(c, gin.H{
		"items": items,
		"total": total,
		"page":  page,
	})
}

// --- Logs ---

func (h *KnowledgeKBHandler) ListLogs(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
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

// --- Search ---

type kbSearchReq struct {
	Query string `json:"query" binding:"required"`
	TopK  int    `json:"topK"`
	Mode  string `json:"mode"`
}

func (h *KnowledgeKBHandler) Search(c *gin.Context) {
	asset, ok := h.kbAssetID(c)
	if !ok {
		return
	}

	var req kbSearchReq
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
