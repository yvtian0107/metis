package token

import (
	"errors"
	. "metis/internal/app/observe/domain"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
	"metis/internal/service"
)

type IntegrationTokenHandler struct {
	svc       *IntegrationTokenService
	sysConfig *service.SysConfigService
}

func NewIntegrationTokenHandler(i do.Injector) (*IntegrationTokenHandler, error) {
	return &IntegrationTokenHandler{
		svc:       do.MustInvoke[*IntegrationTokenService](i),
		sysConfig: do.MustInvoke[*service.SysConfigService](i),
	}, nil
}

type createTokenRequest struct {
	Name string `json:"name" binding:"required"`
}

// Create POST /api/v1/observe/tokens
func (h *IntegrationTokenHandler) Create(c *gin.Context) {
	var req createTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, "name is required")
		return
	}

	userID := c.GetUint("userId")
	_, t, err := h.svc.Create(userID, req.Name)
	if err != nil {
		if errors.Is(err, ErrTokenLimitReached) {
			handler.Fail(c, http.StatusUnprocessableEntity, "token limit reached (max 10)")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, handler.R{
		Code:    0,
		Message: "ok",
		Data: CreateTokenResponse{
			TokenResponse: t.ToResponse(),
		},
	})
}

// List GET /api/v1/observe/tokens
func (h *IntegrationTokenHandler) List(c *gin.Context) {
	userID := c.GetUint("userId")
	tokens, err := h.svc.List(userID)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]TokenResponse, len(tokens))
	for i, t := range tokens {
		resp[i] = t.ToResponse()
	}
	handler.OK(c, resp)
}

// Revoke DELETE /api/v1/observe/tokens/:id
func (h *IntegrationTokenHandler) Revoke(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		handler.Fail(c, http.StatusBadRequest, "invalid token id")
		return
	}

	userID := c.GetUint("userId")
	if err := h.svc.Revoke(uint(id), userID); err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			handler.Fail(c, http.StatusNotFound, "token not found")
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, nil)
}

// GetSettings GET /api/v1/observe/settings
func (h *IntegrationTokenHandler) GetSettings(c *gin.Context) {
	cfg, err := h.sysConfig.Get("observe.otel_endpoint")
	endpoint := ""
	if err == nil {
		endpoint = cfg.Value
	}
	handler.OK(c, gin.H{"otelEndpoint": endpoint})
}
