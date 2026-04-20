## MODIFIED Requirements

### Requirement: Agent CRUD API
The system SHALL retain REST endpoints under `/api/v1/ai/agents` for internal use only (system agent lookup by code, internal agent management). The endpoints SHALL no longer be exposed to end-users through Casbin policies. User-facing CRUD operations SHALL be performed through typed routes (`/api/v1/ai/assistant-agents` and `/api/v1/ai/coding-agents`).

The endpoint behavior remains unchanged: `POST /`, `GET /`, `GET /:id`, `PUT /:id`, `DELETE /:id` continue to work with full type support. However, seed SHALL remove the public Casbin policies for these routes, making them accessible only to internal callers or superadmins with explicit policy grants.

#### Scenario: Internal agent lookup still works
- **WHEN** an internal module calls `GET /api/v1/ai/agents?type=internal`
- **THEN** system SHALL return internal-type agents as before

#### Scenario: Default agent listing excludes internal
- **WHEN** a caller requests `GET /api/v1/ai/agents` without type filter
- **THEN** system SHALL return only `assistant` and `coding` type agents, excluding `internal` type

#### Scenario: Regular admin cannot access generic route after upgrade
- **WHEN** a regular admin (without explicit superadmin policy) calls `GET /api/v1/ai/agents` after the permission migration
- **THEN** Casbin SHALL deny the request since the old `ai:agent:*` policies have been removed

### Requirement: Agent templates
The system SHALL provide seed-based agent templates. Each template SHALL pre-fill agent configuration (type, strategy, system_prompt, suggested tool bindings). Templates are read-only reference data, creating from a template copies the config into a new agent. The template listing endpoint SHALL support filtering by `type` query parameter.

#### Scenario: List templates filtered by type
- **WHEN** user requests `GET /api/v1/ai/assistant-agents/templates`
- **THEN** system SHALL return only templates with `type=assistant`

#### Scenario: List templates without filter via legacy route
- **WHEN** caller requests `GET /api/v1/ai/agents/templates` without type filter
- **THEN** system SHALL return all templates (backward compatible)

#### Scenario: Expanded coding templates
- **WHEN** user requests `GET /api/v1/ai/coding-agents/templates`
- **THEN** system SHALL return coding templates for each supported runtime (Claude Code, OpenCode, Codex, Aider)

## ADDED Requirements

### Requirement: Service-level type enforcement
The AgentService SHALL provide an `EnsureType(agent, expectedType)` method that returns `ErrAgentNotFound` if the agent's type does not match the expected type. This method SHALL be called by typed handlers before proceeding with any Get, Update, or Delete operation.

#### Scenario: EnsureType match
- **WHEN** `EnsureType` is called with an assistant agent and expectedType `assistant`
- **THEN** the method SHALL return nil (success)

#### Scenario: EnsureType mismatch
- **WHEN** `EnsureType` is called with a coding agent and expectedType `assistant`
- **THEN** the method SHALL return `ErrAgentNotFound`

### Requirement: Permission migration on upgrade
The seed function SHALL detect existing roles with old `ai:agent:*` permissions and automatically grant the equivalent new permissions (`ai:assistant-agent:*` and `ai:coding-agent:*`) before removing the old permissions. This ensures no admin loses access during upgrade.

#### Scenario: Upgrade from unified permissions
- **WHEN** the system starts and detects roles with `ai:agent:list` permission
- **THEN** seed SHALL grant those roles `ai:assistant-agent:list` and `ai:coding-agent:list`
- **AND** seed SHALL grant equivalent create/update/delete permissions
- **AND** seed SHALL then remove the old `ai:agent:*` permissions

#### Scenario: Fresh install
- **WHEN** the system runs seed for the first time (no old permissions exist)
- **THEN** seed SHALL create only the new `ai:assistant-agent:*` and `ai:coding-agent:*` permissions
