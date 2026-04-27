package runtime

import (
	"encoding/json"
	"time"

	"metis/internal/model"
)

// Tool types
const (
	ToolTypeBuiltin = "builtin"
)

const (
	ToolAvailabilityAvailable     = "available"
	ToolAvailabilityInactive      = "inactive"
	ToolAvailabilityNeedsConfig   = "needs_config"
	ToolAvailabilityUnimplemented = "unimplemented"
	ToolAvailabilityRiskDisabled  = "risk_disabled"
)

// MCP transport types
const (
	MCPTransportSSE   = "sse"
	MCPTransportSTDIO = "stdio"
)

// MCP auth types
const (
	AuthTypeNone         = "none"
	AuthTypeAPIKey       = "api_key"
	AuthTypeBearer       = "bearer"
	AuthTypeOAuth        = "oauth"
	AuthTypeCustomHeader = "custom_header"
)

// Skill source types
const (
	SkillSourceGitHub = "github"
	SkillSourceUpload = "upload"
)

// --- Tool (builtin) ---

type Tool struct {
	model.BaseModel
	Toolkit             string         `json:"toolkit" gorm:"size:64;not null;default:'';index"`
	Name                string         `json:"name" gorm:"size:64;uniqueIndex;not null"`
	DisplayName         string         `json:"displayName" gorm:"size:128;not null"`
	Description         string         `json:"description" gorm:"type:text"`
	ParametersSchema    model.JSONText `json:"parametersSchema" gorm:"type:text"`
	RuntimeConfigSchema model.JSONText `json:"runtimeConfigSchema" gorm:"type:text"`
	RuntimeConfig       model.JSONText `json:"runtimeConfig" gorm:"type:text"`
	IsActive            bool           `json:"isActive" gorm:"not null;default:true"`
}

func (Tool) TableName() string { return "ai_tools" }

