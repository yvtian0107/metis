package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode"
)

const knowledgeRecallLimit = 5

type assistantRuntimeAssembly struct {
	SystemPrompt   string
	Tools          []ToolDefinition
	ToolRegistries []ToolHandlerRegistry
}

func (gw *AgentGateway) buildAssistantRuntime(ctx context.Context, agent *Agent, session *AgentSession, messages []ExecuteMessage, baseSystemPrompt string) (*assistantRuntimeAssembly, error) {
	assembly := &assistantRuntimeAssembly{
		SystemPrompt:   baseSystemPrompt,
		ToolRegistries: append([]ToolHandlerRegistry(nil), gw.toolRegistries...),
	}
	seenToolNames := map[string]struct{}{}

	if err := gw.addBuiltinRuntimeTools(agent.ID, assembly, seenToolNames); err != nil {
		return nil, err
	}
	if err := gw.addSkillRuntime(agent.ID, assembly, seenToolNames); err != nil {
		return nil, err
	}
	if err := gw.addMCPRuntimeTools(ctx, agent.ID, assembly, seenToolNames); err != nil {
		return nil, err
	}

	runtimeContext := gw.buildAgentRuntimeContext(ctx, agent, session)
	assembly.SystemPrompt = appendPromptBlock(assembly.SystemPrompt, runtimeContext)

	knowledgeBlock := gw.buildKnowledgeContext(ctx, agent.ID, latestUserMessage(messages))
	assembly.SystemPrompt = appendPromptBlock(assembly.SystemPrompt, knowledgeBlock)

	_ = session
	return assembly, nil
}

func (gw *AgentGateway) buildAgentRuntimeContext(ctx context.Context, agent *Agent, session *AgentSession) string {
	if agent == nil || session == nil || len(gw.runtimeContextProviders) == 0 {
		return ""
	}
	var agentCode string
	if agent.Code != nil {
		agentCode = strings.TrimSpace(*agent.Code)
	}
	var blocks []string
	for _, provider := range gw.runtimeContextProviders {
		block, err := provider.BuildAgentRuntimeContext(ctx, agentCode, session.ID, session.UserID)
		if err != nil {
			slog.Warn("agent runtime context provider failed", "agent", agentCode, "sessionID", session.ID, "error", err)
			continue
		}
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		blocks = append(blocks, block)
	}
	return strings.Join(blocks, "\n\n")
}

