# Purpose

Define backend APIs and business rules for assigning users to departments and positions, including primary position enforcement.

## Requirements

### Requirement: Assign multiple positions to a single user
The system SHALL allow a user to hold multiple department/position combinations via a `UserPosition` association table. When adding or updating an assignment, the system SHALL validate that the position is allowed in the target department (if the department has allowed positions configured).

#### Scenario: Assign a primary and secondary position
- **WHEN** an admin assigns two positions to a user, marking one as primary
- **THEN** both associations are persisted and exactly one is marked primary

#### Scenario: Reject assignment with disallowed position
- **WHEN** an admin assigns a user to a department with a position that is not in the department's allowed positions list
- **THEN** the system responds with HTTP 400 and an error message indicating the position is not allowed in this department

### Requirement: Enforce single primary position per user
The system SHALL guarantee that a user has at most one primary position at any time.

#### Scenario: Switch primary position
- **WHEN** an admin assigns a new primary position to a user who already has one
- **THEN** the previous primary position is automatically demoted to non-primary and the new one becomes primary

### Requirement: Personnel assignment API
The system SHALL expose endpoints to retrieve, add, and remove a user's position assignments. The batch replace endpoint is removed in favor of incremental operations.

#### Scenario: Get user positions
- **WHEN** a client calls `GET /api/v1/org/users/:id/positions`
- **THEN** the system returns the full list of departments and positions assigned to that user

#### Scenario: Update single assignment
- **WHEN** a client calls `PUT /api/v1/org/users/:id/positions/:assignmentId` with updated fields (positionId, isPrimary)
- **THEN** the system updates only the specified assignment

### Requirement: Scope helper for department-based data filtering
The system SHALL provide a service helper to retrieve all **active** department IDs accessible to a user, including active sub-departments.

#### Scenario: Retrieve user department scope
- **WHEN** a module requests the department scope for a user with a primary position in a parent department
- **THEN** the helper returns the parent department ID and all its descendant department IDs, excluding departments where `IsActive = false`

#### Scenario: Scope excludes inactive departments
- **WHEN** a user is assigned to department A (active) which has child B (inactive) and grandchild C (active, under B)
- **THEN** the scope returns only department A (B is excluded because inactive; C is excluded because its parent B is inactive and BFS stops traversal at inactive nodes)
