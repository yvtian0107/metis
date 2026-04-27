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

type AgentHandler struct {
	svc  *AgentService
	repo *AgentRepo
}

func NewAgentHandler(i do.Injector) (*AgentHandler, error) {
	return &AgentHandler{
		svc:  do.MustInvoke[*AgentService](i),
		repo: do.MustInvoke[*AgentRepo](i),
	}, nil
}

type createAgentReq struct {
	Name              string                      `json:"name" binding:"required"`
	Code              *string                     `json:"code"`
	Description       string                      `json:"description"`
	Avatar            string                      `json:"avatar"`
	Type              string                      `json:"type" binding:"required"`
	Visibility        string                      `json:"visibility"`
	Strategy          string                      `json:"strategy"`
	ModelID           *uint                       `json:"modelId"`
	SystemPrompt      string                      `json:"systemPrompt"`
	Temperature       float64                     `json:"temperature"`
	MaxTokens         int                         `json:"maxTokens"`
	MaxTurns          int                         `json:"maxTurns"`
	Runtime           string                      `json:"runtime"`
	RuntimeConfig     json.RawMessage             `json:"runtimeConfig"`
	ExecMode          string                      `json:"execMode"`
	NodeID            *uint                       `json:"nodeId"`
	Workspace         string                      `json:"workspace"`
	Instructions      string                      `json:"instructions"`
	ToolIDs           []uint                      `json:"toolIds"`
	SkillIDs          []uint                      `json:"skillIds"`
	MCPServerIDs      []uint                      `json:"mcpServerIds"`
	KnowledgeBaseIDs  []uint                      `json:"knowledgeBaseIds"`
	KnowledgeGraphIDs []uint                      `json:"knowledgeGraphIds"`
	CapabilitySets    []AgentCapabilitySetBinding `json:"capabilitySetBindings"`
	TemplateID        *uint                       `json:"templateId"`
}

