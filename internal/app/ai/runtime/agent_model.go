package runtime

import (
	"encoding/json"
	"time"

	"metis/internal/model"
)

// Agent types
const (
	AgentTypeAssistant = "assistant"
	AgentTypeCoding    = "coding"
	AgentTypeInternal  = "internal"
)

// Agent strategies (assistant type)
const (
	AgentStrategyReact          = "react"
	AgentStrategyPlanAndExecute = "plan_and_execute"
)

// Agent execution modes (coding type)
const (
	AgentExecModeLocal  = "local"
	AgentExecModeRemote = "remote"
)

// Agent coding runtimes
const (
	AgentRuntimeClaudeCode = "claude-code"
	AgentRuntimeCodex      = "codex"
	AgentRuntimeOpenCode   = "opencode"
	AgentRuntimeAider      = "aider"
)

// Agent visibility
const (
	AgentVisibilityPrivate = "private"
	AgentVisibilityTeam    = "team"
	AgentVisibilityPublic  = "public"
)

// Agent session statuses
const (
	SessionStatusRunning   = "running"
	SessionStatusCompleted = "completed"
	SessionStatusCancelled = "cancelled"
	SessionStatusError     = "error"
)

// Session message roles
const (
	MessageRoleUser       = "user"
	MessageRoleAssistant  = "assistant"
	MessageRoleToolCall   = "tool_call"
	MessageRoleToolResult = "tool_result"
)

// Agent memory sources
const (
	MemorySourceAgentGenerated = "agent_generated"
	MemorySourceUserSet        = "user_set"
	MemorySourceSystem         = "system"
)

// --- Agent ---

type Agent struct {
	model.BaseModel
	Name        string  `json:"name" gorm:"size:128;uniqueIndex;not null"`
	Code        *string `json:"code" gorm:"size:128;uniqueIndex"`
	Description string  `json:"description" gorm:"type:text"`
	Avatar      string  `json:"avatar" gorm:"size:256"`
	Type        string  `json:"type" gorm:"size:16;not null"`
	IsActive    bool    `json:"isActive" gorm:"not null;default:true"`
	Visibility  string  `json:"visibility" gorm:"size:16;not null;default:team"`
	CreatedBy   uint    `json:"createdBy" gorm:"not null;index"`

	// assistant type fields
	Strategy     string  `json:"strategy" gorm:"size:32"`
	ModelID      *uint   `json:"modelId" gorm:"index"`
	SystemPrompt string  `json:"systemPrompt" gorm:"type:text"`
	Temperature  float64 `json:"temperature" gorm:"default:0.7"`
	MaxTokens    int     `json:"maxTokens" gorm:"default:4096"`
	MaxTurns     int     `json:"maxTurns" gorm:"default:10"`

	// coding type fields
	Runtime       string         `json:"runtime" gorm:"size:32"`
	RuntimeConfig model.JSONText `json:"runtimeConfig" gorm:"type:text"`
	ExecMode      string         `json:"execMode" gorm:"size:16"`
	NodeID        *uint          `json:"nodeId" gorm:"index"`
	Workspace     string         `json:"workspace" gorm:"size:512"`

	// common
	Instructions     string         `json:"instructions" gorm:"type:text"`
	SuggestedPrompts model.JSONText `json:"suggestedPrompts" gorm:"type:text"`
}

func (Agent) TableName() string { return "ai_agents" }

type AgentResponse struct {
	ID               uint            `json:"id"`
	Name             string          `json:"name"`
	Code             *string         `json:"code,omitempty"`
	Description      string          `json:"description"`
	Avatar           string          `json:"avatar"`
	Type             string          `json:"type"`
	IsActive         bool            `json:"isActive"`
	Visibility       string          `json:"visibility"`
	CreatedBy        uint            `json:"createdBy"`
	Strategy         string          `json:"strategy,omitempty"`
	ModelID          *uint           `json:"modelId,omitempty"`
	SystemPrompt     string          `json:"systemPrompt,omitempty"`
	Temperature      float64         `json:"temperature"`
	MaxTokens        int             `json:"maxTokens"`
	MaxTurns         int             `json:"maxTurns"`
	Runtime          string          `json:"runtime,omitempty"`
	RuntimeConfig    json.RawMessage `json:"runtimeConfig,omitempty"`
	ExecMode         string          `json:"execMode,omitempty"`
	NodeID           *uint           `json:"nodeId,omitempty"`
	Workspace        string          `json:"workspace,omitempty"`
	Instructions     string          `json:"instructions,omitempty"`
	SuggestedPrompts json.RawMessage `json:"suggestedPrompts,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

func (a *Agent) ToResponse() AgentResponse {
	return AgentResponse{
		ID:               a.ID,
		Name:             a.Name,
		Code:             a.Code,
		Description:      a.Description,
		Avatar:           a.Avatar,
		Type:             a.Type,
		IsActive:         a.IsActive,
		Visibility:       a.Visibility,
		CreatedBy:        a.CreatedBy,
		Strategy:         a.Strategy,
		ModelID:          a.ModelID,
		SystemPrompt:     a.SystemPrompt,
		Temperature:      a.Temperature,
		MaxTokens:        a.MaxTokens,
		MaxTurns:         a.MaxTurns,
		Runtime:          a.Runtime,
		RuntimeConfig:    json.RawMessage(a.RuntimeConfig),
		ExecMode:         a.ExecMode,
		NodeID:           a.NodeID,
		Workspace:        a.Workspace,
		Instructions:     a.Instructions,
		SuggestedPrompts: json.RawMessage(a.SuggestedPrompts),
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}

// --- AgentKnowledgeBase (M2M binding) ---

type AgentKnowledgeBase struct {
	AgentID         uint `json:"agentId" gorm:"primaryKey"`
	KnowledgeBaseID uint `json:"knowledgeBaseId" gorm:"primaryKey"`
}

func (AgentKnowledgeBase) TableName() string { return "ai_agent_knowledge_bases" }

// --- AgentTemplate ---

type AgentTemplate struct {
	model.BaseModel
	Name        string         `json:"name" gorm:"size:128;not null"`
	Description string         `json:"description" gorm:"type:text"`
	Icon        string         `json:"icon" gorm:"size:64"`
	Type        string         `json:"type" gorm:"size:16;not null"`
	Config      model.JSONText `json:"config" gorm:"type:text"`
}

func (AgentTemplate) TableName() string { return "ai_agent_templates" }

type AgentTemplateResponse struct {
	ID          uint            `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Icon        string          `json:"icon"`
	Type        string          `json:"type"`
	Config      json.RawMessage `json:"config"`
	CreatedAt   time.Time       `json:"createdAt"`
}

