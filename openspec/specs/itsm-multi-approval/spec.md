## ADDED Requirements

### Requirement: Single approval mode completes on first action

Single mode: when execution_mode is "single", the first participant to approve or reject SHALL complete the activity immediately. All other pending assignments SHALL be marked as cancelled.

#### Scenario: Single mode — first approve completes activity
- **WHEN** an approval activity is created with execution_mode="single" and three assigned participants
- **WHEN** the first participant submits an "approve" action
- **THEN** the activity completes with outcome "approve"
- **THEN** the remaining two pending assignments are marked as cancelled

### Requirement: Parallel approval mode requires all participants

Parallel (会签) mode: when execution_mode is "parallel", the activity SHALL complete only when ALL assignments are resolved. If any participant rejects, the activity SHALL immediately complete with outcome "reject". If all participants approve, the activity SHALL complete with outcome "approve".

#### Scenario: Parallel mode — all approve
- **WHEN** an approval activity is created with execution_mode="parallel" and three assigned participants
- **WHEN** all three participants submit "approve"
- **THEN** the activity completes with outcome "approve"

#### Scenario: Parallel mode — one reject triggers immediate completion
- **WHEN** an approval activity is created with execution_mode="parallel" and three assigned participants
- **WHEN** the first participant submits "approve"
- **WHEN** the second participant submits "reject"
- **THEN** the activity immediately completes with outcome "reject"
- **THEN** the third participant's pending assignment is marked as cancelled

### Requirement: Sequential approval mode progresses one at a time

Sequential (依次) mode: when execution_mode is "sequential", assignments SHALL progress one at a time in sequence order. Only the assignment with is_current=true SHALL be allowed to act. After the current assignment completes, is_current SHALL move to the next assignment in sequence. The last assignment's completion SHALL trigger activity completion.

#### Scenario: Sequential mode — chain progression
- **WHEN** an approval activity is created with execution_mode="sequential" and three assigned participants (seq 1, 2, 3)
- **THEN** only participant 1's assignment has is_current=true
- **WHEN** participant 1 submits "approve"
- **THEN** participant 1's assignment is completed and participant 2's assignment becomes is_current=true
- **WHEN** participant 2 submits "approve"
- **THEN** participant 2's assignment is completed and participant 3's assignment becomes is_current=true

#### Scenario: Sequential mode — last assignment completes the activity
- **WHEN** an approval activity is in sequential mode with three participants
- **WHEN** participants 1 and 2 have already approved
- **WHEN** participant 3 (the last) submits "approve"
- **THEN** the activity completes with outcome "approve"

### Requirement: Progress delegates to mode-specific logic

Progress() SHALL check the activity's execution_mode field and delegate to the corresponding mode-specific completion logic. The engine MUST support "single", "parallel", and "sequential" modes. An unrecognized execution_mode SHALL return an error.

#### Scenario: Progress dispatches to correct mode handler
- **WHEN** Progress() is called on an activity with execution_mode="parallel"
- **THEN** the parallel-mode completion logic is invoked
- **THEN** the single-mode and sequential-mode logic are not invoked
