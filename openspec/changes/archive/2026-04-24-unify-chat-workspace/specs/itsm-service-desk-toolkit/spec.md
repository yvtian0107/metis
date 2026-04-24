## ADDED Requirements

### Requirement: ITSM Service Desk uses shared Chat Workspace
The ITSM Service Desk frontend SHALL use the shared Chat Workspace for the service intake conversation. It SHALL not provide a separate composer, message list, scroll manager, stop button, header, or data surface parser outside Chat Workspace configuration and registered ITSM renderers.

#### Scenario: Service desk conversation opens
- **WHEN** a user opens ITSM Service Desk with a configured service intake agent
- **THEN** the conversation SHALL render with the shared Chat Workspace shell
- **AND** the header SHALL identify the service intake agent using the shared AgentSwitcher pattern

#### Scenario: Service desk sends image context
- **WHEN** a user includes an image in an ITSM Service Desk message
- **THEN** the image SHALL be uploaded and sent through the shared Agent Session image message flow
- **AND** the service desk agent SHALL receive the message in the same session context as text-only messages

### Requirement: ITSM draft form surface renderer
The ITSM Service Desk frontend SHALL register an `itsm.draft_form` surface renderer with Chat Workspace. The renderer SHALL support draft loading, editable confirmation form, inline submit error, submitted state, ticket code display, and workspace invalidation after successful submission.

#### Scenario: Draft loading surface
- **WHEN** the stream contains an `itsm.draft_form` surface with loading status
- **THEN** the registered renderer SHALL show a draft preparation state inside the assistant response area

#### Scenario: Editable draft confirmation
- **WHEN** the stream contains an `itsm.draft_form` surface with a valid form schema
- **THEN** the registered renderer SHALL display the service form as an editable confirmation UI
- **AND** submitting it SHALL call the ITSM draft submit API with the current draft version and form data

#### Scenario: Submitted ticket result
- **WHEN** the draft submit API returns a submitted ticket result
- **THEN** the registered renderer SHALL show the submitted state and ticket code in the assistant response area
- **AND** the service desk workspace SHALL refresh affected session and ticket queries

### Requirement: Service desk not-on-duty state remains business-specific
When no service intake agent is configured, ITSM Service Desk SHALL use a business-specific not-on-duty state that leads the user to Smart Staffing configuration. This state MAY be injected into Chat Workspace as an empty or unavailable state but SHALL use shared chat visual language.

#### Scenario: No intake agent configured
- **WHEN** the service intake post has no configured agent
- **THEN** ITSM Service Desk SHALL show an unavailable state that clearly explains the service intake agent is not configured
- **AND** the primary action SHALL lead to Smart Staffing configuration
