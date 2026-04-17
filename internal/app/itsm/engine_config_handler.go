package itsm

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type EngineConfigHandler struct {
	svc *EngineConfigService
}

func NewEngineConfigHandler(i do.Injector) (*EngineConfigHandler, error) {
	return &EngineConfigHandler{
		svc: do.MustInvoke[*EngineConfigService](i),
	}, nil
}

func (h *EngineConfigHandler) Get(c *gin.Context) {
	cfg, err := h.svc.GetConfig()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, cfg)
}

func (h *EngineConfigHandler) Update(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.UpdateConfig(&req); err != nil {
		if errors.Is(err, ErrModelNotFound) || errors.Is(err, ErrAgentNotFound) || errors.Is(err, ErrFallbackUserNotFound) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "itsm.engine_config.update")
	c.Set("audit_resource", "itsm_engine_config")
	c.Set("audit_summary", "Updated ITSM engine configuration")

	handler.OK(c, nil)
}
