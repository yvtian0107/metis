package runtime

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	openai "github.com/sashabaranov/go-openai"

	"metis/internal/handler"
	"metis/internal/llm"
)

// pid is a shorthand for parsing the :id path parameter as uint.
func pid(c *gin.Context) (uint, bool) { return handler.ParseUintParam(c, "id") }

type ProviderHandler struct {
	svc       *ProviderService
	repo      *ProviderRepo
	modelRepo *ModelRepo
}

func NewProviderHandler(i do.Injector) (*ProviderHandler, error) {
	return &ProviderHandler{
		svc:       do.MustInvoke[*ProviderService](i),
		repo:      do.MustInvoke[*ProviderRepo](i),
		modelRepo: do.MustInvoke[*ModelRepo](i),
	}, nil
}

type createProviderReq struct {
	Name    string `json:"name" binding:"required"`
	Type    string `json:"type" binding:"required"`
	BaseURL string `json:"baseUrl" binding:"required"`
	APIKey  string `json:"apiKey" binding:"required"`
}

func (h *ProviderHandler) Create(c *gin.Context) {
	var req createProviderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	p, err := h.svc.Create(req.Name, req.Type, req.BaseURL, req.APIKey)
	if err != nil {
		if errors.Is(err, ErrInvalidProviderType) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "provider.create")
	c.Set("audit_resource", "ai_provider")
	c.Set("audit_resource_id", strconv.Itoa(int(p.ID)))
	c.Set("audit_summary", "Created AI provider: "+p.Name)

	handler.OK(c, p.ToResponse(h.svc.MaskAPIKey(p), 0, nil))
}

func (h *ProviderHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	providers, total, err := h.repo.List(ProviderListParams{
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	ids := make([]uint, len(providers))
	for i, p := range providers {
		ids[i] = p.ID
	}
	modelCounts, err := h.repo.ModelCountsForProviders(ids)
	if err != nil {
		slog.Warn("ai: failed to query model counts for providers", "error", err)
	}
	modelTypeCounts, err := h.modelRepo.TypeCountsForProviders(ids)
	if err != nil {
		slog.Warn("ai: failed to query model type counts for providers", "error", err)
	}

	items := make([]ProviderResponse, len(providers))
	for i, p := range providers {
		items[i] = p.ToResponse(h.svc.MaskAPIKey(&p), modelCounts[p.ID], modelTypeCounts[p.ID])
	}

	handler.OK(c, gin.H{"items": items, "total": total})
}

func (h *ProviderHandler) Get(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	p, err := h.svc.Get(id)
	if err != nil {
		if errors.Is(err, ErrProviderNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	count, _ := h.repo.CountModelsByProviderID(p.ID)
	typeCounts, _ := h.modelRepo.TypeCountsForProviders([]uint{p.ID})
	handler.OK(c, p.ToResponse(h.svc.MaskAPIKey(p), int(count), typeCounts[p.ID]))
}

type updateProviderReq struct {
	Name    string `json:"name" binding:"required"`
	Type    string `json:"type" binding:"required"`
	BaseURL string `json:"baseUrl" binding:"required"`
	APIKey  string `json:"apiKey"`
}

func (h *ProviderHandler) Update(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	var req updateProviderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	p, err := h.svc.Update(id, req.Name, req.Type, req.BaseURL, req.APIKey)
	if err != nil {
		if errors.Is(err, ErrProviderNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrInvalidProviderType) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "provider.update")
	c.Set("audit_resource", "ai_provider")
	c.Set("audit_resource_id", strconv.Itoa(int(p.ID)))
	c.Set("audit_summary", "Updated AI provider: "+p.Name)

	count, _ := h.repo.CountModelsByProviderID(p.ID)
	typeCounts, _ := h.modelRepo.TypeCountsForProviders([]uint{p.ID})
	handler.OK(c, p.ToResponse(h.svc.MaskAPIKey(p), int(count), typeCounts[p.ID]))
}

func (h *ProviderHandler) Delete(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(id); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "provider.delete")
	c.Set("audit_resource", "ai_provider")
	c.Set("audit_resource_id", strconv.FormatUint(uint64(id), 10))

	handler.OK(c, nil)
}

func (h *ProviderHandler) TestConnection(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	p, err := h.svc.Get(id)
	if err != nil {
		handler.Fail(c, http.StatusNotFound, err.Error())
		return
	}

	apiKey, err := h.svc.DecryptAPIKey(p)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, "failed to decrypt api key")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var testErr error
	switch p.Protocol {
	case "openai":
		testErr = testOpenAIConnection(ctx, p.BaseURL, apiKey)
	case "anthropic":
		modelID := h.resolveAnthropicTestModel(p.ID)
		testErr = testAnthropicConnection(ctx, p.BaseURL, apiKey, modelID)
	}

	if testErr != nil {
		h.repo.UpdateStatus(p.ID, ProviderStatusError)
		handler.OK(c, gin.H{"success": false, "error": testErr.Error()})
		return
	}

	h.repo.UpdateStatus(p.ID, ProviderStatusActive)
	handler.OK(c, gin.H{"success": true})
}

func testOpenAIConnection(ctx context.Context, baseURL, apiKey string) error {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(cfg)
	_, err := client.ListModels(ctx)
	return err
}

func testAnthropicConnection(ctx context.Context, baseURL, apiKey, modelID string) error {
	client, err := llm.NewClient(llm.ProtocolAnthropic, baseURL, apiKey)
	if err != nil {
		return err
	}
	_, err = client.Chat(ctx, llm.ChatRequest{
		Model:     modelID,
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
		MaxTokens: 1,
	})
	return err
}

const defaultAnthropicTestModel = "claude-haiku-3-5-20241022"

// resolveAnthropicTestModel picks a model ID for the connectivity test.
// It prefers the provider's default LLM, then any active model, falling back
// to a well-known model if none are synced yet.
func (h *ProviderHandler) resolveAnthropicTestModel(providerID uint) string {
	models, err := h.modelRepo.ListByProviderID(providerID)
	if err != nil || len(models) == 0 {
		return defaultAnthropicTestModel
	}
	// Prefer the default model of this provider.
	for _, m := range models {
		if m.IsDefault && m.Status == ModelStatusActive {
			return m.ModelID
		}
	}
	// Otherwise pick first active model.
	for _, m := range models {
		if m.Status == ModelStatusActive {
			return m.ModelID
		}
	}
	return defaultAnthropicTestModel
}
