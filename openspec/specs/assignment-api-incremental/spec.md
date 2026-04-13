# Purpose

TBD

## Requirements

### Requirement: Add single position assignment
The system SHALL expose `POST /api/v1/org/users/:id/positions` to add a single department/position assignment to a user.

#### Scenario: Add assignment to user with no existing assignments
- **WHEN** a client calls `POST /api/v1/org/users/:id/positions` with `{ departmentId, positionId, isPrimary: true }`
- **THEN** the system creates the assignment and marks it as primary

#### Scenario: Add assignment to user with existing assignments
- **WHEN** a client calls `POST /api/v1/org/users/:id/positions` with `{ departmentId, positionId }` and the user already has assignments
- **THEN** the system creates the new assignment without affecting existing ones

#### Scenario: Add duplicate department assignment
- **WHEN** a client calls `POST /api/v1/org/users/:id/positions` with a departmentId where the user already has an assignment
- **THEN** the system returns 400 with an error indicating the user is already assigned to that department

#### Scenario: Add assignment with isPrimary flag
- **WHEN** a client calls `POST /api/v1/org/users/:id/positions` with `isPrimary: true` and the user already has a primary assignment
- **THEN** the system creates the new assignment as primary and demotes the previous primary to non-primary

### Requirement: Remove single position assignment
The system SHALL expose `DELETE /api/v1/org/users/:id/positions/:assignmentId` to remove a single assignment.

#### Scenario: Remove a non-primary assignment
- **WHEN** a client calls `DELETE /api/v1/org/users/:id/positions/:assignmentId` for a non-primary assignment
- **THEN** the system deletes the assignment and returns 200

#### Scenario: Remove a primary assignment when other assignments exist
- **WHEN** a client calls `DELETE /api/v1/org/users/:id/positions/:assignmentId` for a primary assignment and the user has other assignments
- **THEN** the system deletes the assignment and auto-promotes the next assignment (by sort order) to primary

#### Scenario: Remove the last assignment
- **WHEN** a client calls `DELETE /api/v1/org/users/:id/positions/:assignmentId` and it is the user's only assignment
- **THEN** the system deletes the assignment (user has no assignments)

#### Scenario: Remove assignment that doesn't belong to user
- **WHEN** a client calls `DELETE /api/v1/org/users/:id/positions/:assignmentId` with an assignmentId that doesn't belong to that user
- **THEN** the system returns 404

### Requirement: Department tree includes member count
The system SHALL include a `memberCount` field in each node of the `GET /api/v1/org/departments/tree` response.

#### Scenario: Tree with members
- **WHEN** a client calls `GET /api/v1/org/departments/tree` and departments have assigned members
- **THEN** each node in the tree includes `memberCount` reflecting the number of direct members (not including sub-department members)

#### Scenario: Tree with empty departments
- **WHEN** a client calls `GET /api/v1/org/departments/tree` and a department has no members
- **THEN** that node's `memberCount` is 0
