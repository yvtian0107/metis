## ADDED Requirements

### Requirement: Provider card display
The system SHALL display each AI provider as a card with: a top color stripe (3-4px) in the provider's brand color, a first-letter Avatar with brand background, the provider name, base URL (truncated), masked API key, model type statistics as chips, status indicator dot, and last health check timestamp.

#### Scenario: Display provider card with brand identity
- **WHEN** a provider of type "openai" is rendered in the list
- **THEN** the card SHALL show an emerald color stripe, an Avatar with "AI" text on emerald-50 background, and all provider metadata

#### Scenario: Display model type statistics
- **WHEN** a provider has models of different types (e.g., 4 LLM, 2 Embed, 1 Rerank)
- **THEN** the card SHALL display compact chips showing each type with its count (e.g., "LLM 4", "Embed 2", "Rerank 1")

#### Scenario: Display status indicator
- **WHEN** a provider has status "active"
- **THEN** the card SHALL show a green dot with subtle pulse animation and relative time since last health check

#### Scenario: Display error status indicator
- **WHEN** a provider has status "error"
- **THEN** the card SHALL show a red dot with faster pulse animation

#### Scenario: Display inactive status indicator
- **WHEN** a provider has status "inactive" or has never been tested
- **THEN** the card SHALL show a gray static dot

### Requirement: Card grid layout
The system SHALL render provider cards in a responsive CSS Grid with `auto-fill, minmax(340px, 1fr)` and gap-4 spacing. The grid SHALL load all providers without pagination (pageSize=100).

#### Scenario: Responsive grid columns
- **WHEN** the viewport width changes
- **THEN** the grid SHALL automatically adjust column count (3 cols on desktop, 2 on tablet, 1 on mobile)

### Requirement: Card interactions
Each provider card SHALL support hover effect (border highlight + shadow elevation), click to navigate to detail page, a quick test connection button visible in the card footer, and a ⋯ dropdown menu with edit and delete actions.

#### Scenario: Navigate to detail page
- **WHEN** user clicks on a provider card (non-action area)
- **THEN** the system SHALL navigate to `/ai/providers/:id`

#### Scenario: Quick test connection from card
- **WHEN** user clicks the test button on a card
- **THEN** the system SHALL call `POST /api/v1/ai/providers/:id/test`, show a spinner on the status dot during testing, update the dot color on completion, and show a toast with the result

#### Scenario: Card dropdown menu actions
- **WHEN** user clicks the ⋯ menu on a card
- **THEN** the system SHALL show a dropdown with "Edit" (navigates to detail page) and "Delete" (shows confirmation dialog) options

### Requirement: Guide card for adding providers
When providers exist, the system SHALL display a dashed-border guide card at the end of the grid as a visual prompt to add more providers. The guide card SHALL display a "+" icon and "Add provider" text, and clicking it SHALL open the create provider Drawer.

#### Scenario: Display guide card
- **WHEN** the provider list has at least one provider
- **THEN** a dashed-border guide card SHALL appear as the last item in the grid

#### Scenario: Empty state display
- **WHEN** the provider list is empty
- **THEN** the system SHALL display a centered empty state with Server icon, descriptive text, and a primary "Add first provider" button

### Requirement: Brand color mapping
The system SHALL maintain a provider type to color mapping: openai → emerald, anthropic → amber, ollama → sky. Unknown types SHALL fall back to the primary theme color.

#### Scenario: Fallback for unknown provider type
- **WHEN** a provider has a type not in the known mapping (e.g., a future "gemini" type)
- **THEN** the card SHALL render with the primary theme color for stripe and Avatar
