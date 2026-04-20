## MODIFIED Requirements

### Requirement: Agent CRUD API
The system SHALL provide REST endpoints under `/api/v1/ai/agents` with JWT + Casbin auth:
- `POST /` — create agent
- `GET /` — list agents (with pagination, keyword search, type filter, visibility filter)
- `GET /:id` — get agent detail
- `PUT /:id` — update agent
- `DELETE /:id` — soft-delete agent (blocked if active sessions exist)

Internal agents SHALL be excluded from the default list response unless explicitly requested via `type=internal` filter.

Detail, update, and delete operations SHALL enforce record-level visibility and ownership rules before returning or mutating an agent. Requests for an agent that is not visible to the current user SHALL behave the same as a missing agent.

#### Scenario: List agents with type filter
- **WHEN** user requests `GET /api/v1/ai/agents?type=assistant`
- **THEN** system SHALL return only assistant-type agents visible to the user

#### Scenario: Default agent listing excludes internal
- **WHEN** user requests `GET /api/v1/ai/agents` without type filter
- **THEN** system SHALL return only `assistant` and `coding` type agents, excluding `internal` type

#### Scenario: Explicit internal agent listing
- **WHEN** user requests `GET /api/v1/ai/agents?type=internal`
- **THEN** system SHALL return internal-type agents

#### Scenario: Delete agent with active sessions
- **WHEN** admin deletes an agent that has sessions with status `running`
- **THEN** system SHALL return a 409 error

#### Scenario: Hidden agent detail behaves as not found
- **WHEN** a user requests `GET /api/v1/ai/agents/:id` for an agent that is not visible to them
- **THEN** system SHALL return the same not-found result used for a missing agent

#### Scenario: Non-owner cannot modify private agent
- **WHEN** a non-creator user requests `PUT /api/v1/ai/agents/:id` for a private agent
- **THEN** system SHALL return the same not-found result used for a missing agent

#### Scenario: Non-owner cannot delete private agent
- **WHEN** a non-creator user requests `DELETE /api/v1/ai/agents/:id` for a private agent
- **THEN** system SHALL return the same not-found result used for a missing agent

## ADDED Requirements

### Requirement: Agent visibility enforcement on session creation
Creating a session for an agent SHALL enforce the same visibility rules as agent detail access. A user SHALL only be able to create a session for an agent that is visible to them.

#### Scenario: Create session for own private agent
- **WHEN** the creator requests `POST /api/v1/ai/sessions` for their private agent
- **THEN** system SHALL create the session successfully

#### Scenario: Create session for team agent
- **WHEN** an authenticated user requests `POST /api/v1/ai/sessions` for a team-visible agent
- **THEN** system SHALL create the session successfully

#### Scenario: Create session for another user's private agent
- **WHEN** a non-creator user requests `POST /api/v1/ai/sessions` for a private agent owned by someone else
- **THEN** system SHALL return the same not-found result used for a missing agent
