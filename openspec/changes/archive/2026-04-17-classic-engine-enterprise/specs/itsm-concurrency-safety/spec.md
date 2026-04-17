## ADDED Requirements

### Requirement: Progress acquires FOR UPDATE lock on Activity and ExecutionToken

Progress() SHALL acquire a FOR UPDATE row-level lock on both the Activity and its ExecutionToken before performing any status checks or state transitions. This prevents two concurrent callers from reading stale state and both attempting to advance the same activity.

#### Scenario: Concurrent approve by two users on the same activity
- **WHEN** two users simultaneously call Progress() on the same user_task activity
- **THEN** the first transaction acquires the FOR UPDATE lock on the Activity and ExecutionToken
- **THEN** the second transaction waits until the first transaction commits
- **THEN** only one user's approval is applied; the second sees the already-completed status and returns an error

### Requirement: tryCompleteJoin acquires FOR UPDATE lock on parent token

tryCompleteJoin() SHALL acquire a FOR UPDATE lock on the parent ExecutionToken before counting completed sibling tokens. This prevents two parallel branches completing simultaneously from both reading an incomplete sibling count and both attempting to activate the join node.

#### Scenario: Concurrent parallel branch completion at join gateway
- **WHEN** two parallel branches complete at the same instant, both calling tryCompleteJoin() for the same join gateway
- **THEN** the first transaction acquires the FOR UPDATE lock on the parent token
- **THEN** the second transaction waits until the first transaction commits
- **THEN** the join gateway activates exactly once after all sibling branches are complete

### Requirement: Timer task handlers acquire FOR UPDATE lock on activity

Timer task handlers (itsm-wait-timer and itsm-boundary-timer) SHALL acquire a FOR UPDATE lock on the target Activity before checking its status. This prevents a race between a timer firing and a user manually progressing the same activity.

#### Scenario: Concurrent timer fire with manual progress on the same activity
- **WHEN** a wait-timer fires at the same moment a user calls Progress() on the same activity
- **THEN** whichever transaction acquires the FOR UPDATE lock first proceeds
- **THEN** the second transaction waits and then observes the updated status
- **THEN** the second transaction aborts gracefully without corrupting workflow state

### Requirement: Locked row causes wait, not failure

When a locked row is already being modified by another transaction, the second transaction SHALL wait for the lock to be released rather than failing immediately. The engine MUST NOT use NOWAIT or SKIP LOCKED semantics for workflow state transitions.

#### Scenario: Second transaction waits on locked row
- **WHEN** transaction A holds a FOR UPDATE lock on an Activity row
- **WHEN** transaction B attempts to acquire a FOR UPDATE lock on the same Activity row
- **THEN** transaction B blocks until transaction A commits or rolls back
- **THEN** transaction B does not return a lock-conflict error
