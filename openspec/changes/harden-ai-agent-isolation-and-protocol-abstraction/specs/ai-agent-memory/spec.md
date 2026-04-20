## MODIFIED Requirements

### Requirement: Memory management API
The system SHALL provide REST endpoints under `/api/v1/ai/agents/:id/memories` with JWT auth:
- `GET /` — list current user's memories for this agent
- `POST /` — create/update a memory entry (source=`user_set`)
- `DELETE /:mid` — delete a specific memory entry

All memory API handlers SHALL load the current user identity from the same JWT context key used by the rest of the AI module. Memory listing, creation, update, and deletion SHALL be scoped to the current user and target agent. Deleting a memory entry that does not belong to the current user for that agent SHALL behave the same as deleting a missing memory.

#### Scenario: User views memories
- **WHEN** user requests `GET /api/v1/ai/agents/:id/memories`
- **THEN** system SHALL return all memory entries for the current user and this agent

#### Scenario: User deletes memory
- **WHEN** user deletes a memory entry (regardless of source)
- **THEN** system SHALL soft-delete the entry and it SHALL no longer be injected into context

#### Scenario: User manually sets memory
- **WHEN** user creates a memory via API with key and content
- **THEN** system SHALL store it with source `user_set`

#### Scenario: Memory handler reads standard user context key
- **WHEN** a memory API request is authenticated by JWT middleware
- **THEN** the handler SHALL resolve the current user from the standard `userId` context key used by the AI module

#### Scenario: Cross-user memory delete behaves as not found
- **WHEN** a user requests `DELETE /api/v1/ai/agents/:id/memories/:mid` for a memory entry owned by another user
- **THEN** system SHALL return the same not-found result used for a missing memory
