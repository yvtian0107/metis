## MODIFIED Requirements

### Requirement: Provider CRUD
The system SHALL support creating, reading, updating, and deleting AI providers. Each provider SHALL have: name, type (`openai` / `anthropic` / `ollama`), protocol (`openai` / `anthropic`), base_url, api_key (encrypted), status (`active` / `inactive` / `error`), and health_checked_at timestamp.

#### Scenario: Create a provider
- **WHEN** admin submits a valid provider form with name, type, base_url, and api_key
- **THEN** the system creates the provider with api_key encrypted via AES-256-GCM, status set to `inactive`, and protocol auto-derived from type

#### Scenario: List providers
- **WHEN** admin requests the provider list
- **THEN** the system returns paginated providers with api_key masked (show only last 4 chars), including model count per provider

#### Scenario: Update a provider without changing connection parameters
- **WHEN** admin updates only the provider name or type (not base_url or api_key)
- **THEN** the system saves the changes and preserves the current status

#### Scenario: Update a provider with changed connection parameters
- **WHEN** admin updates a provider's base_url or provides a new api_key
- **THEN** the system saves the changes, re-encrypting api_key if changed, and resets status to `inactive`

#### Scenario: Delete a provider
- **WHEN** admin deletes a provider that has no active models
- **THEN** the system soft-deletes the provider

#### Scenario: Delete a provider with active models
- **WHEN** admin deletes a provider that has active models
- **THEN** the system rejects the deletion with error message

### Requirement: Provider connectivity test
The system SHALL provide an API to test a provider's connectivity and API key validity.

#### Scenario: Test OpenAI-compatible provider
- **WHEN** admin triggers a connectivity test for a provider with protocol `openai`
- **THEN** the system calls `GET {base_url}/v1/models` with the decrypted api_key and returns success/failure with error detail

#### Scenario: Test Anthropic provider
- **WHEN** admin triggers a connectivity test for a provider with protocol `anthropic`
- **THEN** the system sends a minimal chat request using a model from the provider's synced model list (or fallback to `claude-haiku-3-5-20241022` if no models are synced) and returns success/failure

#### Scenario: Successful connectivity test
- **WHEN** a connectivity test succeeds
- **THEN** the provider status is updated to `active` and health_checked_at is set to current time

#### Scenario: Failed connectivity test
- **WHEN** a connectivity test fails
- **THEN** the provider status is updated to `error` and the error message is returned to the caller

## ADDED Requirements

### Requirement: Request parameter validation
The system SHALL validate path parameters (e.g., `:id`) in all AI provider and model API handlers. Invalid (non-numeric) path parameters SHALL result in a 400 Bad Request response.

#### Scenario: Non-numeric ID in path
- **WHEN** a request is made to `/api/v1/ai/providers/abc`
- **THEN** the system returns HTTP 400 with a descriptive error message

### Requirement: LIKE query escaping
The system SHALL escape SQL LIKE wildcards (`%` and `_`) in user-supplied keyword search parameters before constructing LIKE queries.

#### Scenario: Search with percent character
- **WHEN** admin searches providers with keyword `100%`
- **THEN** the system treats `%` as a literal character, not a wildcard

### Requirement: Graceful count query errors
The system SHALL log a warning (not silently ignore) when auxiliary count queries (model counts, type counts) fail during provider listing, while still returning the main provider data.

#### Scenario: Count query fails during list
- **WHEN** the model count query fails but the main provider query succeeds
- **THEN** the system returns providers with zero counts and logs a warning
