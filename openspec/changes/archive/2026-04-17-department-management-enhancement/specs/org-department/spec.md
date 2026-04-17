## MODIFIED Requirements

### Requirement: Department CRUD API
The system SHALL expose REST endpoints for creating, listing, retrieving, updating, and deleting departments.

#### Scenario: List departments with tree endpoint
- **WHEN** a client calls `GET /api/v1/org/departments/tree`
- **THEN** the system returns a hierarchical tree of all active departments, each node including `managerName` (the username of the manager) resolved via LEFT JOIN on the users table

#### Scenario: Prevent deletion of department with members
- **WHEN** an admin attempts to delete a department that still has associated user positions
- **THEN** the system responds with HTTP 400 and an error message indicating the department is in use
