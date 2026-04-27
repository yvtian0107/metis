package config

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

func (h *EngineConfigHandler) GetSmartStaffing(c *gin.Context) {
	cfg, err := h.svc.GetSmartStaffingConfig()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, cfg)
}

func (h *EngineConfigHandler) UpdateSmartStaffing(c *gin.Context) {
	var req UpdateSmartStaffingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.UpdateSmartStaffingConfig(&req); err != nil {
		if errors.Is(err, ErrModelNotFound) || errors.Is(err, ErrAgentNotFound) || errors.Is(err, ErrFallbackUserNotFound) || errors.Is(err, ErrInvalidEngineConfig) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "itsm.smart_staffing.update")
	c.Set("audit_resource", "itsm_smart_staffing")
	c.Set("audit_summary", "Updated ITSM smart staffing configuration")

	handler.OK(c, nil)
}

func (h *EngineConfigHandler) GetEngineSettings(c *gin.Context) {
	cfg, err := h.svc.GetEngineSettingsConfig()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, cfg)
}

func (h *EngineConfigHandler) UpdateEngineSettings(c *gin.Context) {
	var req UpdateEngineSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.UpdateEngineSettingsConfig(&req); err != nil {
		if errors.Is(err, ErrModelNotFound) || errors.Is(err, ErrAgentNotFound) || errors.Is(err, ErrFallbackUserNotFound) || errors.Is(err, ErrInvalidEngineConfig) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "itsm.engine_settings.update")
	c.Set("audit_resource", "itsm_engine_settings")
	c.Set("audit_summary", "Updated ITSM engine settings")

	handler.OK(c, nil)
}
