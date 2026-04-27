## MODIFIED Requirements

### Requirement: Unified chat interface
The system SHALL provide chat surfaces at `/ai/chat/:sid` and any business-embedded agent chat such as ITSM Service Desk through the shared Chat Workspace interface. The interface SHALL display user messages, assistant responses, tool call/result activity, reasoning blocks, plan progress, streaming text, business data surfaces, and a shared composer with send/stop controls. Assistant message text SHALL be rendered using `ai-elements` `MessageResponse` or an equivalent `@ai-sdk/ui` component to support incremental Markdown streaming without full re-parsing per frame. Page components SHALL NOT duplicate message grouping, composer, stop control, scroll handling, or data surface parsing.

#### Scenario: Stream text response
- **WHEN** agent streams text-delta events through Data Stream
- **THEN** UI SHALL render text incrementally using `MessageResponse` with a typing cursor, without DOM flashes or full-page re-renders

#### Scenario: Display tool call
- **WHEN** agent emits tool_call followed by tool_result
- **THEN** UI SHALL show a collapsible block with tool name, arguments, and result; the text output of tools MAY be rendered inside `MessageResponse` if provided as tool result parts

#### Scenario: Display plan (Plan & Execute)
- **WHEN** agent emits a plan event
- **THEN** UI SHALL show a numbered step list with progress indicators, updating as each step_start arrives, positioned above the assistant message rendered by `MessageResponse`

#### Scenario: Business surface rendered by workspace
- **WHEN** a business agent chat emits a registered data surface
- **THEN** the shared Chat Workspace SHALL render the surface through its registry
- **AND** the business page SHALL NOT parse `UIMessage.parts` directly for normal surface rendering

#### Scenario: ITSM chat has full input parity
- **WHEN** a user chats with the ITSM Service Desk agent
- **THEN** the input area SHALL provide the same shared text and image input behavior as AI Management chat

## ADDED Requirements

### Requirement: Agent chat page adopts Chat Workspace
The AI Management chat session page SHALL be a configuration of the shared Chat Workspace. AI-specific capabilities such as memory panel, editable user messages, session deletion, and continue generation SHALL be injected as Chat Workspace actions or panels.

#### Scenario: Memory panel remains available
- **WHEN** a user opens an AI Management chat session with an agent
- **THEN** the memory panel action SHALL remain available through the Chat Workspace header or panel slot

#### Scenario: User message edit remains available
- **WHEN** editable user messages are enabled for AI Management chat
- **THEN** the shared message pair renderer SHALL expose the edit action
- **AND** saving the edit SHALL regenerate from the edited message
