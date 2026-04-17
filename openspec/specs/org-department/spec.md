# Purpose

Define the backend API and data model for managing departments as a self-referential tree structure.

## Requirements

### Requirement: Department model supports self-referential tree structure
The system SHALL model departments with a self-referencing parent relationship, allowing unlimited depth.

#### Scenario: Create a sub-department
- **WHEN** an admin creates a department with a valid `parentId` pointing to an existing department
- **THEN** the new department is persisted with the correct parent reference

### Requirement: Department CRUD API
The system SHALL expose REST endpoints for creating, listing, retrieving, updating, and deleting departments.

#### Scenario: List departments with tree endpoint
- **WHEN** a client calls `GET /api/v1/org/departments/tree`
- **THEN** the system returns a hierarchical tree of all active departments, each node including `managerName` (the username of the manager) resolved via LEFT JOIN on the users table

#### Scenario: Prevent deletion of department with members
- **WHEN** an admin attempts to delete a department that still has associated user positions
- **THEN** the system responds with HTTP 400 and an error message indicating the department is in use

### Requirement: Department uniqueness and validation
The system SHALL enforce that `code` is unique per department and that `name` is required.

#### Scenario: Duplicate department code
- **WHEN** an admin creates a department with a code that already exists
- **THEN** the system responds with HTTP 400 indicating the code is duplicated