func (gw *AgentGateway) addBuiltinRuntimeTools(agentID uint, assembly *assistantRuntimeAssembly, seen map[string]struct{}) error {
	toolIDs, err := gw.agentRepo.GetToolIDs(agentID)
	if err != nil {
		return err
	}
	for _, tid := range toolIDs {
		var tool Tool
		if err := gw.agentRepo.db.First(&tool, tid).Error; err != nil {
			slog.Warn("tool not found for binding", "toolID", tid)
			continue
		}
		if !tool.IsActive {
			continue
		}
		hasHandler := hasToolHandler(gw.toolRegistries, tool.Name)
		if !hasHandler {
			slog.Warn("skipping builtin tool without executable handler", "tool", tool.Name)
			continue
		}
		if availability := classifyToolAvailability(tool, hasHandler); !availability.IsExecutable {
			slog.Warn("skipping builtin tool unavailable at runtime", "tool", tool.Name, "status", availability.Status)
			continue
		}
		params := json.RawMessage(tool.ParametersSchema)
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		if err := addRuntimeToolDefinition(&assembly.Tools, seen, ToolDefinition{
			Type:        "builtin",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  params,
			SourceID:    tool.ID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (gw *AgentGateway) addSkillRuntime(agentID uint, assembly *assistantRuntimeAssembly, seen map[string]struct{}) error {
	skillIDs, err := gw.agentRepo.GetSkillIDs(agentID)
	if err != nil {
		return err
	}
	registry := NewSkillToolRegistry()
	for _, sid := range skillIDs {
		var skill Skill
		if err := gw.agentRepo.db.First(&skill, sid).Error; err != nil {
			slog.Warn("skill not found for binding", "skillID", sid)
			continue
		}
		if !skill.IsActive {
			continue
		}
		if strings.TrimSpace(skill.Instructions) != "" {
			assembly.SystemPrompt = appendPromptBlock(assembly.SystemPrompt, fmt.Sprintf("## Skill: %s\n%s", displaySkillName(skill), strings.TrimSpace(skill.Instructions)))
		}
		for _, spec := range parseSkillTools(skill) {
			if strings.TrimSpace(spec.Name) == "" {
				continue
			}
			endpoint := endpointFromSkillToolSpec(spec)
			if endpoint.URL == "" {
				slog.Warn("skipping skill tool without supported endpoint contract", "skillID", skill.ID, "tool", spec.Name)
				continue
			}
			exposedName := "skill__" + sanitizeToolName(skill.Name) + "__" + sanitizeToolName(spec.Name)
			params := skillToolParameters(spec)
			if !validJSONSchema(params) {
				slog.Warn("skipping skill tool with invalid parameters schema", "skillID", skill.ID, "tool", spec.Name)
				continue
			}
			if err := addRuntimeToolDefinition(&assembly.Tools, seen, ToolDefinition{
				Type:        "skill",
				Name:        exposedName,
				Description: spec.Description,
				Parameters:  params,
				SourceID:    skill.ID,
			}); err != nil {
				return err
			}
			registry.Register(exposedName, skill, spec)
		}
	}
	if registry.Len() > 0 {
		assembly.ToolRegistries = append(assembly.ToolRegistries, registry)
	}
	return nil
}

func (gw *AgentGateway) addMCPRuntimeTools(ctx context.Context, agentID uint, assembly *assistantRuntimeAssembly, seen map[string]struct{}) error {
	mcpIDs, err := gw.agentRepo.GetMCPServerIDs(agentID)
	if err != nil {
		return err
	}
	if len(mcpIDs) == 0 || gw.mcpClient == nil {
		return nil
	}
	registry := NewMCPToolRegistry(gw.mcpClient)
	for _, mid := range mcpIDs {
		var server MCPServer
		if err := gw.agentRepo.db.First(&server, mid).Error; err != nil {
			slog.Warn("MCP server not found for binding", "mcpServerID", mid)
			continue
		}
		if !server.IsActive {
			continue
		}
		tools, err := gw.mcpClient.DiscoverTools(ctx, server)
		if err != nil {
			slog.Warn("MCP discovery failed", "mcpServerID", server.ID, "name", server.Name, "error", err)
			continue
		}
		for _, tool := range tools {
			exposedName := "mcp__" + sanitizeToolName(server.Name) + "__" + sanitizeToolName(tool.Name)
			params := tool.Parameters
			if len(params) == 0 {
				params = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			if !validJSONSchema(params) {
				slog.Warn("skipping MCP tool with invalid parameters schema", "mcpServerID", server.ID, "tool", tool.Name)
				continue
			}
			if err := addRuntimeToolDefinition(&assembly.Tools, seen, ToolDefinition{
				Type:        "mcp",
				Name:        exposedName,
				Description: tool.Description,
				Parameters:  params,
				SourceID:    server.ID,
			}); err != nil {
				return err
			}
			registry.Register(exposedName, server, tool.Name)
		}
	}
	if registry.Len() > 0 {
		assembly.ToolRegistries = append(assembly.ToolRegistries, registry)
	}
	return nil
}

func (gw *AgentGateway) buildKnowledgeContext(ctx context.Context, agentID uint, query string) string {
	if gw.knowledgeSearcher == nil || strings.TrimSpace(query) == "" {
		return ""
	}
	kbIDs, err := gw.agentRepo.GetKnowledgeBaseIDs(agentID)
	if err != nil {
		slog.Warn("failed to load agent knowledge base bindings", "agentID", agentID, "error", err)
		return ""
	}
	kgIDs, err := gw.agentRepo.GetKnowledgeGraphIDs(agentID)
	if err != nil {
		slog.Warn("failed to load agent knowledge graph bindings", "agentID", agentID, "error", err)
		return ""
	}
	assetIDs := uniqueUintSlice(append(kbIDs, kgIDs...))
	if len(assetIDs) == 0 {
		return ""
	}
	results, err := gw.knowledgeSearcher.SearchKnowledgeWithContext(ctx, assetIDs, query, knowledgeRecallLimit)
	if err != nil {
		slog.Warn("knowledge recall failed", "agentID", agentID, "error", err)
		return ""
	}
	if len(results) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Knowledge Context\n")
	sb.WriteString("Use the following retrieved knowledge as grounding when relevant:\n")
	for i, result := range results {
		content := strings.TrimSpace(result.Content)
		if len(content) > 1200 {
			content = content[:1200] + "..."
		}
		title := strings.TrimSpace(result.Title)
		if title == "" {
			title = "Knowledge"
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n%s\n", i+1, title, content))
	}
	return sb.String()
}

func addRuntimeToolDefinition(defs *[]ToolDefinition, seen map[string]struct{}, def ToolDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("runtime tool name is required")
	}
	if _, exists := seen[def.Name]; exists {
		return fmt.Errorf("duplicate runtime tool name: %s", def.Name)
	}
	seen[def.Name] = struct{}{}
	*defs = append(*defs, def)
	return nil
}

func hasToolHandler(registries []ToolHandlerRegistry, name string) bool {
	for _, reg := range registries {
		if reg.HasTool(name) {
			return true
		}
	}
	return false
}

func latestUserMessage(messages []ExecuteMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == MessageRoleUser || messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func appendPromptBlock(prompt, block string) string {
	block = strings.TrimSpace(block)
	if block == "" {
		return prompt
	}
	if strings.TrimSpace(prompt) == "" {
		return block
	}
	return strings.TrimSpace(prompt) + "\n\n" + block
}

func sanitizeToolName(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	lastUnderscore := false
	for _, r := range name {
		valid := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
		if valid {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_-")
	if out == "" {
		out = "tool"
	}
	first := []rune(out)[0]
	if unicode.IsDigit(first) {
		out = "tool_" + out
	}
	return out
}

func validJSONSchema(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var parsed any
	return json.Unmarshal(raw, &parsed) == nil
}

func displaySkillName(skill Skill) string {
	if skill.DisplayName != "" {
		return skill.DisplayName
	}
	return skill.Name
}
