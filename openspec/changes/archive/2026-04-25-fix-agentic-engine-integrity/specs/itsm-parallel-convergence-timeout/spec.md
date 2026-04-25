## ADDED Requirements

### Requirement: Parallel group convergence timeout detection
ensureContinuation() SHALL check whether a parallel activity group has exceeded its convergence timeout. Timeout value SHALL be determined in priority order: (1) ticket's SLA resolution_deadline if present, (2) EngineConfigProvider.ParallelConvergenceTimeout() if configured, (3) hardcoded fallback of 168 hours (7 days). The timeout window starts from the earliest activity creation time in the group.

#### Scenario: Parallel group within timeout
- **WHEN** ensureContinuation() checks a parallel group where all activities were created 2 hours ago and the convergence timeout is 72 hours
- **THEN** no timeout action is taken, continuation waits for remaining activities

#### Scenario: Parallel group exceeds timeout
- **WHEN** ensureContinuation() checks a parallel group where the earliest activity was created 73 hours ago and the convergence timeout is 72 hours
- **THEN** a timeout is detected and the itsm-smart-timeout task is submitted

#### Scenario: SLA deadline takes precedence for timeout
- **WHEN** a ticket has SLA resolution_deadline set to 4 hours from now and EngineConfigProvider returns 72h
- **THEN** the convergence timeout uses the SLA resolution_deadline (4 hours) instead of the configured 72h

### Requirement: Timeout action — cancel pending siblings
When parallel group convergence timeout fires, the system SHALL cancel all non-completed sibling activities in the group by setting their status to "cancelled" with reason "convergence_timeout". Completed activities' results SHALL be preserved. A timeline event SHALL be recorded with type "parallel_convergence_timeout".

#### Scenario: Timeout cancels pending activities
- **WHEN** a parallel group of 3 activities times out, where activity A is completed, activity B is in_progress, and activity C is pending
- **THEN** activities B and C are set to status "cancelled" with reason "convergence_timeout", and activity A's result is preserved

#### Scenario: Timeline records timeout event
- **WHEN** parallel convergence timeout fires for a group
- **THEN** a timeline event is recorded with type "parallel_convergence_timeout" including the group_id and list of cancelled activity IDs

### Requirement: Post-timeout continuation
After timeout cancellation completes, the system SHALL call ensureContinuation() to trigger the next decision cycle. The decision cycle SHALL receive context about the timeout via the completed_activity anchor in seed, including which activities completed and which were cancelled.

#### Scenario: Decision cycle after timeout
- **WHEN** parallel timeout cancels 2 of 3 activities and 1 completed with approval
- **THEN** ensureContinuation triggers a new decision cycle where the seed includes the approval outcome and notes which activities were cancelled due to timeout

#### Scenario: All activities cancelled by timeout
- **WHEN** parallel timeout fires and no activities in the group had completed
- **THEN** ensureContinuation triggers a decision cycle where the seed indicates all parallel activities timed out with no outcomes
