## MODIFIED Requirements

### Requirement: Unified chat interface
The system SHALL provide a chat page at `/ai/chat/:sid` (or embeddable panel) with a unified interface for all agent types. The interface SHALL display: message bubbles (user on right, assistant on left), tool call/result collapsible blocks, streaming text with typing indicator, and input area with send button. The assistant message text SHALL be rendered using `ai-elements` `MessageResponse` (or equivalent `@ai-sdk/ui` component) to support incremental Markdown streaming without full re-parsing per frame.

#### Scenario: Stream text response
- **WHEN** agent streams text-delta events through Data Stream
- **THEN** UI SHALL render text incrementally using `MessageResponse` with a typing cursor, without DOM flashes or full-page re-renders

#### Scenario: Display tool call
- **WHEN** agent emits tool_call followed by tool_result
- **THEN** UI SHALL show a collapsible block with tool name, arguments, and result; the text output of tools MAY be rendered inside `MessageResponse` if provided as tool result parts

#### Scenario: Display plan (Plan & Execute)
- **WHEN** agent emits a plan event
- **THEN** UI SHALL show a numbered step list with progress indicators, updating as each step_start arrives, positioned above the assistant message rendered by `MessageResponse`

### Requirement: Cancel button
The chat interface SHALL display a "停止" button while execution is in progress. Clicking it SHALL call the cancel API.

#### Scenario: Cancel mid-execution
- **WHEN** user clicks "停止" while agent is responding
- **THEN** UI SHALL call `POST /api/v1/ai/sessions/:sid/cancel` and display the partial response with a "已中断" indicator; the partial text SHALL remain visible without disappearing during state transition
