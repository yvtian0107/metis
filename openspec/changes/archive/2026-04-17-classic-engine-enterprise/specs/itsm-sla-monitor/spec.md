## ADDED Requirements

### Requirement: itsm-sla-check cron task runs every 60 seconds

The itsm-sla-check scheduler task SHALL be registered as a cron task with a 60-second interval. It SHALL execute automatically without manual triggering.

#### Scenario: Response deadline breach detected
- **WHEN** itsm-sla-check runs
- **WHEN** a ticket has status=in_progress, sla_status="active", and response_deadline has passed
- **THEN** the ticket's sla_status is updated to "breached_response"
- **THEN** a timeline entry records the SLA response breach

### Requirement: Task scans in-progress tickets with active SLA

The itsm-sla-check task SHALL scan all tickets where status is "in_progress" AND sla_status is NOT IN ("breached_resolution"). Tickets with sla_status of "breached_response" SHALL still be scanned for resolution deadline checks.

#### Scenario: Resolution deadline breach detected
- **WHEN** itsm-sla-check runs
- **WHEN** a ticket has status=in_progress, sla_status="breached_response", and resolution_deadline has passed
- **THEN** the ticket's sla_status is updated to "breached_resolution"
- **THEN** a timeline entry records the SLA resolution breach

### Requirement: Task checks response and resolution deadlines

For each scanned ticket, the itsm-sla-check task SHALL evaluate whether response_deadline or resolution_deadline has passed relative to the current time. Response deadline is checked when sla_status is "active". Resolution deadline is checked when sla_status is "active" or "breached_response".

### Requirement: Breach detection triggers escalation rule matching

On breach detection, the task SHALL update the ticket's sla_status and then query EscalationRule records matching the ticket's service definition and the breach trigger_type (e.g., "response_breached", "resolution_breached"). Matched rules SHALL be executed in priority order.

#### Scenario: Escalation rule — notify on breach
- **WHEN** a response deadline breach is detected for a ticket
- **WHEN** an EscalationRule exists with trigger_type="response_breached" and action="notify"
- **THEN** a notification is sent to the escalation rule's target (user or role)
- **THEN** a timeline entry records the escalation action

#### Scenario: Escalation rule — reassign on breach
- **WHEN** a resolution deadline breach is detected for a ticket
- **WHEN** an EscalationRule exists with trigger_type="resolution_breached" and action="reassign"
- **THEN** the ticket's assignee_id is updated to the escalation rule's target user
- **THEN** a timeline entry records the reassignment

### Requirement: Escalation actions — notify, reassign, escalate_priority

Escalation actions SHALL support three types: "notify" sends a notification to the escalation target (user ID or role), "reassign" changes the ticket's assignee_id to the target user, and "escalate_priority" increases the ticket's priority by one level (e.g., medium to high). Each executed action SHALL be recorded in the ticket timeline.

### Requirement: SLA pause via API

PUT /api/v1/itsm/tickets/:id/sla/pause SHALL set sla_paused_at to the current timestamp and transition sla_status to include the paused state. A ticket that is already paused SHALL return an error.

#### Scenario: Pause then resume extends deadline
- **WHEN** a ticket has response_deadline at T+4h and resolution_deadline at T+8h
- **WHEN** SLA is paused at T+1h
- **WHEN** SLA is resumed at T+3h (2 hours of pause)
- **THEN** response_deadline is extended to T+6h (original + 2h pause duration)
- **THEN** resolution_deadline is extended to T+10h (original + 2h pause duration)
- **THEN** sla_paused_at is cleared

### Requirement: SLA resume via API

PUT /api/v1/itsm/tickets/:id/sla/resume SHALL calculate the pause duration (now minus sla_paused_at), extend both response_deadline and resolution_deadline by that duration, and clear sla_paused_at. A ticket that is not paused SHALL return an error.

### Requirement: Paused tickets excluded from SLA scanning

Tickets with sla_paused_at set (non-null) SHALL be excluded from the itsm-sla-check scanning query. The scanner MUST NOT evaluate deadlines or trigger escalations for paused tickets.

#### Scenario: Paused ticket skipped by scanner
- **WHEN** itsm-sla-check runs
- **WHEN** a ticket has status=in_progress and sla_paused_at is set
- **THEN** the ticket is not evaluated for deadline breaches
- **THEN** no escalation rules are triggered for the ticket
