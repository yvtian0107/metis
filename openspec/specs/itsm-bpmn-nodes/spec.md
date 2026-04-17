# Delta Spec: itsm-bpmn-nodes

> Capability: itsm-bpmn-nodes
> Change: classic-engine-enterprise
> Type: MODIFIED

## MODIFIED Requirements

### Requirement: Approve node execution supports multi-person modes

The approve node SHALL execute differently based on execution_mode:
- **single**: First participant action completes the activity (existing behavior).
- **parallel**: All assigned participants MUST complete their assignment. Activity completes only when all assignments are resolved. Any single reject immediately completes the activity with "reject" outcome.
- **sequential**: Participants act in sequence order. Only the assignment with is_current=true can act. After each completion, is_current advances to the next assignment. The last assignment completion triggers activity completion.

#### Scenario: approve-single first person completes

- WHEN an approve node has execution_mode "single" (or empty)
- AND the first assigned participant submits an approve or reject action
- THEN the activity SHALL be marked as completed immediately
- AND the outcome SHALL reflect the participant's action

#### Scenario: approve-parallel all must act

- WHEN an approve node has execution_mode "parallel"
- AND all assigned participants submit an approve action
- THEN the activity SHALL be marked as completed with "approve" outcome only after the last participant acts

#### Scenario: approve-parallel any reject short-circuits

- WHEN an approve node has execution_mode "parallel"
- AND any one assigned participant submits a reject action
- THEN the activity SHALL be marked as completed immediately with "reject" outcome
- AND remaining unresolved assignments SHALL be cancelled

#### Scenario: approve-sequential advances is_current

- WHEN an approve node has execution_mode "sequential"
- AND the participant with is_current=true submits an approve action
- AND there are remaining assignments after the current one
- THEN is_current SHALL be set to false on the completed assignment
- AND is_current SHALL be set to true on the next assignment in sequence order
- AND the activity SHALL remain in progress

#### Scenario: approve-sequential last completes activity

- WHEN an approve node has execution_mode "sequential"
- AND the participant with is_current=true submits an action
- AND there are no remaining assignments after the current one
- THEN the activity SHALL be marked as completed
- AND the outcome SHALL reflect the last participant's action

### Requirement: Notify node sends actual notifications

The notify node SHALL invoke NotificationSender.Send() to deliver notifications via the configured channel_id. If NotificationSender is nil (not configured), the node SHALL skip sending and only record a timeline entry. Notification failure SHALL NOT block workflow advancement.

#### Scenario: notify sends via channel

- WHEN a notify node is executed
- AND NotificationSender is configured (non-nil)
- AND the node configuration includes a valid channel_id
- THEN the system SHALL invoke NotificationSender.Send() with the channel_id and rendered notification content
- AND a timeline entry SHALL be recorded

#### Scenario: notify skips when sender nil

- WHEN a notify node is executed
- AND NotificationSender is nil (not configured)
- THEN the system SHALL skip the send operation
- AND a timeline entry SHALL be recorded indicating notification was skipped
- AND workflow advancement SHALL proceed normally

#### Scenario: notify failure non-blocking

- WHEN a notify node is executed
- AND NotificationSender.Send() returns an error
- THEN the system SHALL log the error
- AND a timeline entry SHALL be recorded indicating notification failure
- AND workflow advancement SHALL NOT be blocked
- AND the activity SHALL be marked as completed
