# Delta Spec: itsm-execution-token

> Capability: itsm-execution-token
> Change: classic-engine-enterprise
> Type: MODIFIED

## MODIFIED Requirements

### Requirement: ExecutionToken query with locking

When querying ExecutionToken rows during Progress() or tryCompleteJoin(), the system SHALL use SELECT ... FOR UPDATE to prevent concurrent modification. The join completion check (counting remaining active siblings) SHALL be performed after acquiring the parent token lock to ensure atomic join evaluation.

#### Scenario: token locked during progress

- WHEN Progress() queries an ExecutionToken associated with the current activity
- THEN the system SHALL use SELECT ... FOR UPDATE on the token row
- AND no other transaction SHALL be able to modify that token until the lock is released

#### Scenario: parent token locked during join check

- WHEN tryCompleteJoin() evaluates whether a parallel gateway join is complete
- THEN the system SHALL acquire a FOR UPDATE lock on the parent token row before counting remaining active sibling tokens
- AND the sibling count SHALL reflect a consistent snapshot under the parent lock

#### Scenario: concurrent join attempts serialized

- WHEN two parallel branches complete simultaneously and both invoke tryCompleteJoin()
- THEN the FOR UPDATE lock on the parent token SHALL serialize the two join evaluations
- AND exactly one of the two transactions SHALL observe zero remaining active siblings and complete the join
- AND the other transaction SHALL observe the already-completed join and take no further action
