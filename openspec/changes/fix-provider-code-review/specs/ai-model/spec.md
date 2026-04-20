## MODIFIED Requirements

### Requirement: Default model per type
The system SHALL support one default model per **(provider_id, type)** combination. Setting a new default SHALL automatically clear the previous default of the same provider and type, within a single database transaction. A global query for the default model of a given type (used by internal services) SHALL return any model with `is_default=true` for that type across all providers.

#### Scenario: Set default model
- **WHEN** admin marks a model as default within a provider's detail page
- **THEN** the system clears `is_default` on all other models of the same provider and same type, and sets `is_default` on the selected model, atomically in one transaction

#### Scenario: Set default does not affect other providers
- **WHEN** admin sets model A as default LLM in Provider X
- **AND** Provider Y already has model B as default LLM
- **THEN** model B remains default in Provider Y

#### Scenario: Transaction failure rolls back
- **WHEN** the system fails to set `is_default=true` on the target model after clearing previous defaults
- **THEN** the entire operation is rolled back and the previous default remains unchanged

#### Scenario: Query default model (global)
- **WHEN** another module queries for the default LLM model
- **THEN** the system returns any model with `is_default=true` and `type=llm` (first found across all providers)

## ADDED Requirements

### Requirement: Model type classification fallback
The system SHALL assign type `"other"` to synced models whose model_id does not match any known pattern. The `"other"` type SHALL be included in the set of valid model types.

#### Scenario: Unknown model synced from provider
- **WHEN** a model is synced from an OpenAI-compatible provider and its model_id does not match any known LLM, embed, rerank, tts, stt, or image pattern
- **THEN** the model type SHALL be set to `"other"`

#### Scenario: Other type displayed in UI
- **WHEN** the frontend displays models grouped by type
- **THEN** models with type `"other"` SHALL appear in a dedicated "其他" panel
