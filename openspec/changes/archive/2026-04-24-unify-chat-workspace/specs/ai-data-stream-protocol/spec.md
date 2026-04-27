## ADDED Requirements

### Requirement: Frontend surface consumption through registry
The frontend SHALL consume Data Stream data parts and `data-ui-surface` payloads through a shared Chat Workspace surface registry. Business pages SHALL register renderers for business surface types instead of directly parsing protocol parts inside route components.

#### Scenario: Registered surface payload
- **WHEN** the stream emits a data surface whose type has a registered renderer
- **THEN** the Chat Workspace SHALL render it through that renderer
- **AND** the surface lifecycle SHALL remain consistent with text, tool, reasoning, and plan rendering

#### Scenario: Protocol adapter update
- **WHEN** the frontend adapter for Data Stream chunks or UIMessage parts changes
- **THEN** the update SHALL be made in the shared Chat Workspace protocol/surface layer
- **AND** AI Management and ITSM Service Desk SHALL NOT require separate page-level protocol changes

### Requirement: Business surface text suppression
The surface registry SHALL allow a renderer to declare that assistant text should be suppressed when the registered surface is present, so structured UI can be the primary confirmation interface.

#### Scenario: ITSM draft form suppresses summary text
- **WHEN** an ITSM draft form surface is present and its renderer declares text suppression
- **THEN** the Chat Workspace SHALL render the draft form as the primary assistant response
- **AND** the ordinary text summary SHALL NOT duplicate the confirmation UI
