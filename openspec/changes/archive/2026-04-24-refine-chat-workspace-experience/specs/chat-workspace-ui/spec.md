## MODIFIED Requirements

### Requirement: Shared chat workspace shell
The system SHALL provide a shared frontend Chat Workspace shell used by both AI Management agent chat and ITSM Service Desk chat. The shell SHALL own the full-height layout, header, optional session sidebar, message scroll area, composer area, busy stop control, inline error area, jump-to-bottom behavior, and semantic layout variants for density, message width, composer placement, and empty-state tone.

#### Scenario: AI chat uses shared shell
- **WHEN** a user opens an AI Management agent chat session
- **THEN** the page SHALL render through the shared Chat Workspace shell
- **AND** the page SHALL choose shared layout semantics instead of page-local composer or message layout styling

#### Scenario: ITSM service desk uses shared shell
- **WHEN** a user opens an ITSM Service Desk session
- **THEN** the page SHALL render through the shared Chat Workspace shell
- **AND** the page SHALL use a service-desk tone while preserving shared header, composer, sidebar, and message flow behavior

### Requirement: Unified composer with image input
The Chat Workspace composer SHALL support multiline text, Enter-to-send, Shift+Enter newline, IME-safe key handling, image paste, image selection, preview, removal, upload, send pending state, stop state, and semantic visual variants. Business pages SHALL select composer variant and width through shared props, not naked layout class overrides.

#### Scenario: ITSM stage composer
- **WHEN** ITSM Service Desk renders the welcome stage
- **THEN** the composer SHALL use the shared stage variant with service-desk attachment tone
- **AND** the composer SHALL keep a bounded professional width rather than expanding to the full workspace

### Requirement: Business-specific configuration without duplicated chat systems
AI Management and ITSM Service Desk SHALL express differences through Chat Workspace configuration, slots, registered surface renderers, and semantic visual variants. They SHALL NOT fork independent chat systems for composer, message list, header, session sidebar, protocol consumption, or core sizing.

#### Scenario: UX update affects both chat surfaces
- **WHEN** shared composer, message flow, or sidebar design changes
- **THEN** AI Management chat and ITSM Service Desk SHALL receive the change from the shared implementation
- **AND** business pages SHALL NOT need to patch core chat layout with page-local sizing classes

### Requirement: Timeline message flow
The Chat Workspace message flow SHALL render visible `UIMessage[]` entries as a timeline instead of requiring user and assistant messages to form pairs. Empty states SHALL only render when there are no visible messages and the workspace is not busy.

#### Scenario: User message is pending assistant response
- **WHEN** a user sends a message and no assistant response has been created yet
- **THEN** the user message SHALL remain visible in the message timeline
- **AND** the workspace SHALL show a lightweight processing state rather than the empty or welcome state
