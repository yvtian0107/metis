# Delta Spec: itsm-sla

> Capability: itsm-sla
> Change: classic-engine-enterprise
> Type: ADDED

## ADDED Requirements

### Requirement: SLA pause and resume

The system SHALL support pausing and resuming SLA timers on tickets.
- **Pause**: sets ticket.sla_paused_at to current time. Paused tickets are excluded from itsm-sla-check scanning.
- **Resume**: calculates pause duration (now - sla_paused_at), extends response_deadline and resolution_deadline by that duration, clears sla_paused_at.
- **API**: PUT /api/v1/itsm/tickets/:id/sla/pause and PUT /api/v1/itsm/tickets/:id/sla/resume.

#### Scenario: pause sets sla_paused_at

- WHEN PUT /api/v1/itsm/tickets/:id/sla/pause is called for an active ticket
- THEN the system SHALL set ticket.sla_paused_at to the current timestamp
- AND the system SHALL return 200 OK with the updated ticket state

#### Scenario: resume extends deadlines

- WHEN PUT /api/v1/itsm/tickets/:id/sla/resume is called for a paused ticket
- THEN the system SHALL calculate pause_duration as (now - sla_paused_at)
- AND the system SHALL extend response_deadline by pause_duration
- AND the system SHALL extend resolution_deadline by pause_duration
- AND the system SHALL clear sla_paused_at to nil
- AND the system SHALL return 200 OK with the updated ticket state

#### Scenario: paused ticket excluded from scan

- WHEN the itsm-sla-check task scans tickets for SLA breaches
- AND a ticket has sla_paused_at set (non-nil)
- THEN the scanner SHALL skip that ticket entirely
- AND no escalation or breach SHALL be recorded for the paused ticket

#### Scenario: resume on non-paused ticket rejected

- WHEN PUT /api/v1/itsm/tickets/:id/sla/resume is called
- AND the ticket's sla_paused_at is nil (not paused)
- THEN the system SHALL return 400 Bad Request
- AND the response SHALL indicate that the ticket is not currently paused

#### Scenario: pause on terminal ticket rejected

- WHEN PUT /api/v1/itsm/tickets/:id/sla/pause is called
- AND the ticket is in a terminal state (closed, cancelled, resolved)
- THEN the system SHALL return 400 Bad Request
- AND the response SHALL indicate that SLA cannot be paused on a terminal ticket