func (h *AgentHandler) Create(c *gin.Context) {
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
		Type:          req.Type,
		Visibility:    req.Visibility,
		CreatedBy:     userID,
		Strategy:      req.Strategy,
		ModelID:       req.ModelID,
		SystemPrompt:  req.SystemPrompt,
		Temperature:   req.Temperature,
		MaxTokens:     req.MaxTokens,
		MaxTurns:      req.MaxTurns,
		Runtime:       req.Runtime,
		RuntimeConfig: model.JSONText(req.RuntimeConfig),
		ExecMode:      req.ExecMode,
		NodeID:        req.NodeID,
		Workspace:     req.Workspace,
		Instructions:  req.Instructions,
	}

	if a.Visibility == "" {
		a.Visibility = AgentVisibilityTeam
	}

	if err := h.svc.CreateWithBindings(a, agentBindingsFromCreateReq(req)); err != nil {
		if errors.Is(err, ErrAgentNameConflict) || errors.Is(err, ErrAgentCodeConflict) {
			handler.Fail(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrInvalidAgentType) || errors.Is(err, ErrModelRequired) ||
			errors.Is(err, ErrRuntimeRequired) || errors.Is(err, ErrNodeRequired) ||
			errors.Is(err, ErrCodeRequired) || errors.Is(err, ErrInvalidBinding) {
			handler.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		handler.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Set("audit_action", "agent.create")
	c.Set("audit_resource", "ai_agent")
	c.Set("audit_resource_id", strconv.Itoa(int(a.ID)))
	c.Set("audit_summary", "Created agent: "+a.Name)

	handler.OK(c, h.agentWithBindings(a))
}

func (h *AgentHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	agents, total, err := h.svc.List(AgentListParams{
		Keyword:    c.Query("keyword"),
		Type:       c.Query("type"),
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

func (h *AgentHandler) Get(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	a, err := h.svc.GetAccessible(uint(id), userID)
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

type updateAgentReq struct {
	Name              string                      `json:"name" binding:"required"`
	Description       string                      `json:"description"`
	Avatar            string                      `json:"avatar"`
	Visibility        string                      `json:"visibility"`
	IsActive          *bool                       `json:"isActive"`
	Strategy          string                      `json:"strategy"`
	ModelID           *uint                       `json:"modelId"`
	SystemPrompt      string                      `json:"systemPrompt"`
	Temperature       float64                     `json:"temperature"`
	MaxTokens         int                         `json:"maxTokens"`
	MaxTurns          int                         `json:"maxTurns"`
	Runtime           string                      `json:"runtime"`
	RuntimeConfig     json.RawMessage             `json:"runtimeConfig"`
	ExecMode          string                      `json:"execMode"`
	NodeID            *uint                       `json:"nodeId"`
	Workspace         string                      `json:"workspace"`
	Instructions      string                      `json:"instructions"`
	ToolIDs           []uint                      `json:"toolIds"`
	SkillIDs          []uint                      `json:"skillIds"`
	MCPServerIDs      []uint                      `json:"mcpServerIds"`
	KnowledgeBaseIDs  []uint                      `json:"knowledgeBaseIds"`
	KnowledgeGraphIDs []uint                      `json:"knowledgeGraphIds"`
	CapabilitySets    []AgentCapabilitySetBinding `json:"capabilitySetBindings"`
}

func (h *AgentHandler) Update(c *gin.Context) {
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

	a, err := h.svc.GetOwned(uint(id), userID)
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
	a.Strategy = req.Strategy
	a.ModelID = req.ModelID
	a.SystemPrompt = req.SystemPrompt
	a.Temperature = req.Temperature
	a.MaxTokens = req.MaxTokens
	a.MaxTurns = req.MaxTurns
	a.Runtime = req.Runtime
	a.RuntimeConfig = model.JSONText(req.RuntimeConfig)
	a.ExecMode = req.ExecMode
	a.NodeID = req.NodeID
	a.Workspace = req.Workspace
	a.Instructions = req.Instructions

	if err := h.svc.UpdateWithBindings(a, agentBindingsFromUpdateReq(req)); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Set("audit_action", "agent.update")
	c.Set("audit_resource", "ai_agent")
	c.Set("audit_resource_id", strconv.Itoa(int(a.ID)))
	c.Set("audit_summary", "Updated agent: "+a.Name)

	handler.OK(c, h.agentWithBindings(a))
}

func (h *AgentHandler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userID, ok := requireUserID(c)
	if !ok {
		handler.Fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, err := h.svc.GetOwned(uint(id), userID); err != nil {
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

	c.Set("audit_action", "agent.delete")
	c.Set("audit_resource", "ai_agent")
	c.Set("audit_resource_id", c.Param("id"))

	handler.OK(c, nil)
}

func (h *AgentHandler) ListTemplates(c *gin.Context) {
	templates, err := h.svc.ListTemplates()
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

// agentWithBindings returns agent response enriched with binding IDs
func (h *AgentHandler) agentWithBindings(a *Agent) gin.H {
	resp := a.ToResponse()
	toolIDs, _ := h.repo.GetToolIDs(a.ID)
	skillIDs, _ := h.repo.GetSkillIDs(a.ID)
	mcpIDs, _ := h.repo.GetMCPServerIDs(a.ID)
	kbIDs, _ := h.repo.GetKnowledgeBaseIDs(a.ID)
	kgIDs, _ := h.repo.GetKnowledgeGraphIDs(a.ID)
	capabilitySetBindings, _ := h.repo.GetCapabilitySetBindings(a.ID)

	return gin.H{
		"agent":                 resp,
		"toolIds":               toolIDs,
		"skillIds":              skillIDs,
		"mcpServerIds":          mcpIDs,
		"knowledgeBaseIds":      kbIDs,
		"knowledgeGraphIds":     kgIDs,
		"capabilitySetBindings": capabilitySetBindings,
	}
}

func agentBindingsFromCreateReq(req createAgentReq) AgentBindings {
	return AgentBindings{
		ToolIDs:           req.ToolIDs,
		SkillIDs:          req.SkillIDs,
		MCPServerIDs:      req.MCPServerIDs,
		KnowledgeBaseIDs:  req.KnowledgeBaseIDs,
		KnowledgeGraphIDs: req.KnowledgeGraphIDs,
		CapabilitySets:    req.CapabilitySets,
	}
}

func agentBindingsFromUpdateReq(req updateAgentReq) AgentBindings {
	return AgentBindings{
		ToolIDs:           req.ToolIDs,
		SkillIDs:          req.SkillIDs,
		MCPServerIDs:      req.MCPServerIDs,
		KnowledgeBaseIDs:  req.KnowledgeBaseIDs,
		KnowledgeGraphIDs: req.KnowledgeGraphIDs,
		CapabilitySets:    req.CapabilitySets,
	}
}
