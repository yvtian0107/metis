## ADDED Requirements

### Requirement: Provider service test infrastructure
The system SHALL provide a test harness for `ProviderService` using an in-memory SQLite database with `Provider` and `AIModel` tables, and a deterministic 32-byte test encryption key.

#### Scenario: Setup test database
- **WHEN** a provider service test initializes
- **THEN** it SHALL migrate the `ai_providers` and `ai_models` tables into a shared-memory SQLite database

### Requirement: Test provider creation
The service-layer test suite SHALL verify that `ProviderService.Create` correctly persists providers with encrypted API keys and inferred protocols.

#### Scenario: Create an OpenAI-compatible provider
- **WHEN** `Create` is called with type="openai", baseURL="https://api.openai.com/v1", and an API key
- **THEN** the provider is persisted with protocol="openai", status="inactive", and the API key is encrypted

#### Scenario: Create an Anthropic provider
- **WHEN** `Create` is called with type="anthropic", baseURL="https://api.anthropic.com", and an API key
- **THEN** the provider is persisted with protocol="anthropic"

### Requirement: Test provider update and deletion
The service-layer test suite SHALL verify that updates correctly re-encrypt keys when provided, leave keys intact when empty, and delete cascades models.

#### Scenario: Update provider without changing API key
- **WHEN** `Update` is called with an empty API key for an existing provider
- **THEN** the existing encrypted API key is preserved

#### Scenario: Update provider with new API key
- **WHEN** `Update` is called with a non-empty API key
- **THEN** the API key is re-encrypted and stored

#### Scenario: Delete provider removes associated models
- **WHEN** `Delete` is called for a provider that has associated models
- **THEN** the provider and all its models are removed

### Requirement: Test API key masking
The service-layer test suite SHALL verify that `MaskAPIKey` returns a human-readable masked string.

#### Scenario: Mask a long API key
- **WHEN** `MaskAPIKey` is called on a provider with an API key longer than 8 characters
- **THEN** it returns the first 3 characters, "****", and the last 4 characters

#### Scenario: Mask a short API key
- **WHEN** `MaskAPIKey` is called on a provider with an API key of 8 or fewer characters
- **THEN** it returns "****"
