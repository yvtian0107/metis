## MODIFIED Requirements

### Requirement: ITSM Service Desk uses shared Chat Workspace
The ITSM Service Desk frontend SHALL use the shared Chat Workspace for the service intake conversation. It SHALL not provide a separate composer, message list, scroll manager, stop button, header, or data surface parser outside Chat Workspace configuration and registered ITSM renderers. It SHALL use the service-desk semantic variants for welcome stage, active conversation, and sidebar presentation.

#### Scenario: Service desk welcome stage renders
- **WHEN** a user opens ITSM Service Desk with a configured service intake agent and no active session
- **THEN** the welcome stage SHALL render a bounded stage composer, service intake identity, image input, and quick prompts without stretching the composer across the full workspace

### Requirement: ITSM draft form surface renderer
The ITSM Service Desk frontend SHALL register an `itsm.draft_form` surface renderer with Chat Workspace. The renderer SHALL support draft loading, editable confirmation form, inline submit error, submitted state, ticket code display, workspace invalidation after successful submission, and a service-desk-specific visual treatment that reads as an AI-prepared service application draft.

#### Scenario: Submitted ticket result
- **WHEN** the draft submit API returns a submitted ticket result
- **THEN** the registered renderer SHALL show the submitted state, ticket code, and next-step message in the assistant response area
- **AND** the service desk workspace SHALL refresh affected session and ticket queries
