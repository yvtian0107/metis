## ADDED Requirements

### Requirement: Coding agent typed API routes
The system SHALL provide REST endpoints under `/api/v1/ai/coding-agents` with JWT + Casbin auth. All operations SHALL enforce `type=coding` without accepting type from the client.

- `POST /` — create coding agent (type forced to `coding`)
- `GET /` — list coding agents (with pagination, keyword search, visibility filter)
- `GET /:id` — get coding agent detail (returns 404 if agent is not type `coding`)
- `PUT /:id` — update coding agent (returns 404 if agent is not type `coding`)
- `DELETE /:id` — delete coding agent (returns 404 if agent is not type `coding`)
- `GET /templates` — list templates filtered to `type=coding`

#### Scenario: Create coding agent via typed route
- **WHEN** admin sends `POST /api/v1/ai/coding-agents` with name, runtime, execMode
- **THEN** system SHALL create the agent with `type=coding` regardless of any `type` field in the request body

#### Scenario: List only coding agents
- **WHEN** user sends `GET /api/v1/ai/coding-agents`
- **THEN** system SHALL return only agents with `type=coding`, excluding `assistant` and `internal` types

#### Scenario: Access assistant agent via coding route
- **WHEN** user sends `GET /api/v1/ai/coding-agents/:id` where the agent has `type=assistant`
- **THEN** system SHALL return 404

#### Scenario: Update assistant agent via coding route
- **WHEN** user sends `PUT /api/v1/ai/coding-agents/:id` where the agent has `type=assistant`
- **THEN** system SHALL return 404

#### Scenario: List coding templates
- **WHEN** user sends `GET /api/v1/ai/coding-agents/templates`
- **THEN** system SHALL return only templates with `type=coding`

### Requirement: Coding agent typed permissions
The system SHALL define the following Casbin permissions for coding agent operations:

- `ai:coding-agent:list` — view coding agents list and menu
- `ai:coding-agent:create` — create coding agents
- `ai:coding-agent:update` — edit coding agents
- `ai:coding-agent:delete` — delete coding agents

These permissions SHALL be independent from assistant agent permissions, allowing organizations to restrict coding agent management to a smaller set of administrators.

#### Scenario: User with only coding permission
- **WHEN** a user has `ai:coding-agent:list` but not `ai:assistant-agent:list`
- **THEN** the user SHALL see the coding agents menu but NOT the assistant agents menu

#### Scenario: Coding permission independence
- **WHEN** an admin revokes `ai:coding-agent:create` from a role
- **THEN** users with that role SHALL still be able to create assistant agents if they have `ai:assistant-agent:create`
