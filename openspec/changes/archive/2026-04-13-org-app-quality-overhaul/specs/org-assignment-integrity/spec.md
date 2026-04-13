## ADDED Requirements

### Requirement: Primary assignment operations MUST be atomic
All operations that modify primary status (add with isPrimary, update isPrimary, set primary) SHALL execute demote-old + set-new within a single database transaction. No intermediate state where zero or multiple primaries exist SHALL be observable by concurrent requests.

#### Scenario: Concurrent add with isPrimary=true
- **WHEN** two concurrent requests both call `AddUserPosition` with `isPrimary=true` for the same user
- **THEN** exactly one assignment SHALL be primary after both complete, and the other SHALL be non-primary

#### Scenario: Add first assignment auto-sets primary
- **WHEN** `AddUserPosition` is called for a user with no existing assignments and `isPrimary=false`
- **THEN** the system SHALL set `isPrimary=true` on the new assignment within the same transaction

#### Scenario: SetPrimary with non-existent assignmentId
- **WHEN** `SetPrimary` is called with an `assignmentId` that does not exist for the given user
- **THEN** the system SHALL return `ErrAssignmentNotFound` and SHALL NOT modify any existing assignments

### Requirement: AddUserPosition MUST validate foreign key entities
Before creating a user-position assignment, the service layer SHALL verify that the referenced department and position both exist and are active.

#### Scenario: Department does not exist
- **WHEN** `AddUserPosition` is called with a `deptID` that does not exist in the departments table
- **THEN** the system SHALL return `ErrDepartmentNotFound` (HTTP 400)

#### Scenario: Department is inactive
- **WHEN** `AddUserPosition` is called with a `deptID` referencing an inactive department
- **THEN** the system SHALL return an error indicating the department is inactive (HTTP 400)

#### Scenario: Position does not exist
- **WHEN** `AddUserPosition` is called with a `posID` that does not exist in the positions table
- **THEN** the system SHALL return `ErrPositionNotFound` (HTTP 400)

#### Scenario: Position is inactive
- **WHEN** `AddUserPosition` is called with a `posID` referencing an inactive position
- **THEN** the system SHALL return an error indicating the position is inactive (HTTP 400)

### Requirement: demoteCurrentPrimary errors MUST be propagated
The `demoteCurrentPrimary` internal method SHALL return errors to callers. Callers SHALL NOT discard the error with `_ =`. If demotion fails, the entire operation SHALL fail and return the error.

#### Scenario: DB connection fails during demote
- **WHEN** `demoteCurrentPrimary` encounters a database error
- **THEN** the calling method (`AddUserPosition` or `UpdateUserPosition`) SHALL return the error without creating/updating the assignment

### Requirement: RemoveUserPosition MUST return proper error for missing assignment
When removing a user position, the service layer SHALL distinguish between "assignment not found" and other database errors.

#### Scenario: Assignment does not exist
- **WHEN** `RemoveUserPosition` is called with an `assignmentID` that does not exist for the given user
- **THEN** the handler SHALL return HTTP 404 with `ErrAssignmentNotFound`

### Requirement: DepartmentService.Update MUST use struct input
The `DepartmentService.Update` method SHALL accept a struct parameter instead of individual pointer arguments for improved readability and maintainability.

#### Scenario: Partial update with struct
- **WHEN** `DepartmentService.Update` is called with an `UpdateDepartmentInput` struct where only `Name` and `IsActive` are set (non-nil)
- **THEN** only the `name` and `is_active` columns SHALL be updated in the database

### Requirement: GetUserDepartmentScope MUST use single-query loading
The scope expansion from user departments to all sub-departments SHALL load all active departments in a single query and perform BFS in memory, instead of issuing one query per tree level.

#### Scenario: 5-level deep department tree
- **WHEN** `GetUserDepartmentScope` is called for a user in a department with 5 levels of sub-departments
- **THEN** the system SHALL execute at most 2 SQL queries total (one for user's department IDs, one for all active departments)

### Requirement: CountAssignments MUST NOT use dynamic column names in SQL
The repository method for counting assignments SHALL use fixed column names or parameterized queries. Map keys SHALL NOT be concatenated into SQL strings.

#### Scenario: Count by department
- **WHEN** `CountAssignments` is called with a department filter
- **THEN** the SQL query SHALL use parameterized column references, not string-concatenated column names
