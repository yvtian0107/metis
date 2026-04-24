# chat-workspace-ui Specification

## Purpose
TBD - created by archiving change unify-chat-workspace. Update Purpose after archive.
## Requirements
### Requirement: Shared chat workspace shell
The system SHALL provide a shared frontend Chat Workspace shell used by both AI Management agent chat and ITSM Service Desk chat. The shell SHALL own the full-height layout, header, optional session sidebar, message scroll area, composer area, busy stop control, inline error area, and jump-to-bottom behavior.

#### Scenario: AI chat uses shared shell
- **WHEN** a user opens an AI Management agent chat session
- **THEN** the page SHALL render through the shared Chat Workspace shell
- **AND** the page SHALL NOT maintain a separate page-level implementation for message scroll, busy stop, inline error, or composer layout

#### Scenario: ITSM service desk uses shared shell
- **WHEN** a user opens an ITSM Service Desk session
- **THEN** the page SHALL render through the shared Chat Workspace shell
- **AND** the page SHALL NOT maintain a separate page-level implementation for message scroll, busy stop, inline error, or composer layout

### Requirement: Unified composer with image input
The Chat Workspace composer SHALL support multiline text, Enter-to-send, Shift+Enter newline, IME-safe key handling, image paste, image selection, preview, removal, upload, send pending state, and stop state. The same composer implementation SHALL be used by AI Management and ITSM Service Desk.

#### Scenario: ITSM user sends screenshot
- **WHEN** an ITSM Service Desk user pastes or selects an image and submits a message
- **THEN** the composer SHALL upload the image through the shared Agent Session image upload API
- **AND** the message SHALL be sent with the uploaded image URL included in the shared session message payload

#### Scenario: Upload fails
- **WHEN** an image upload fails before message submission
- **THEN** the composer SHALL keep the typed text and selected image previews
- **AND** the system SHALL show the upload error without sending a partial message

### Requirement: Unified agent header and switcher
The Chat Workspace SHALL provide one shared header and AgentSwitcher visual pattern for all chat surfaces. Business pages MAY provide different switch behavior, but the header structure, density, status treatment, and switch affordance SHALL remain consistent.

#### Scenario: AI agent switcher
- **WHEN** a user opens AI Management chat
- **THEN** the header SHALL show the current agent identity using the shared AgentSwitcher pattern
- **AND** switching agents SHALL create or enter a session for the selected agent according to AI Management behavior

#### Scenario: ITSM service desk agent switcher
- **WHEN** a user opens ITSM Service Desk
- **THEN** the header SHALL show the service intake agent using the shared AgentSwitcher pattern
- **AND** the label SHALL preserve ITSM service desk semantics such as service intake agent or smart staffing configuration

### Requirement: Surface registry
The Chat Workspace SHALL provide a surface registry for rendering tool activity, reasoning, plan progress, generic data surfaces, and business-specific surfaces. Business pages SHALL register surface renderers instead of directly parsing `UIMessage.parts` in page components.

#### Scenario: ITSM draft surface renders through registry
- **WHEN** an assistant message contains a `data-ui-surface` payload with `surfaceType="itsm.draft_form"`
- **THEN** Chat Workspace SHALL route the payload to the registered ITSM draft form renderer
- **AND** the page component SHALL NOT manually inspect `UIMessage.parts` to decide how to render that surface

#### Scenario: Unknown surface type
- **WHEN** an assistant message contains a data surface with no registered renderer
- **THEN** Chat Workspace SHALL keep the conversation stable
- **AND** the unknown surface SHALL NOT break text rendering or the rest of the message list

### Requirement: Business-specific configuration without duplicated chat systems
AI Management and ITSM Service Desk SHALL express differences through Chat Workspace configuration, slots, and registered surface renderers. They SHALL NOT fork independent chat systems for composer, message list, header, session sidebar, or protocol consumption.

#### Scenario: ITSM retains service desk features
- **WHEN** ITSM Service Desk needs to show draft confirmation, form editing, ticket submit result, or smart staffing guidance
- **THEN** those features SHALL be injected through Chat Workspace slots or surface renderers
- **AND** the base chat shell, composer, and message list SHALL remain shared

#### Scenario: UX update affects both chat surfaces
- **WHEN** the shared composer visual design or send behavior is changed
- **THEN** AI Management chat and ITSM Service Desk SHALL receive the change from the shared implementation
- **AND** developers SHALL NOT need to modify separate page-level composer implementations

