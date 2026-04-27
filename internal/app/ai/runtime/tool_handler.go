package runtime

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

type ToolHandler struct {
	svc        *ToolService
	runtimeSvc *ToolRuntimeService
}

func NewToolHandler(i do.Injector) (*ToolHandler, error) {
	return &ToolHandler{
		svc:        do.MustInvoke[*ToolService](i),
		runtimeSvc: do.MustInvoke[*ToolRuntimeService](i),
	}, nil
}

type toolkitGroup struct {
	Toolkit string         `json:"toolkit"`
	Tools   []ToolResponse `json:"tools"`
}

func (h *ToolHandler) List(c *gin.Context) {
	tools, err := h.svc.List()
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Group by toolkit, preserve insertion order
	orderMap := map[string]int{}
	var groups []toolkitGroup
	for _, resp := range tools {
		idx, ok := orderMap[resp.Toolkit]
		if !ok {
			idx = len(groups)
			orderMap[resp.Toolkit] = idx
			groups = append(groups, toolkitGroup{Toolkit: resp.Toolkit})
		}
		groups[idx].Tools = append(groups[idx].Tools, resp)
	}
	handler.OK(c, gin.H{"items": groups})
}

type toggleToolReq struct {
	IsActive      bool `json:"isActive"`
	ConfirmImpact bool `json:"confirmImpact"`
}

func (h *ToolHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req toggleToolReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	t, err := h.svc.ToggleActive(uint(id), req.IsActive, ToggleToolOptions{ConfirmImpact: req.ConfirmImpact})
	if err != nil {
		if errors.Is(err, ErrToolNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrToolNotExecutable) {
			handler.Fail(c, http.StatusConflict, err.Error())
			return
		}
		var impactErr *ToolImpactError
		if errors.As(err, &impactErr) {
			c.JSON(http.StatusConflict, handler.R{
				Code:    -1,
				Message: "tool is bound to agents; confirmation required",
				Data: gin.H{
					"toolName":        impactErr.ToolName,
					"boundAgentCount": impactErr.BoundAgentCount,
				},
			})
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "tool.update")
	c.Set("audit_resource", "ai_tool")
	c.Set("audit_resource_id", strconv.Itoa(int(t.ID)))
	c.Set("audit_summary", "Toggled tool: "+t.Name)

	handler.OK(c, t)
}

func (h *ToolHandler) UpdateRuntime(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req UpdateToolRuntimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	t, err := h.runtimeSvc.UpdateRuntimeConfig(uint(id), req.RuntimeConfig)
	if err != nil {
		switch {
		case errors.Is(err, ErrToolNotFound):
			handler.Fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrToolRuntimeNotConfigured), errors.Is(err, ErrToolRuntimeInvalid), errors.Is(err, ErrModelNotFound):
			handler.Fail(c, http.StatusBadRequest, err.Error())
		default:
			handler.Fail(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.Set("audit_action", "tool.runtime.update")
	c.Set("audit_resource", "ai_tool")
	c.Set("audit_resource_id", strconv.Itoa(int(t.ID)))
	c.Set("audit_summary", "Updated tool runtime: "+t.Name)

	handler.OK(c, t)
}
