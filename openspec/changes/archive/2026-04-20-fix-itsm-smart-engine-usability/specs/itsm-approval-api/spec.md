## MODIFIED Requirements

### Requirement: Approval count reflects multi-person pending state

GET /api/v1/itsm/tickets/approvals/count SHALL count activities where the requesting user has a pending assignment with is_current=true (for sequential) or status=pending (for parallel/single). An activity in parallel mode with some assignments completed but not all SHALL still appear in the user's pending list if their assignment is unresolved. For AI `pending_approval` activities, the count SHALL include only items where the requesting user is an authorized actionable approver for that AI confirmation.

#### Scenario: parallel partially-approved still counts for remaining users
- **WHEN** GET /api/v1/itsm/tickets/approvals/count is called by a user
- **AND** there exists a parallel-mode activity where some assignments are completed but the user's assignment is still pending
- **THEN** the activity SHALL be included in the user's pending approval count

#### Scenario: sequential only counts for current user
- **WHEN** GET /api/v1/itsm/tickets/approvals/count is called by a user
- **AND** there exists a sequential-mode activity where the user has an assignment but is_current=false
- **THEN** the activity SHALL NOT be included in the user's pending approval count

#### Scenario: completed approval not counted
- **WHEN** GET /api/v1/itsm/tickets/approvals/count is called by a user
- **AND** the user's assignment on an activity is already completed (approved or rejected)
- **THEN** the activity SHALL NOT be included in the user's pending approval count

#### Scenario: AI pending approval belongs to authorized approver only
- **WHEN** GET /api/v1/itsm/tickets/approvals/count is called by a user
- **AND** there exists a smart activity with status=`pending_approval`
- **THEN** the activity SHALL be counted only if that user is authorized to confirm or reject the AI decision

#### Scenario: Unauthorized user does not see AI pending approval in count
- **WHEN** GET /api/v1/itsm/tickets/approvals/count is called by a user who is not an authorized approver of a status=`pending_approval` activity
- **THEN** that activity SHALL NOT be included in the returned count

## ADDED Requirements

### Requirement: AI decision confirm and reject authorization
AI decision confirm/reject endpoints SHALL enforce server-side authorization against the actionable approver set of the `pending_approval` activity. Frontend visibility SHALL NOT be treated as sufficient authorization.

#### Scenario: Authorized user confirms AI decision
- **WHEN** an authorized approver requests confirmation of a status=`pending_approval` AI activity
- **THEN** the system SHALL allow the operation and persist the confirmation result

#### Scenario: Authorized user rejects AI decision
- **WHEN** an authorized approver requests rejection of a status=`pending_approval` AI activity with a reason
- **THEN** the system SHALL allow the operation and persist the rejection result and reason

#### Scenario: Unauthorized user cannot confirm AI decision
- **WHEN** a user who is not an authorized approver requests confirmation of a status=`pending_approval` AI activity
- **THEN** the system SHALL return 403 Forbidden

#### Scenario: Unauthorized user cannot reject AI decision
- **WHEN** a user who is not an authorized approver requests rejection of a status=`pending_approval` AI activity
- **THEN** the system SHALL return 403 Forbidden

#### Scenario: Non-pending AI activity cannot be confirmed twice
- **WHEN** a user requests confirmation or rejection of an AI activity whose status is no longer `pending_approval`
- **THEN** the system SHALL reject the operation as invalid current state
