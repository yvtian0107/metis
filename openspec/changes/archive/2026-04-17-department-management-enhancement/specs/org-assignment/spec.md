## MODIFIED Requirements

### Requirement: Assign multiple positions to a single user
The system SHALL allow a user to hold multiple department/position combinations via a `UserPosition` association table. When adding or updating an assignment, the system SHALL validate that the position is allowed in the target department (if the department has allowed positions configured).

#### Scenario: Assign a primary and secondary position
- **WHEN** an admin assigns two positions to a user, marking one as primary
- **THEN** both associations are persisted and exactly one is marked primary

#### Scenario: Reject assignment with disallowed position
- **WHEN** an admin assigns a user to a department with a position that is not in the department's allowed positions list
- **THEN** the system responds with HTTP 400 and an error message indicating the position is not allowed in this department
