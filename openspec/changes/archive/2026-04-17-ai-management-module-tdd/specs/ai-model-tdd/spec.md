## ADDED Requirements

### Requirement: Model service test infrastructure
The system SHALL provide a test harness for `ModelService` using an in-memory SQLite database with `AIModel` and `Provider` tables.

#### Scenario: Setup test database
- **WHEN** a model service test initializes
- **THEN** it SHALL migrate the `ai_models` and `ai_providers` tables into a shared-memory SQLite database

### Requirement: Test model CRUD
The service-layer test suite SHALL verify creation, retrieval, update, and deletion of AI models.

#### Scenario: Create a model linked to a provider
- **WHEN** `Create` is called with a valid `AIModel` referencing an existing provider
- **THEN** the model is persisted with the correct provider ID and default status="active"

#### Scenario: Update model fields
- **WHEN** `Update` is called with changes to display name, context window, and prices
- **THEN** the persisted model reflects those changes

#### Scenario: Delete a model
- **WHEN** `Delete` is called for an existing model
- **THEN** the model is removed from the database

### Requirement: Test default model switching
The service-layer test suite SHALL verify that only one model per type can be the default.

#### Scenario: Set a model as default
- **WHEN** `SetDefault` is called for a model of type="llm"
- **THEN** that model becomes `isDefault=true` and any previously default LLM becomes `isDefault=false`

#### Scenario: Switch default model
- **WHEN** `SetDefault` is called for a second LLM after a first LLM is already default
- **THEN** only the second LLM has `isDefault=true`

### Requirement: Test preset model sync
The service-layer test suite SHALL verify that syncing preset models from a provider adds missing models and skips existing ones.

#### Scenario: Sync Anthropic preset models
- **WHEN** `SyncModels` is called for an Anthropic provider
- **THEN** all `AnthropicPresetModels` are inserted, each with correct capabilities, context window, and max output tokens

#### Scenario: Re-sync does not duplicate
- **WHEN** `SyncModels` is called again for the same provider
- **THEN** no duplicate models are created
