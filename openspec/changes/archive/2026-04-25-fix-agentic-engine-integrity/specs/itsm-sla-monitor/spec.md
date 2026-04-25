## ADDED Requirements

### Requirement: SLA deadline feeds parallel convergence timeout
When itsm-sla-check detects that a ticket has parallel activity groups, the SLA resolution_deadline SHALL be available as a convergence timeout source for SmartEngine. The SLA monitor does NOT directly enforce convergence — it provides the deadline value. SmartEngine's ensureContinuation reads the ticket's SLA resolution_deadline to determine whether parallel groups have exceeded their timeout.

#### Scenario: SLA deadline available for convergence timeout
- **WHEN** a ticket has SLA resolution_deadline set and has a parallel activity group
- **THEN** SmartEngine's ensureContinuation can read the resolution_deadline as convergence timeout source

#### Scenario: No SLA on ticket
- **WHEN** a ticket has no SLA (resolution_deadline is null) and has a parallel activity group
- **THEN** SmartEngine falls back to EngineConfigProvider.ParallelConvergenceTimeout() for convergence timeout
