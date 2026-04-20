## Purpose

ITSM ClassicEngine workflow engine -- token-based BPMN-style execution for classic service processes.

## Requirements

### Requirement: Progress method advances workflow

ClassicEngine.Progress() SHALL acquire a FOR UPDATE lock on the activity row and its associated execution token row before performing status checks. If the activity type is "approve" and execution_mode is "parallel" or "sequential", Progress SHALL delegate to the multi-approval logic (progressApproval) instead of immediately completing the activity. For single mode or non-approve activities, behavior is unchanged. When creating the next activity, the engine SHALL read form schema from the workflow node's inline `formSchema` field. The engine SHALL NOT query FormDefinition table. When writing form bindings from a completed activity, the engine SHALL parse the schema from `activity.FormSchema` (already snapshotted) as before.

#### Scenario: progress with FOR UPDATE lock acquired

- WHEN Progress() is called for an activity
- THEN the system SHALL issue SELECT ... FOR UPDATE on the activity row
- AND the system SHALL issue SELECT ... FOR UPDATE on the associated execution token row
- THEN status checks and state transitions proceed under the acquired locks

#### Scenario: progress approve-parallel delegates to multi-approval

- WHEN Progress() is called for an activity with type "approve" and execution_mode "parallel"
- THEN Progress SHALL delegate to progressApproval() instead of immediately completing the activity
- AND progressApproval SHALL evaluate whether all parallel assignments are resolved before completing the activity

#### Scenario: progress approve-sequential delegates to multi-approval

- WHEN Progress() is called for an activity with type "approve" and execution_mode "sequential"
- THEN Progress SHALL delegate to progressApproval() instead of immediately completing the activity
- AND progressApproval SHALL evaluate whether the current sequential assignment is resolved and advance to the next

#### Scenario: progress approve-single unchanged

- WHEN Progress() is called for an activity with type "approve" and execution_mode "single" (or empty)
- THEN Progress SHALL complete the activity immediately upon the first participant action
- AND behavior MUST remain identical to the existing single-approval flow

#### Scenario: Activity creation reads inline formSchema

- WHEN the engine creates an activity for a form/user_task node
- THEN it SHALL copy `node.FormSchema` directly into `activity.FormSchema`
- AND it SHALL NOT perform any database query to resolve the form

#### Scenario: Form binding write unchanged

- WHEN a user completes an activity with form data
- THEN the engine SHALL parse bindings from `activity.FormSchema` and write process variables
- AND the binding behavior SHALL be identical to the current implementation

### Requirement: Classic engine code organization
The `classic.go` monolithic file SHALL be split into multiple files by responsibility. All functions SHALL remain in the `engine` package with unchanged signatures and behavior.

#### Scenario: File split does not change behavior

- WHEN classic engine files are reorganized
- THEN all existing unit tests and BDD tests SHALL pass without modification

#### Scenario: File organization by responsibility

- WHEN the split is complete
- THEN the files SHALL be organized as:
  - `classic_core.go` -- Start/Progress/Cancel entry points and graph traversal
  - `classic_nodes.go` -- Per-node-type processing functions
  - `classic_activity.go` -- Activity creation, update, and query helpers
  - `classic_token.go` -- ExecutionToken tree operations
  - `classic_notify.go` -- Notification dispatch logic
  - `classic_helpers.go` -- Type aliases, JSON helpers, and small utility functions

### Requirement: Timer task returns ErrNotReady when not yet due

When itsm-wait-timer or itsm-boundary-timer task handler determines that execute_after time has not been reached, it SHALL return scheduler.ErrNotReady instead of nil. The scheduler SHALL recognize ErrNotReady and retain the task for the next poll cycle without marking it as completed or failed.

#### Scenario: timer not yet due returns ErrNotReady

- WHEN the itsm-wait-timer or itsm-boundary-timer handler executes
- AND the current time is before the task's execute_after timestamp
- THEN the handler SHALL return scheduler.ErrNotReady

#### Scenario: scheduler retains ErrNotReady task

- WHEN a task handler returns scheduler.ErrNotReady
- THEN the scheduler SHALL NOT mark the task as completed or failed
- AND the scheduler SHALL retain the task for the next poll cycle

#### Scenario: timer due executes normally

- WHEN the itsm-wait-timer or itsm-boundary-timer handler executes
- AND the current time is at or past the task's execute_after timestamp
- THEN the handler SHALL proceed with normal timer execution logic
- AND return nil on success
