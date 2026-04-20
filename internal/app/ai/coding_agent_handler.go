package ai

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

// CodingAgentHandler exposes typed CRUD routes that force type=coding.
type CodingAgentHandler struct {
	svc  *AgentService
	repo *AgentRepo
}

func NewCodingAgentHandler(i do.Injector) (*CodingAgentHandler, error) {
	return &CodingAgentHandler{
		svc:  do.MustInvoke[*AgentService](i),
		repo: do.MustInvoke[*AgentRepo](i),
	}, nil
}

func (h *CodingAgentHandler) Create(c *gin.Context) {
	var req createAgentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	a := &Agent{
		Name:          req.Name,
		Code:          req.Code,
		Description:   req.Description,
		Avatar:        req.Avatar,
		Type:          AgentTypeCoding, // forced
		Visibility:    req.Visibility,
		CreatedBy:     userID,
		Runtime:       req.Runtime,
		RuntimeConfig: model.JSONText(req.RuntimeConfig),
		ExecMode:      req.ExecMode,
		NodeID:        req.NodeID,
		Workspace:     req.Workspace,
		Temperature:   req.Temperature,
		MaxTokens:     req.MaxTokens,
		MaxTurns:      req.MaxTurns,
		Instructions:  req.Instructions,
	}

	if a.Visibility == "" {
		a.Visibility = AgentVisibilityTeam
	}

	if err := h.svc.Create(a); err != nil {
		if errors.Is(err, ErrAgentNameConflict) || errors.Is(err, ErrAgentCodeConflict) {
			handler.Fail(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrInvalidAgentType) || errors.Is(err, ErrRuntimeRequired) ||
			errors.Is(err, ErrNodeRequired) || errors.Is(err, ErrCodeRequired) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	if len(req.ToolIDs) > 0 || len(req.SkillIDs) > 0 || len(req.MCPServerIDs) > 0 || len(req.KnowledgeBaseIDs) > 0 {
		if err := h.svc.UpdateBindings(a.ID, req.ToolIDs, req.SkillIDs, req.MCPServerIDs, req.KnowledgeBaseIDs); err != nil {
			handler.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	c.Set("audit_action", "coding-agent.create")
	c.Set("audit_resource", "ai_agent")
	c.Set("audit_resource_id", strconv.Itoa(int(a.ID)))
	c.Set("audit_summary", "Created coding agent: "+a.Name)

	handler.OK(c, h.agentWithBindings(a))
}

func (h *CodingAgentHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	agents, total, err := h.svc.List(AgentListParams{
		Keyword:    c.Query("keyword"),
		Type:       AgentTypeCoding, // forced
		Visibility: c.Query("visibility"),
		UserID:     userID,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]AgentResponse, len(agents))
	for i, a := range agents {
		items[i] = a.ToResponse()
	}

	handler.OK(c, gin.H{"items": items, "total": total})
}

func (h *CodingAgentHandler) Get(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	a, err := h.svc.GetAccessibleByType(uint(id), userID, AgentTypeCoding)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	handler.OK(c, h.agentWithBindings(a))
}

func (h *CodingAgentHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req updateAgentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	a, err := h.svc.GetOwnedByType(uint(id), userID, AgentTypeCoding)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	a.Name = req.Name
	a.Description = req.Description
	a.Avatar = req.Avatar
	if req.Visibility != "" {
		a.Visibility = req.Visibility
	}
	if req.IsActive != nil {
		a.IsActive = *req.IsActive
	}
	a.Runtime = req.Runtime
	a.RuntimeConfig = model.JSONText(req.RuntimeConfig)
	a.ExecMode = req.ExecMode
	a.NodeID = req.NodeID
	a.Workspace = req.Workspace
	a.Temperature = req.Temperature
	a.MaxTokens = req.MaxTokens
	a.MaxTurns = req.MaxTurns
	a.Instructions = req.Instructions

	if err := h.svc.Update(a); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.UpdateBindings(a.ID, req.ToolIDs, req.SkillIDs, req.MCPServerIDs, req.KnowledgeBaseIDs); err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "coding-agent.update")
	c.Set("audit_resource", "ai_agent")
	c.Set("audit_resource_id", strconv.Itoa(int(a.ID)))
	c.Set("audit_summary", "Updated coding agent: "+a.Name)

	handler.OK(c, h.agentWithBindings(a))
}

func (h *CodingAgentHandler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, err := h.svc.GetOwnedByType(uint(id), userID, AgentTypeCoding); err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			handler.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.svc.Delete(uint(id)); err != nil {
		if errors.Is(err, ErrAgentHasRunningSessions) {
			handler.Fail(c, http.StatusConflict, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "coding-agent.delete")
	c.Set("audit_resource", "ai_agent")
	c.Set("audit_resource_id", c.Param("id"))

	handler.OK(c, nil)
}

func (h *CodingAgentHandler) ListTemplates(c *gin.Context) {
	templates, err := h.svc.ListTemplatesByType(AgentTypeCoding)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]AgentTemplateResponse, len(templates))
	for i, t := range templates {
		items[i] = t.ToResponse()
	}
	handler.OK(c, items)
}

func (h *CodingAgentHandler) agentWithBindings(a *Agent) gin.H {
	resp := a.ToResponse()
	toolIDs, _ := h.repo.GetToolIDs(a.ID)
	skillIDs, _ := h.repo.GetSkillIDs(a.ID)
	mcpIDs, _ := h.repo.GetMCPServerIDs(a.ID)
	kbIDs, _ := h.repo.GetKnowledgeBaseIDs(a.ID)

	return gin.H{
		"agent":            resp,
		"toolIds":          toolIDs,
		"skillIds":         skillIDs,
		"mcpServerIds":     mcpIDs,
		"knowledgeBaseIds": kbIDs,
	}
}

// Ensure json and model imports are used.
var (
	_ = json.RawMessage{}
	_ = model.JSONText("")
)
