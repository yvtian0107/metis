# Delta Spec: itsm-approval-api

> Capability: itsm-approval-api
> Change: classic-engine-enterprise
> Type: MODIFIED + ADDED

## MODIFIED Requirements

### Requirement: Approve endpoint respects multi-person mode

POST /api/v1/itsm/tickets/:id/activities/:aid/approve SHALL check the activity's execution_mode before calling engine.Progress(). For parallel mode, it SHALL complete the caller's assignment and check if all assignments are resolved. For sequential mode, it SHALL verify the caller owns the is_current assignment. The endpoint SHALL return the activity's current state (still pending if not all approvals done, or completed).

#### Scenario: approve in parallel mode completes own assignment

- WHEN POST /api/v1/itsm/tickets/:id/activities/:aid/approve is called
- AND the activity has execution_mode "parallel"
- THEN the system SHALL locate the caller's pending assignment
- AND mark it as completed with the caller's action (approve/reject)
- AND return the activity state reflecting remaining pending assignments

#### Scenario: approve in parallel mode all-done triggers progress

- WHEN POST /api/v1/itsm/tickets/:id/activities/:aid/approve is called
- AND the activity has execution_mode "parallel"
- AND the caller's assignment is the last unresolved assignment
- THEN the system SHALL complete the caller's assignment
- AND the system SHALL call engine.Progress() to advance the workflow
- AND return the activity state as completed

#### Scenario: approve in sequential mode wrong sequence rejected

- WHEN POST /api/v1/itsm/tickets/:id/activities/:aid/approve is called
- AND the activity has execution_mode "sequential"
- AND the caller does NOT own the assignment with is_current=true
- THEN the system SHALL return 403 Forbidden
- AND the response SHALL indicate it is not the caller's turn to act

#### Scenario: deny in parallel mode short-circuits

- WHEN POST /api/v1/itsm/tickets/:id/activities/:aid/approve is called with action "reject"
- AND the activity has execution_mode "parallel"
- THEN the system SHALL mark the caller's assignment as rejected
- AND the system SHALL immediately complete the activity with "reject" outcome
- AND remaining unresolved assignments SHALL be cancelled
- AND engine.Progress() SHALL be called to advance the workflow

## ADDED Requirements

### Requirement: Approval count reflects multi-person pending state

GET /api/v1/itsm/tickets/approvals/count SHALL count activities where the requesting user has a pending assignment with is_current=true (for sequential) or status=pending (for parallel/single). An activity in parallel mode with some assignments completed but not all SHALL still appear in the user's pending list if their assignment is unresolved.

#### Scenario: parallel partially-approved still counts for remaining users

- WHEN GET /api/v1/itsm/tickets/approvals/count is called by a user
- AND there exists a parallel-mode activity where some assignments are completed but the user's assignment is still pending
- THEN the activity SHALL be included in the user's pending approval count

#### Scenario: sequential only counts for current user

- WHEN GET /api/v1/itsm/tickets/approvals/count is called by a user
- AND there exists a sequential-mode activity where the user has an assignment but is_current=false
- THEN the activity SHALL NOT be included in the user's pending approval count

#### Scenario: completed approval not counted

- WHEN GET /api/v1/itsm/tickets/approvals/count is called by a user
- AND the user's assignment on an activity is already completed (approved or rejected)
- THEN the activity SHALL NOT be included in the user's pending approval count