func (t *AgentTemplate) ToResponse() AgentTemplateResponse {
	cfg := json.RawMessage(t.Config)
	if len(cfg) == 0 {
		cfg = json.RawMessage("{}")
	}
	return AgentTemplateResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Icon:        t.Icon,
		Type:        t.Type,
		Config:      cfg,
		CreatedAt:   t.CreatedAt,
	}
}

// --- AgentSession ---

type AgentSession struct {
	model.BaseModel
	AgentID uint           `json:"agentId" gorm:"not null;index"`
	UserID  uint           `json:"userId" gorm:"not null;index"`
	Status  string         `json:"status" gorm:"size:16;not null;default:running"`
	Title   string         `json:"title" gorm:"size:256"`
	Pinned  bool           `json:"pinned" gorm:"not null;default:false"`
	State   model.JSONText `json:"state" gorm:"type:text"`
}

func (AgentSession) TableName() string { return "ai_agent_sessions" }

type AgentSessionResponse struct {
	ID        uint            `json:"id"`
	AgentID   uint            `json:"agentId"`
	UserID    uint            `json:"userId"`
	Status    string          `json:"status"`
	Title     string          `json:"title"`
	Pinned    bool            `json:"pinned"`
	State     json.RawMessage `json:"state,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

func (s *AgentSession) ToResponse() AgentSessionResponse {
	var state json.RawMessage
	if len(s.State) > 0 {
		state = json.RawMessage(s.State)
	}
	return AgentSessionResponse{
		ID:        s.ID,
		AgentID:   s.AgentID,
		UserID:    s.UserID,
		Status:    s.Status,
		Title:     s.Title,
		Pinned:    s.Pinned,
		State:     state,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

// --- SessionMessage ---

type SessionMessage struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	SessionID  uint           `json:"sessionId" gorm:"not null;index"`
	Role       string         `json:"role" gorm:"size:16;not null"`
	Content    string         `json:"content" gorm:"type:text"`
	Metadata   model.JSONText `json:"metadata" gorm:"type:text"`
	TokenCount int            `json:"tokenCount" gorm:"default:0"`
	Sequence   int            `json:"sequence" gorm:"not null;index"`
	CreatedAt  time.Time      `json:"createdAt"`
}

func (SessionMessage) TableName() string { return "ai_session_messages" }

type SessionMessageResponse struct {
	ID         uint            `json:"id"`
	SessionID  uint            `json:"sessionId"`
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	TokenCount int             `json:"tokenCount"`
	Sequence   int             `json:"sequence"`
	CreatedAt  time.Time       `json:"createdAt"`
}

func (m *SessionMessage) ToResponse() SessionMessageResponse {
	meta := json.RawMessage(m.Metadata)
	if len(meta) == 0 {
		meta = nil
	}
	return SessionMessageResponse{
		ID:         m.ID,
		SessionID:  m.SessionID,
		Role:       m.Role,
		Content:    m.Content,
		Metadata:   meta,
		TokenCount: m.TokenCount,
		Sequence:   m.Sequence,
		CreatedAt:  m.CreatedAt,
	}
}

// --- AgentMemory ---

type AgentMemory struct {
	model.BaseModel
	AgentID uint   `json:"agentId" gorm:"not null;uniqueIndex:idx_agent_user_key"`
	UserID  uint   `json:"userId" gorm:"not null;uniqueIndex:idx_agent_user_key"`
	Key     string `json:"key" gorm:"size:128;not null;uniqueIndex:idx_agent_user_key"`
	Content string `json:"content" gorm:"type:text;not null"`
	Source  string `json:"source" gorm:"size:20;not null;default:agent_generated"`
}

func (AgentMemory) TableName() string { return "ai_agent_memories" }

type AgentMemoryResponse struct {
	ID        uint      `json:"id"`
	AgentID   uint      `json:"agentId"`
	Key       string    `json:"key"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (m *AgentMemory) ToResponse() AgentMemoryResponse {
	return AgentMemoryResponse{
		ID:        m.ID,
		AgentID:   m.AgentID,
		Key:       m.Key,
		Content:   m.Content,
		Source:    m.Source,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
