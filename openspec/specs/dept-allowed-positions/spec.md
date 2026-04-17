# Purpose

Define the association model and APIs for restricting which positions are allowed in a given department, and enforce this constraint during user-position assignment.

## Requirements

### Requirement: DepartmentPosition association model
The system SHALL maintain a `department_positions` table with `department_id` and `position_id` columns, enforcing a composite unique index on `(department_id, position_id)`.

#### Scenario: Create department-position association
- **WHEN** an admin sets allowed positions for a department
- **THEN** the system persists the associations in the `department_positions` table with correct foreign key references

### Requirement: Batch set allowed positions API
The system SHALL expose `PUT /api/v1/org/departments/:id/positions` accepting `{ "positionIds": [1, 2, 3] }` to fully replace the department's allowed positions in a single transaction.

#### Scenario: Set allowed positions for a department
- **WHEN** a client calls `PUT /api/v1/org/departments/:id/positions` with `positionIds: [1, 2]`
- **THEN** the system deletes all existing associations for that department and creates new ones for position 1 and 2

#### Scenario: Clear allowed positions
- **WHEN** a client calls `PUT /api/v1/org/departments/:id/positions` with `positionIds: []`
- **THEN** all associations for that department are removed, meaning no position restriction is enforced

### Requirement: Query allowed positions API
The system SHALL expose `GET /api/v1/org/departments/:id/positions` returning the list of allowed positions for a department.

#### Scenario: Get allowed positions
- **WHEN** a client calls `GET /api/v1/org/departments/:id/positions`
- **THEN** the system returns an array of `PositionResponse` objects representing the allowed positions

#### Scenario: Department has no allowed positions configured
- **WHEN** a client calls `GET /api/v1/org/departments/:id/positions` for a department with no associations
- **THEN** the system returns an empty array

### Requirement: Assignment validation against allowed positions
The system SHALL validate that the assigned position is in the department's allowed positions list when adding or updating a user position assignment. If the department has no allowed positions configured (empty list), the system SHALL skip validation.

#### Scenario: Assign user with valid allowed position
- **WHEN** an admin assigns a user to department A with position P, and P is in department A's allowed positions
- **THEN** the assignment succeeds

#### Scenario: Assign user with invalid position
- **WHEN** an admin assigns a user to department A with position P, and department A has allowed positions configured but P is not among them
- **THEN** the system responds with HTTP 400 and error message indicating the position is not allowed in this department

#### Scenario: Assign user when department has no allowed positions configured
- **WHEN** an admin assigns a user to department B with any active position, and department B has no allowed positions configured
- **THEN** the assignment succeeds without position restriction
