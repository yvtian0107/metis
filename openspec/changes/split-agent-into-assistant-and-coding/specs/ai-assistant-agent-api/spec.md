## ADDED Requirements

### Requirement: Assistant agent typed API routes
The system SHALL provide REST endpoints under `/api/v1/ai/assistant-agents` with JWT + Casbin auth. All operations SHALL enforce `type=assistant` without accepting type from the client.

- `POST /` — create assistant agent (type forced to `assistant`)
- `GET /` — list assistant agents (with pagination, keyword search, visibility filter)
- `GET /:id` — get assistant agent detail (returns 404 if agent is not type `assistant`)
- `PUT /:id` — update assistant agent (returns 404 if agent is not type `assistant`)
- `DELETE /:id` — delete assistant agent (returns 404 if agent is not type `assistant`)
- `GET /templates` — list templates filtered to `type=assistant`

#### Scenario: Create assistant agent via typed route
- **WHEN** admin sends `POST /api/v1/ai/assistant-agents` with name, modelId, strategy
- **THEN** system SHALL create the agent with `type=assistant` regardless of any `type` field in the request body

#### Scenario: List only assistant agents
- **WHEN** user sends `GET /api/v1/ai/assistant-agents`
- **THEN** system SHALL return only agents with `type=assistant`, excluding `coding` and `internal` types

#### Scenario: Access coding agent via assistant route
- **WHEN** user sends `GET /api/v1/ai/assistant-agents/:id` where the agent has `type=coding`
- **THEN** system SHALL return 404

#### Scenario: Update coding agent via assistant route
- **WHEN** user sends `PUT /api/v1/ai/assistant-agents/:id` where the agent has `type=coding`
- **THEN** system SHALL return 404

#### Scenario: List assistant templates
- **WHEN** user sends `GET /api/v1/ai/assistant-agents/templates`
- **THEN** system SHALL return only templates with `type=assistant`

### Requirement: Assistant agent typed permissions
The system SHALL define the following Casbin permissions for assistant agent operations:

- `ai:assistant-agent:list` — view assistant agents list and menu
- `ai:assistant-agent:create` — create assistant agents
- `ai:assistant-agent:update` — edit assistant agents
- `ai:assistant-agent:delete` — delete assistant agents

These permissions SHALL be independent from coding agent permissions.

#### Scenario: User with only assistant permission
- **WHEN** a user has `ai:assistant-agent:list` but not `ai:coding-agent:list`
- **THEN** the user SHALL see the assistant agents menu but NOT the coding agents menu

#### Scenario: Permission seed on upgrade
- **WHEN** the system starts after upgrading from the unified `ai:agent:*` permission model
- **THEN** the seed SHALL detect roles with old `ai:agent:*` permissions and grant them both `ai:assistant-agent:*` and `ai:coding-agent:*` equivalents before removing the old permissions

### Requirement: Type consistency enforcement
The system SHALL provide a service-level type consistency check that ensures an agent accessed through a typed route matches the expected type. If the agent exists but has a different type, the system SHALL return the same error as agent-not-found (404), consistent with the existing not-found-over-403 strategy.

#### Scenario: Type mismatch returns not found
- **WHEN** a coding agent is accessed via `/api/v1/ai/assistant-agents/:id`
- **THEN** system SHALL return 404 with "agent not found" message

#### Scenario: Type match succeeds
- **WHEN** an assistant agent is accessed via `/api/v1/ai/assistant-agents/:id`
- **THEN** system SHALL proceed with normal access checks (visibility, ownership)