type ToolResponse struct {
	ID                  uint            `json:"id"`
	Toolkit             string          `json:"toolkit"`
	Name                string          `json:"name"`
	DisplayName         string          `json:"displayName"`
	Description         string          `json:"description"`
	ParametersSchema    json.RawMessage `json:"parametersSchema"`
	RuntimeConfigSchema json.RawMessage `json:"runtimeConfigSchema,omitempty"`
	RuntimeConfig       json.RawMessage `json:"runtimeConfig,omitempty"`
	IsActive            bool            `json:"isActive"`
	IsExecutable        bool            `json:"isExecutable"`
	AvailabilityStatus  string          `json:"availabilityStatus"`
	AvailabilityReason  string          `json:"availabilityReason,omitempty"`
	BoundAgentCount     int64           `json:"boundAgentCount"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}

func (t *Tool) ToResponse() ToolResponse {
	params := json.RawMessage(t.ParametersSchema)
	if len(params) == 0 {
		params = json.RawMessage("{}")
	}
	runtimeSchema := json.RawMessage(t.RuntimeConfigSchema)
	if len(runtimeSchema) == 0 {
		runtimeSchema = nil
	}
	runtimeConfig := json.RawMessage(t.RuntimeConfig)
	if len(runtimeConfig) == 0 {
		runtimeConfig = nil
	}
	return ToolResponse{
		ID:                  t.ID,
		Toolkit:             t.Toolkit,
		Name:                t.Name,
		DisplayName:         t.DisplayName,
		Description:         t.Description,
		ParametersSchema:    params,
		RuntimeConfigSchema: runtimeSchema,
		RuntimeConfig:       runtimeConfig,
		IsActive:            t.IsActive,
		IsExecutable:        t.IsActive,
		AvailabilityStatus:  ToolAvailabilityAvailable,
		CreatedAt:           t.CreatedAt,
		UpdatedAt:           t.UpdatedAt,
	}
}

// --- MCPServer ---

type MCPServer struct {
	model.BaseModel
	Name                string         `json:"name" gorm:"size:128;not null"`
	Description         string         `json:"description" gorm:"type:text"`
	Transport           string         `json:"transport" gorm:"size:16;not null"` // sse | stdio
	URL                 string         `json:"url" gorm:"size:512"`               // SSE endpoint
	Command             string         `json:"command" gorm:"size:256"`           // STDIO command
	Args                model.JSONText `json:"args" gorm:"type:text"`             // STDIO args
	Env                 model.JSONText `json:"env" gorm:"type:text"`              // STDIO env vars
	AuthType            string         `json:"authType" gorm:"size:32;not null;default:none"`
	AuthConfigEncrypted []byte         `json:"-" gorm:"column:auth_config_encrypted;type:bytes"`
	IsActive            bool           `json:"isActive" gorm:"not null;default:true"`
}

func (MCPServer) TableName() string { return "ai_mcp_servers" }

type MCPServerResponse struct {
	ID          uint            `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Transport   string          `json:"transport"`
	URL         string          `json:"url,omitempty"`
	Command     string          `json:"command,omitempty"`
	Args        json.RawMessage `json:"args,omitempty"`
	Env         json.RawMessage `json:"env,omitempty"`
	AuthType    string          `json:"authType"`
	AuthMasked  string          `json:"authMasked,omitempty"`
	IsActive    bool            `json:"isActive"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

func (m *MCPServer) ToResponse(authMasked string) MCPServerResponse {
	return MCPServerResponse{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Transport:   m.Transport,
		URL:         m.URL,
		Command:     m.Command,
		Args:        json.RawMessage(m.Args),
		Env:         json.RawMessage(m.Env),
		AuthType:    m.AuthType,
		AuthMasked:  authMasked,
		IsActive:    m.IsActive,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// --- Skill ---

type Skill struct {
	model.BaseModel
	Name                string         `json:"name" gorm:"size:128;not null"`
	DisplayName         string         `json:"displayName" gorm:"size:128;not null"`
	Description         string         `json:"description" gorm:"type:text"`
	SourceType          string         `json:"sourceType" gorm:"size:16;not null"` // github | upload
	SourceURL           string         `json:"sourceUrl" gorm:"size:512"`
	Manifest            model.JSONText `json:"manifest" gorm:"type:text"`
	Instructions        string         `json:"instructions" gorm:"type:text"`
	ToolsSchema         model.JSONText `json:"toolsSchema" gorm:"type:text"`
	AuthType            string         `json:"authType" gorm:"size:32;not null;default:none"`
	AuthConfigEncrypted []byte         `json:"-" gorm:"column:auth_config_encrypted;type:bytes"`
	IsActive            bool           `json:"isActive" gorm:"not null;default:true"`
}

func (Skill) TableName() string { return "ai_skills" }

type SkillResponse struct {
	ID              uint            `json:"id"`
	Name            string          `json:"name"`
	DisplayName     string          `json:"displayName"`
	Description     string          `json:"description"`
	SourceType      string          `json:"sourceType"`
	SourceURL       string          `json:"sourceUrl,omitempty"`
	Manifest        json.RawMessage `json:"manifest,omitempty"`
	Instructions    string          `json:"instructions,omitempty"`
	ToolsSchema     json.RawMessage `json:"toolsSchema,omitempty"`
	ToolCount       int             `json:"toolCount"`
	HasInstructions bool            `json:"hasInstructions"`
	AuthType        string          `json:"authType"`
	IsActive        bool            `json:"isActive"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

func (s *Skill) ToResponse() SkillResponse {
	toolCount := 0
	if len(s.ToolsSchema) > 0 {
		var tools []json.RawMessage
		if json.Unmarshal([]byte(s.ToolsSchema), &tools) == nil {
			toolCount = len(tools)
		}
	}
	return SkillResponse{
		ID:              s.ID,
		Name:            s.Name,
		DisplayName:     s.DisplayName,
		Description:     s.Description,
		SourceType:      s.SourceType,
		SourceURL:       s.SourceURL,
		Manifest:        json.RawMessage(s.Manifest),
		Instructions:    s.Instructions,
		ToolsSchema:     json.RawMessage(s.ToolsSchema),
		ToolCount:       toolCount,
		HasInstructions: s.Instructions != "",
		AuthType:        s.AuthType,
		IsActive:        s.IsActive,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

// --- Binding tables ---

type AgentTool struct {
	AgentID uint `json:"agentId" gorm:"primaryKey"`
	ToolID  uint `json:"toolId" gorm:"primaryKey"`
}

func (AgentTool) TableName() string { return "ai_agent_tools" }

type AgentMCPServer struct {
	AgentID     uint `json:"agentId" gorm:"primaryKey"`
	MCPServerID uint `json:"mcpServerId" gorm:"primaryKey;column:mcp_server_id"`
}

func (AgentMCPServer) TableName() string { return "ai_agent_mcp_servers" }

type AgentSkill struct {
	AgentID uint `json:"agentId" gorm:"primaryKey"`
	SkillID uint `json:"skillId" gorm:"primaryKey"`
}

func (AgentSkill) TableName() string { return "ai_agent_skills" }
