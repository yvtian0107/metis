## MODIFIED Requirements

### Requirement: Assistant agent tool binding
An assistant-type Agent SHALL support binding to: Tool IDs (M2M via ai_agent_tools), Skill IDs (M2M via ai_agent_skills), MCP Server IDs (M2M via ai_agent_mcp_servers), Knowledge Base IDs (M2M via ai_agent_knowledge_bases), Knowledge Graph IDs (M2M via ai_agent_knowledge_graphs). All bindings are optional.

#### Scenario: Bind tools to agent
- **WHEN** admin updates an assistant agent with tool_ids, skill_ids, mcp_server_ids, knowledge_base_ids, and knowledge_graph_ids
- **THEN** system SHALL replace the existing bindings with the new set

#### Scenario: Bound resource deleted
- **WHEN** a Tool/Skill/MCP Server/Knowledge Base/Knowledge Graph bound to an agent is deleted
- **THEN** the binding record SHALL be cascade-deleted

#### Scenario: Agent knowledge search at runtime
- **WHEN** an agent with bound knowledge bases and knowledge graphs receives a knowledge search request
- **THEN** system SHALL query all bound assets via their respective engines and merge results

### Requirement: Common agent fields
Both agent types SHALL support an `instructions` text field for injecting contextual guidance. Both types SHALL support `knowledge_base_ids` and `knowledge_graph_ids` bindings for knowledge context injection.

#### Scenario: Instructions on assistant agent
- **WHEN** an assistant agent has instructions set
- **THEN** instructions SHALL be appended to the system prompt during execution

#### Scenario: Instructions on coding agent
- **WHEN** a coding agent has instructions set
- **THEN** instructions SHALL be injected into the coding tool's instruction mechanism (e.g., CLAUDE.md for claude-code)
