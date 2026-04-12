package ai

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type KnowledgeBaseHandler struct {
	svc        *KnowledgeBaseService
	repo       *KnowledgeBaseRepo
	graphRepo  *KnowledgeGraphRepo
	compileSvc *KnowledgeCompileService
}

func NewKnowledgeBaseHandler(i do.Injector) (*KnowledgeBaseHandler, error) {
	return &KnowledgeBaseHandler{
		svc:        do.MustInvoke[*KnowledgeBaseService](i),
		repo:       do.MustInvoke[*KnowledgeBaseRepo](i),
		graphRepo:  do.MustInvoke[*KnowledgeGraphRepo](i),
		compileSvc: do.MustInvoke[*KnowledgeCompileService](i),
	}, nil
}

type createKBReq struct {
	Name                string `json:"name" binding:"required"`
	Description         string `json:"description"`
	CompileMethod       string `json:"compileMethod"`
	CompileModelID      *uint  `json:"compileModelId"`
	EmbeddingProviderID *uint  `json:"embeddingProviderId"`
	EmbeddingModelID    string `json:"embeddingModelId"`
	AutoCompile         bool   `json:"autoCompile"`
}

func (h *KnowledgeBaseHandler) Create(c *gin.Context) {
	var req createKBReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	compileMethod := req.CompileMethod
	if compileMethod == "" {
		compileMethod = CompileMethodKnowledgeGraph
	}

	kb := &KnowledgeBase{
		Name:                req.Name,
		Description:         req.Description,
		CompileMethod:       compileMethod,
		CompileModelID:      req.CompileModelID,
		EmbeddingProviderID: req.EmbeddingProviderID,
		EmbeddingModelID:    req.EmbeddingModelID,
		AutoCompile:         req.AutoCompile,
	}

	if err := h.svc.Create(kb); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledgeBase.create")
	c.Set("audit_resource", "ai_knowledge_base")
	c.Set("audit_resource_id", strconv.Itoa(int(kb.ID)))
	c.Set("audit_summary", "Created knowledge base: "+kb.Name)

	handler.OK(c, kb.ToResponse())
}

func (h *KnowledgeBaseHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	items, total, err := h.repo.List(KBListParams{
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]KnowledgeBaseResponse, len(items))
	for i, kb := range items {
		resp[i] = kb.ToResponse()
	}
	handler.OK(c, gin.H{"items": resp, "total": total})
}

func (h *KnowledgeBaseHandler) Get(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	kb, err := h.svc.Get(uint(id))
	if err != nil {
		if errors.Is(err, ErrKnowledgeBaseNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp := kb.ToResponse()
	nodeCount, _ := h.graphRepo.CountNodes(kb.ID)
	edgeCount, _ := h.graphRepo.CountEdges(kb.ID)
	resp.NodeCount = int(nodeCount)
	resp.EdgeCount = int(edgeCount)
	handler.OK(c, resp)
}

type updateKBReq struct {
	Name                string `json:"name" binding:"required"`
	Description         string `json:"description"`
	CompileMethod       string `json:"compileMethod"`
	CompileModelID      *uint  `json:"compileModelId"`
	EmbeddingProviderID *uint  `json:"embeddingProviderId"`
	EmbeddingModelID    string `json:"embeddingModelId"`
	AutoCompile         bool   `json:"autoCompile"`
}

func (h *KnowledgeBaseHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req updateKBReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	kb, err := h.svc.Get(uint(id))
	if err != nil {
		handler.Fail(c, http.StatusNotFound, err.Error())
		return
	}

	kb.Name = req.Name
	kb.Description = req.Description
	kb.CompileMethod = req.CompileMethod
	if kb.CompileMethod == "" {
		kb.CompileMethod = CompileMethodKnowledgeGraph
	}
	kb.CompileModelID = req.CompileModelID
	kb.EmbeddingProviderID = req.EmbeddingProviderID
	kb.EmbeddingModelID = req.EmbeddingModelID
	kb.AutoCompile = req.AutoCompile

	if err := h.svc.Update(kb); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledgeBase.update")
	c.Set("audit_resource", "ai_knowledge_base")
	c.Set("audit_resource_id", strconv.Itoa(int(kb.ID)))
	c.Set("audit_summary", "Updated knowledge base: "+kb.Name)

	handler.OK(c, kb.ToResponse())
}

func (h *KnowledgeBaseHandler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.svc.Delete(uint(id)); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "knowledgeBase.delete")
	c.Set("audit_resource", "ai_knowledge_base")
	c.Set("audit_resource_id", c.Param("id"))

	handler.OK(c, nil)
}

func (h *KnowledgeBaseHandler) Compile(c *gin.Context) {
	h.enqueueCompile(c, false, "knowledgeBase.compile", "Triggered compilation: ")
}

func (h *KnowledgeBaseHandler) Recompile(c *gin.Context) {
	h.enqueueCompile(c, true, "knowledgeBase.recompile", "Triggered recompilation: ")
}

func (h *KnowledgeBaseHandler) enqueueCompile(c *gin.Context, recompile bool, auditAction, auditPrefix string) {
	id, _ := strconv.Atoi(c.Param("id"))
	kb, err := h.svc.Get(uint(id))
	if err != nil {
		if errors.Is(err, ErrKnowledgeBaseNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	if kb.CompileStatus == CompileStatusCompiling {
		handler.Fail(c, http.StatusConflict, "compilation already in progress")
		return
	}

	kb.CompileStatus = CompileStatusCompiling
	if err := h.svc.Update(kb); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.compileSvc.EnqueueCompile(kb.ID, recompile)

	c.Set("audit_action", auditAction)
	c.Set("audit_resource", "ai_knowledge_base")
	c.Set("audit_resource_id", strconv.Itoa(int(kb.ID)))
	c.Set("audit_summary", auditPrefix+kb.Name)

	handler.OK(c, kb.ToResponse())
}

func (h *KnowledgeBaseHandler) GetProgress(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	kb, err := h.svc.Get(uint(id))
	if err != nil {
		if errors.Is(err, ErrKnowledgeBaseNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	progress := kb.GetCompileProgress()
	if progress == nil {
		progress = &CompileProgress{
			Stage: CompileStageIdle,
		}
	}

	handler.OK(c, progress)
}
