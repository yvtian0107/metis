## ADDED Requirements

### Requirement: Transfer operation reassigns activity to target user

Transfer (转办): POST /api/v1/itsm/tickets/:id/activities/:aid/transfer with body {target_user_id} SHALL create a new pending assignment for the target user, mark the original assignment as "transferred" with transfer_from set to the original assignee, update ticket.assignee_id to the target user, and record a timeline entry for the transfer.

#### Scenario: Transfer to another user
- **WHEN** user A is the current assignee of an activity
- **WHEN** user A calls POST /api/v1/itsm/tickets/:id/activities/:aid/transfer with target_user_id=B
- **THEN** a new pending assignment is created for user B
- **THEN** user A's assignment status is set to "transferred"
- **THEN** ticket.assignee_id is updated to user B
- **THEN** a timeline entry records "user A transferred to user B"

#### Scenario: Transfer by non-assignee is rejected
- **WHEN** user C is not the current assignee and is not an admin
- **WHEN** user C calls POST /api/v1/itsm/tickets/:id/activities/:aid/transfer
- **THEN** the request returns HTTP 403
- **THEN** no assignment changes are made

### Requirement: Delegate operation with auto-return to original assignee

Delegate (委派): POST /api/v1/itsm/tickets/:id/activities/:aid/delegate with body {target_user_id} SHALL create a new pending assignment for the target user with delegated_from set to the original assignee, and mark the original assignment as "delegated". When the delegate completes their assignment, if delegated_from is set, the engine SHALL automatically create a new pending assignment back to the original assignee (auto-return). Timeline SHALL record both the delegation and the auto-return.

#### Scenario: Delegate and auto-return
- **WHEN** user A delegates an activity to user B
- **THEN** a new pending assignment is created for user B with delegated_from=A
- **THEN** user A's assignment status is set to "delegated"
- **THEN** a timeline entry records "user A delegated to user B"
- **WHEN** user B completes the delegated assignment
- **THEN** a new pending assignment is automatically created for user A
- **THEN** a timeline entry records "auto-returned from user B to user A"

### Requirement: Claim operation among multiple assignees

Claim (抢单): POST /api/v1/itsm/tickets/:id/activities/:aid/claim SHALL require the activity to have multiple unresolved (pending) assignments. The calling user's assignment SHALL be marked as "claimed", all other pending assignments SHALL be marked as "claimed_by_other", and ticket.assignee_id SHALL be updated to the claiming user. Timeline SHALL record the claim.

#### Scenario: Claim among multiple assignees
- **WHEN** an activity has three pending assignments for users A, B, and C
- **WHEN** user B calls POST /api/v1/itsm/tickets/:id/activities/:aid/claim
- **THEN** user B's assignment status is set to "claimed"
- **THEN** user A's and user C's assignment statuses are set to "claimed_by_other"
- **THEN** ticket.assignee_id is updated to user B
- **THEN** a timeline entry records "user B claimed the task"

### Requirement: Authorization for transfer and delegate

Only the current assignee of the activity or a user with admin role SHALL be authorized to perform transfer and delegate operations. Any other user SHALL receive HTTP 403 Forbidden.

### Requirement: Assignment model adds delegation and transfer fields

The Assignment model SHALL add the following fields: delegated_from (*uint) to track the original assignee during delegation, and transfer_from (*uint) to track the original assignee during transfer. Both fields SHALL be nullable.

### Requirement: Assignment model adds new statuses

The Assignment model SHALL support the following additional status values: "transferred" (original assignment after transfer), "delegated" (original assignment after delegation), "claimed" (assignment claimed by this user), and "claimed_by_other" (assignment cancelled because another user claimed). These are in addition to existing statuses such as "pending" and "completed".
