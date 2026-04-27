package runtime

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
	"metis/internal/model"
)

type ModelHandler struct {
	svc  *ModelService
	repo *ModelRepo
}

func NewModelHandler(i do.Injector) (*ModelHandler, error) {
	return &ModelHandler{
		svc:  do.MustInvoke[*ModelService](i),
		repo: do.MustInvoke[*ModelRepo](i),
	}, nil
}

// mid is a shorthand for parsing the :id path parameter as uint.
func mid(c *gin.Context) (uint, bool) { return handler.ParseUintParam(c, "id") }

type createModelReq struct {
	ModelID         string          `json:"modelId" binding:"required"`
	DisplayName     string          `json:"displayName" binding:"required"`
	ProviderID      uint            `json:"providerId" binding:"required"`
	Type            string          `json:"type" binding:"required"`
	Capabilities    json.RawMessage `json:"capabilities"`
	ContextWindow   int             `json:"contextWindow"`
	MaxOutputTokens int             `json:"maxOutputTokens"`
	InputPrice      float64         `json:"inputPrice"`
	OutputPrice     float64         `json:"outputPrice"`
}

func (h *ModelHandler) Create(c *gin.Context) {
	var req createModelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	caps := model.JSONText(req.Capabilities)
	if len(caps) == 0 {
		caps = model.JSONText("[]")
	}

	m := &AIModel{
		ModelID:         req.ModelID,
		DisplayName:     req.DisplayName,
		ProviderID:      req.ProviderID,
		Type:            req.Type,
		Capabilities:    caps,
		ContextWindow:   req.ContextWindow,
		MaxOutputTokens: req.MaxOutputTokens,
		InputPrice:      req.InputPrice,
		OutputPrice:     req.OutputPrice,
		Status:          ModelStatusActive,
	}

	if err := h.svc.Create(m); err != nil {
		if errors.Is(err, ErrInvalidType) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "model.create")
	c.Set("audit_resource", "ai_model")
	c.Set("audit_resource_id", strconv.Itoa(int(m.ID)))
	c.Set("audit_summary", "Created AI model: "+m.DisplayName)

	handler.OK(c, m.ToResponse())
}

func (h *ModelHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	providerID, _ := strconv.Atoi(c.DefaultQuery("providerId", "0"))

	models, total, err := h.repo.List(ModelListParams{
		Keyword:    c.Query("keyword"),
		Type:       c.Query("type"),
		ProviderID: uint(providerID),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]AIModelResponse, len(models))
	for i, m := range models {
		items[i] = m.ToResponse()
	}

	handler.OK(c, gin.H{"items": items, "total": total})
}

func (h *ModelHandler) Get(c *gin.Context) {
	id, ok := mid(c)
	if !ok {
		return
	}
	m, err := h.svc.Get(id)
	if err != nil {
		if errors.Is(err, ErrModelNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, m.ToResponse())
}

type updateModelReq struct {
	ModelID         string          `json:"modelId" binding:"required"`
	DisplayName     string          `json:"displayName" binding:"required"`
	Type            string          `json:"type" binding:"required"`
	Capabilities    json.RawMessage `json:"capabilities"`
	ContextWindow   int             `json:"contextWindow"`
	MaxOutputTokens int             `json:"maxOutputTokens"`
	InputPrice      float64         `json:"inputPrice"`
	OutputPrice     float64         `json:"outputPrice"`
	Status          string          `json:"status"`
}

func (h *ModelHandler) Update(c *gin.Context) {
	id, ok := mid(c)
	if !ok {
		return
	}
	var req updateModelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	m, err := h.svc.Get(id)
	if err != nil {
		handler.Fail(c, http.StatusNotFound, err.Error())
		return
	}

	m.ModelID = req.ModelID
	m.DisplayName = req.DisplayName
	m.Type = req.Type
	m.Capabilities = model.JSONText(req.Capabilities)
	m.ContextWindow = req.ContextWindow
	m.MaxOutputTokens = req.MaxOutputTokens
	m.InputPrice = req.InputPrice
	m.OutputPrice = req.OutputPrice
	if req.Status != "" {
		m.Status = req.Status
	}

	if err := h.svc.Update(m); err != nil {
		if errors.Is(err, ErrInvalidType) || errors.Is(err, ErrInvalidStatus) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "model.update")
	c.Set("audit_resource", "ai_model")
	c.Set("audit_resource_id", strconv.Itoa(int(m.ID)))
	c.Set("audit_summary", "Updated AI model: "+m.DisplayName)

	handler.OK(c, m.ToResponse())
}

func (h *ModelHandler) Delete(c *gin.Context) {
	id, ok := mid(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(id); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "model.delete")
	c.Set("audit_resource", "ai_model")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))

	handler.OK(c, nil)
}

func (h *ModelHandler) SetDefault(c *gin.Context) {
	id, ok := mid(c)
	if !ok {
		return
	}
	if err := h.svc.SetDefault(id); err != nil {
		if errors.Is(err, ErrModelNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "model.setDefault")
	c.Set("audit_resource", "ai_model")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "Set as default model")

	handler.OK(c, nil)
}

func (h *ModelHandler) SyncModels(c *gin.Context) {
	id, ok := mid(c)
	if !ok {
		return
	}
	added, err := h.svc.SyncModels(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrProviderNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "model.sync")
	c.Set("audit_resource", "ai_model")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))
	c.Set("audit_summary", "Synced models from provider")

	handler.OK(c, gin.H{"added": added})
}
