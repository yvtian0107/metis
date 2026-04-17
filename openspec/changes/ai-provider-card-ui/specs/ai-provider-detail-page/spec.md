## ADDED Requirements

### Requirement: Provider detail page layout
The system SHALL provide a detail page at route `/ai/providers/:id` with two sections: a provider information section and a model management section. The page SHALL display a back navigation link to the provider list.

#### Scenario: Navigate to detail page
- **WHEN** user accesses `/ai/providers/:id` with a valid provider ID
- **THEN** the system SHALL fetch the provider via `GET /api/v1/ai/providers/:id` and display the detail page with provider info and models

#### Scenario: Navigate with invalid provider ID
- **WHEN** user accesses `/ai/providers/:id` with a non-existent ID
- **THEN** the system SHALL show an error state or redirect to the provider list

### Requirement: Provider information section
The detail page SHALL display provider metadata in a structured layout: name, type (with brand badge), protocol, base URL, masked API key, status indicator, and last health check timestamp. The section SHALL include "Test Connection" and "Sync Models" action buttons, and an "Edit" button that opens a Drawer/inline form for editing provider fields.

#### Scenario: Display provider info
- **WHEN** the detail page loads for a provider
- **THEN** the system SHALL show all provider fields in a description-list layout with labels and values

#### Scenario: Edit provider from detail page
- **WHEN** user clicks "Edit" on the detail page
- **THEN** the system SHALL open the ProviderSheet in edit mode with the current provider data pre-filled

#### Scenario: Test connection from detail page
- **WHEN** user clicks "Test Connection" on the detail page
- **THEN** the system SHALL call `POST /api/v1/ai/providers/:id/test`, update the status indicator in real-time, and show a toast with the result

#### Scenario: Sync models from detail page
- **WHEN** user clicks "Sync Models" on the detail page
- **THEN** the system SHALL call `POST /api/v1/ai/providers/:id/sync-models`, refresh the model list on success, and show a toast with the count of added models

### Requirement: Model management in detail page
The detail page SHALL display all models for the provider in a grouped table layout (grouped by model type: LLM, Embed, Rerank, TTS, STT, Image). The model section SHALL support search filtering, creating new models, editing models, deleting models, and setting a model as default.

#### Scenario: Display models grouped by type
- **WHEN** the detail page loads and the provider has models
- **THEN** the system SHALL fetch models via `GET /api/v1/ai/models?providerId=:id&pageSize=100` and display them grouped by type with type headers showing count

#### Scenario: Search models
- **WHEN** user enters a keyword in the model search field
- **THEN** the displayed models SHALL be filtered client-side by display name or model ID matching the keyword

#### Scenario: Create model from detail page
- **WHEN** user clicks "Add Model" in the model section
- **THEN** the system SHALL open the ModelSheet with the current provider pre-selected

#### Scenario: Empty model state
- **WHEN** the provider has no models
- **THEN** the system SHALL display an empty state with a "Sync Models" prompt and a manual "Add Model" button
