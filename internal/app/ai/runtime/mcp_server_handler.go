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

type MCPServerHandler struct {
	svc *MCPServerService
}

func NewMCPServerHandler(i do.Injector) (*MCPServerHandler, error) {
	return &MCPServerHandler{
		svc: do.MustInvoke[*MCPServerService](i),
	}, nil
}

type createMCPServerReq struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description"`
	Transport   string          `json:"transport" binding:"required"`
	URL         string          `json:"url"`
	Command     string          `json:"command"`
	Args        json.RawMessage `json:"args"`
	Env         json.RawMessage `json:"env"`
	AuthType    string          `json:"authType"`
	AuthConfig  string          `json:"authConfig"`
	IsActive    bool            `json:"isActive"`
}

func (h *MCPServerHandler) Create(c *gin.Context) {
	var req createMCPServerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	authType := req.AuthType
	if authType == "" {
		authType = AuthTypeNone
	}

	m := &MCPServer{
		Name:        req.Name,
		Description: req.Description,
		Transport:   req.Transport,
		URL:         req.URL,
		Command:     req.Command,
		Args:        model.JSONText(req.Args),
		Env:         model.JSONText(req.Env),
		AuthType:    authType,
		IsActive:    req.IsActive,
	}

	if err := h.svc.Create(m, req.AuthConfig); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidTransport) || errors.Is(err, ErrSSERequiresURL) || errors.Is(err, ErrSTDIORequiresCommand) {
			status = http.StatusBadRequest
		}
		handler.Fail(c, status, err.Error())
		return
	}

	c.Set("audit_action", "mcpServer.create")
	c.Set("audit_resource", "ai_mcp_server")
	c.Set("audit_resource_id", strconv.Itoa(int(m.ID)))
	c.Set("audit_summary", "Created MCP server: "+m.Name)

	handler.OK(c, m.ToResponse(h.svc.MaskAuthConfig(m)))
}

func (h *MCPServerHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	servers, total, err := h.svc.List(MCPServerListParams{
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]MCPServerResponse, len(servers))
	for i, s := range servers {
		items[i] = s.ToResponse(h.svc.MaskAuthConfig(&s))
	}
	handler.OK(c, gin.H{"items": items, "total": total})
}

func (h *MCPServerHandler) Get(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	m, err := h.svc.Get(uint(id))
	if err != nil {
		if errors.Is(err, ErrMCPServerNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, m.ToResponse(h.svc.MaskAuthConfig(m)))
}

type updateMCPServerReq struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description"`
	Transport   string          `json:"transport" binding:"required"`
	URL         string          `json:"url"`
	Command     string          `json:"command"`
	Args        json.RawMessage `json:"args"`
	Env         json.RawMessage `json:"env"`
	AuthType    string          `json:"authType"`
	AuthConfig  string          `json:"authConfig"`
	IsActive    bool            `json:"isActive"`
}

func (h *MCPServerHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req updateMCPServerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	authType := req.AuthType
	if authType == "" {
		authType = AuthTypeNone
	}

	updates := &MCPServer{
		Name:        req.Name,
		Description: req.Description,
		Transport:   req.Transport,
		URL:         req.URL,
		Command:     req.Command,
		Args:        model.JSONText(req.Args),
		Env:         model.JSONText(req.Env),
		AuthType:    authType,
		IsActive:    req.IsActive,
	}

	m, err := h.svc.Update(uint(id), updates, req.AuthConfig)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrMCPServerNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, ErrInvalidTransport) || errors.Is(err, ErrSSERequiresURL) || errors.Is(err, ErrSTDIORequiresCommand) {
			status = http.StatusBadRequest
		}
		handler.Fail(c, status, err.Error())
		return
	}

	c.Set("audit_action", "mcpServer.update")
	c.Set("audit_resource", "ai_mcp_server")
	c.Set("audit_resource_id", strconv.Itoa(int(m.ID)))
	c.Set("audit_summary", "Updated MCP server: "+m.Name)

	handler.OK(c, m.ToResponse(h.svc.MaskAuthConfig(m)))
}

func (h *MCPServerHandler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.svc.Delete(uint(id)); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "mcpServer.delete")
	c.Set("audit_resource", "ai_mcp_server")
	c.Set("audit_resource_id", c.Param("id"))

	handler.OK(c, nil)
}

func (h *MCPServerHandler) TestConnection(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	m, err := h.svc.Get(uint(id))
	if err != nil {
		handler.Fail(c, http.StatusNotFound, err.Error())
		return
	}

	if m.Transport != MCPTransportSSE {
		handler.OK(c, gin.H{
			"success": false,
			"error":   "test connection is only available for SSE transport",
		})
		return
	}

	tools, err := h.svc.TestConnection(c.Request.Context(), m.ID)
	if err != nil {
		handler.OK(c, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	handler.OK(c, gin.H{
		"success": true,
		"tools":   tools,
	})
}
