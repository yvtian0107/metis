## MODIFIED Requirements

### Requirement: Decision tools data access layer
All 8 decision tools SHALL access data through Repository interfaces or dedicated query methods instead of raw `tx.Table()` queries. The `decisionToolContext` struct SHALL hold Repository references instead of a bare `*gorm.DB`.

#### Scenario: ticket_context tool uses Repository
- **WHEN** `decision.ticket_context` is called
- **THEN** it SHALL use `TicketRepo` and `ActivityRepo` methods instead of raw table queries for ticket data, activity history, executed actions, assignments, and parallel groups

#### Scenario: resolve_participant tool uses Repository
- **WHEN** `decision.resolve_participant` is called
- **THEN** it SHALL use `ParticipantResolver` and user lookup via Repository instead of `tx.Table("users")`

#### Scenario: similar_history tool uses Repository
- **WHEN** `decision.similar_history` is called
- **THEN** it SHALL use `TicketRepo.ListCompleted()` or equivalent instead of raw table queries

#### Scenario: Missing Repository methods added
- **WHEN** a decision tool requires a query not currently on the Repository
- **THEN** a new method SHALL be added to the appropriate Repository (e.g., `TicketRepo.GetContextForDecision`, `ActivityRepo.ListCompletedByTicket`)

### Requirement: Decision tools receive context via ToolHandler closure
Decision tools SHALL receive ticket-specific context (ticketID, serviceID, repositories) through the `ToolHandler` closure provided by SmartEngine, rather than through a shared mutable `decisionToolContext` struct.

#### Scenario: Tool handler closure captures ticket context
- **WHEN** SmartEngine builds the `ToolHandler` for a `DecisionRequest`
- **THEN** the closure SHALL capture the current ticket's repositories and IDs, and dispatch to the appropriate tool handler function
