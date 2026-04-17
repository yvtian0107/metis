## ADDED Requirements

### Requirement: Agent service test infrastructure
The system SHALL provide a test harness for `AgentService` using an in-memory SQLite database with `ai_agents`, `ai_agent_tools`, `ai_agent_mcp_servers`, `ai_agent_skills`, and `ai_agent_knowledge_bases` tables.

#### Scenario: Setup test database
- **WHEN** an agent service test initializes
- **THEN** it SHALL migrate agent and binding tables into a shared-memory SQLite database

### Requirement: Test agent creation validation
The service-layer test suite SHALL verify that agent creation enforces type-specific constraints and uniqueness.

#### Scenario: Reject invalid agent type
- **WHEN** `Create` is called with type="unknown"
- **THEN** it returns `ErrInvalidAgentType`

#### Scenario: Reject assistant agent without model
- **WHEN** `Create` is called with type="assistant" and no `ModelID`
- **THEN** it returns `ErrModelRequired`

#### Scenario: Reject coding agent without runtime
- **WHEN** `Create` is called with type="coding" and an empty `Runtime`
- **THEN** it returns `ErrRuntimeRequired`

#### Scenario: Reject remote coding agent without node
- **WHEN** `Create` is called with type="coding", execMode="remote", and no `NodeID`
- **THEN** it returns `ErrNodeRequired`

#### Scenario: Reject internal agent without code
- **WHEN** `Create` is called with type="internal" and no `Code`
- **THEN** it returns `ErrCodeRequired`

#### Scenario: Reject duplicate agent name
- **WHEN** `Create` is called twice with the same name
- **THEN** the second call returns `ErrAgentNameConflict`

#### Scenario: Reject duplicate internal agent code
- **WHEN** `Create` is called twice with type="internal" and the same code
- **THEN** the second call returns `ErrAgentCodeConflict`

### Requirement: Test agent update and deletion
The service-layer test suite SHALL verify update validation and safe deletion.

#### Scenario: Update agent fields
- **WHEN** `Update` is called with a new description and temperature
- **THEN** the persisted agent reflects the changes

#### Scenario: Delete agent with running sessions fails
- **WHEN** `Delete` is called for an agent that has a session with status="running"
- **THEN** it returns `ErrAgentHasRunningSessions`

#### Scenario: Delete agent without running sessions succeeds
- **WHEN** `Delete` is called for an agent with no running sessions
- **THEN** the agent is removed

### Requirement: Test agent binding management
The service-layer test suite SHALL verify atomic replacement of tool, MCP, skill, and knowledge base bindings.

#### Scenario: Replace all bindings
- **WHEN** `UpdateBindings` is called with toolIDs=[1,2], skillIDs=[3], mcpIDs=[4], kbIDs=[5]
- **THEN** the agent has exactly those bindings and no others

#### Scenario: Get bindings returns IDs
- **WHEN** `GetBindings` is called after setting bindings
- **THEN** it returns the exact toolIDs, skillIDs, mcpIDs, and kbIDs

### Requirement: Test agent templates
The service-layer test suite SHALL verify template listing.

#### Scenario: List agent templates
- **WHEN** `ListTemplates` is called after seeding templates
- **THEN** it returns all seeded `AgentTemplate` records
