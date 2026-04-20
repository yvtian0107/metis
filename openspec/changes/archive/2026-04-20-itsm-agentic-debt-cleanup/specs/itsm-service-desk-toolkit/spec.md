## MODIFIED Requirements

### Requirement: Agent ticket creation uses TicketService
The `Operator.CreateTicket` method SHALL delegate to a `TicketCreator` interface implemented by `TicketService`, instead of directly inserting into the database. This ensures Agent-created tickets receive the same processing as UI-created tickets.

#### Scenario: Agent-created ticket starts workflow engine
- **WHEN** an Agent creates a ticket via `itsm.ticket_create` tool
- **THEN** the ticket SHALL have its workflow engine started (classic or smart), SLA deadlines calculated, and timeline events recorded — identical to a ticket created via the HTTP API

#### Scenario: Agent-created ticket has SLA deadlines
- **WHEN** an Agent creates a ticket for a service with an SLA template
- **THEN** the ticket SHALL have `sla_response_deadline` and `sla_resolution_deadline` populated

#### Scenario: Agent-created ticket records timeline
- **WHEN** an Agent creates a ticket
- **THEN** a `ticket_created` timeline event SHALL be recorded

### Requirement: ServiceDeskState explicit state machine
The `ServiceDeskState` stage transitions SHALL be validated by a centralized transition table. Invalid transitions SHALL return an error.

#### Scenario: Valid stage transition
- **WHEN** the state is `candidates_ready` and a transition to `service_selected` is requested
- **THEN** the transition SHALL succeed

#### Scenario: Invalid stage transition
- **WHEN** the state is `idle` and a transition to `confirmed` is requested
- **THEN** the transition SHALL return an error indicating the invalid transition

#### Scenario: Reset to idle always allowed
- **WHEN** a transition to `idle` is requested from any stage
- **THEN** the transition SHALL succeed (reset via `itsm.new_request`)

### Requirement: System prompt seed synchronization
The `SeedAgents` function SHALL update the system prompt of preset agents on every `seed.Sync()` invocation, matching agents by their `code` field. User-created agents (without a preset code) SHALL NOT be affected.

#### Scenario: Prompt updated on restart
- **WHEN** the application restarts and `seed.Sync()` runs
- **THEN** preset agents (matched by code) SHALL have their `system_prompt` updated to the latest version defined in code

#### Scenario: Non-preset agents unaffected
- **WHEN** a user creates a custom agent without a preset code
- **THEN** `seed.Sync()` SHALL NOT modify that agent's system prompt

### Requirement: Typed context keys for session ID
The `ai_session_id` context key SHALL use a typed `contextKey` type instead of a plain string, defined in a shared location accessible to both AI App and ITSM tools.

#### Scenario: Session ID injection uses typed key
- **WHEN** `CompositeToolExecutor` injects `ai_session_id` into context
- **THEN** it SHALL use the typed context key

#### Scenario: ITSM tool handlers read typed key
- **WHEN** ITSM tool handlers read `ai_session_id` from context
- **THEN** they SHALL use the same typed context key
