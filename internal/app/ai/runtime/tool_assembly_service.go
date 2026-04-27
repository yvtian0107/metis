package runtime

import (
	"encoding/json"
	"fmt"

	"github.com/samber/do/v2"

	"metis/internal/pkg/crypto"
)

// ToolAssemblyService assembles the complete tool configuration for an Agent's soul_config.
type ToolAssemblyService struct {
	toolRepo      *AgentToolRepo
	mcpServerRepo *AgentMCPServerRepo
	skillRepo     *AgentSkillRepo
	mcpSvc        *MCPServerService
	skillSvc      *SkillService
	encKey        crypto.EncryptionKey
}

func NewToolAssemblyService(i do.Injector) (*ToolAssemblyService, error) {
	return &ToolAssemblyService{
		toolRepo:      do.MustInvoke[*AgentToolRepo](i),
		mcpServerRepo: do.MustInvoke[*AgentMCPServerRepo](i),
		skillRepo:     do.MustInvoke[*AgentSkillRepo](i),
		mcpSvc:        do.MustInvoke[*MCPServerService](i),
		skillSvc:      do.MustInvoke[*SkillService](i),
		encKey:        do.MustInvoke[crypto.EncryptionKey](i),
	}, nil
}

// AssembledToolConfig is the complete tool configuration included in soul_config.
type AssembledToolConfig struct {
	Tools      []AssembledTool      `json:"tools"`
	MCPServers []AssembledMCPServer `json:"mcpServers"`
	Skills     []AssembledSkill     `json:"skills"`
}

type AssembledTool struct {
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	ParametersSchema json.RawMessage `json:"parametersSchema"`
}

type AssembledMCPServer struct {
	ID        uint            `json:"id"`
	Name      string          `json:"name"`
	Transport string          `json:"transport"`
	URL       string          `json:"url,omitempty"`
	Command   string          `json:"command,omitempty"`
	Args      json.RawMessage `json:"args,omitempty"`
	Env       json.RawMessage `json:"env,omitempty"`
	Auth      json.RawMessage `json:"auth,omitempty"`
}

type AssembledSkill struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	DownloadURL string `json:"downloadUrl"`
	Checksum    string `json:"checksum"`
}

// Assemble builds the complete tool configuration for an agent.
func (s *ToolAssemblyService) Assemble(agentID uint, baseURL string) (*AssembledToolConfig, error) {
	config := &AssembledToolConfig{}

	// 1. Builtin tools
	tools, err := s.toolRepo.ListByAgent(agentID)
	if err != nil {
		return nil, fmt.Errorf("list agent tools: %w", err)
	}
	for _, t := range tools {
		params := json.RawMessage(t.ParametersSchema)
		if len(params) == 0 {
			params = json.RawMessage("{}")
		}
		config.Tools = append(config.Tools, AssembledTool{
			Name:             t.Name,
			Description:      t.Description,
			ParametersSchema: params,
		})
	}

	// 2. MCP servers
	servers, err := s.mcpServerRepo.ListByAgent(agentID)
	if err != nil {
		return nil, fmt.Errorf("list agent mcp servers: %w", err)
	}
	for _, srv := range servers {
		assembled := AssembledMCPServer{
			ID:        srv.ID,
			Name:      srv.Name,
			Transport: srv.Transport,
			URL:       srv.URL,
			Command:   srv.Command,
			Args:      json.RawMessage(srv.Args),
			Env:       json.RawMessage(srv.Env),
		}
		// Decrypt auth config
		if len(srv.AuthConfigEncrypted) > 0 && srv.AuthType != AuthTypeNone {
			plain, err := crypto.Decrypt(srv.AuthConfigEncrypted, s.encKey)
			if err == nil {
				assembled.Auth = json.RawMessage(plain)
			}
		}
		config.MCPServers = append(config.MCPServers, assembled)
	}

	// 3. Skills
	skills, err := s.skillRepo.ListByAgent(agentID)
	if err != nil {
		return nil, fmt.Errorf("list agent skills: %w", err)
	}
	for _, skill := range skills {
		config.Skills = append(config.Skills, AssembledSkill{
			ID:          skill.ID,
			Name:        skill.Name,
			DownloadURL: fmt.Sprintf("%s/api/v1/ai/internal/skills/%d/package", baseURL, skill.ID),
			Checksum:    s.skillSvc.Checksum(&skill),
		})
	}

	return config, nil
}
