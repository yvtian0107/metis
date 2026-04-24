## ADDED Requirements

### Requirement: Reusable chat session sidebar
The system SHALL provide a reusable Chat Workspace session sidebar that can be configured for AI Management chat and ITSM Service Desk. The sidebar SHALL support the same base interaction model, selected state, loading state, empty state, date grouping where enabled, and new-session action.

#### Scenario: AI chat sidebar uses reusable component
- **WHEN** a user opens AI Management chat with a current agent
- **THEN** the session sidebar SHALL be rendered by the reusable Chat Workspace session sidebar
- **AND** AI-specific actions such as rename, delete, pin, export, or collapse SHALL be configured through sidebar capabilities

#### Scenario: ITSM service desk sidebar uses reusable component
- **WHEN** a user opens ITSM Service Desk
- **THEN** the service desk session list SHALL be rendered by the reusable Chat Workspace session sidebar
- **AND** it SHALL use the same selected state and density as AI Management chat while preserving ITSM labels

### Requirement: Sidebar behavior changes in one place
Changes to shared sidebar interaction or visual treatment SHALL be implemented in the reusable Chat Workspace session sidebar and apply to all configured chat surfaces.

#### Scenario: Selected style updated
- **WHEN** the selected session style is changed in the shared sidebar
- **THEN** AI Management chat and ITSM Service Desk SHALL both reflect the updated selected state without page-specific style changes
