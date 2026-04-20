## REMOVED Requirements

### Requirement: Agent list page
**Reason**: Replaced by separate `ai-assistant-agent-ui` and `ai-coding-agent-ui` capabilities. The unified `/ai/agents` page that mixed both agent types is no longer needed.
**Migration**: Use `/ai/assistant-agents` for assistant agents and `/ai/coding-agents` for coding agents.

### Requirement: Agent creation wizard
**Reason**: Replaced by type-specific creation pages in `ai-assistant-agent-ui` and `ai-coding-agent-ui`. The multi-step wizard with type selection step is replaced by direct creation pages that know their type from the route.
**Migration**: Use `/ai/assistant-agents/create` or `/ai/coding-agents/create`.

### Requirement: Agent edit page
**Reason**: Replaced by type-specific edit pages in `ai-assistant-agent-ui` and `ai-coding-agent-ui`.
**Migration**: Use `/ai/assistant-agents/:id/edit` or `/ai/coding-agents/:id/edit`.

### Requirement: Menu and permission seeding
**Reason**: The unified "Agents" menu and `ai:agent:*` permissions are replaced by two separate menus and permission sets, defined in the `ai-management-navigation` delta spec and `ai-assistant-agent-api` / `ai-coding-agent-api` specs.
**Migration**: New menu items `助手智能体` and `编码智能体` are seeded under the `智能体` group. Old `ai:agent:list` menu is soft-deleted.
