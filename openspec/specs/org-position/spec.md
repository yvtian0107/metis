# Purpose

Define the backend API and business rules for managing positions (job titles) within the organization module.

## Requirements

### Requirement: Position dictionary management API
The system SHALL expose REST endpoints for creating, listing, retrieving, updating, and deleting positions.

#### Scenario: List paginated positions
- **WHEN** a client calls `GET /api/v1/org/positions?page=1&pageSize=20`
- **THEN** the system returns a paginated list of positions ordered by `sort` and `level`

#### Scenario: Prevent deletion of an in-use position
- **WHEN** an admin attempts to delete a position that is still referenced by at least one user assignment
- **THEN** the system responds with HTTP 400 and an error message indicating the position is in use

### Requirement: Position uniqueness and validation
The system SHALL enforce that `code` is unique per position and that `name` is required.

#### Scenario: Duplicate position code
- **WHEN** an admin creates a position with a code that already exists
- **THEN** the system responds with HTTP 400 indicating the code is duplicated

### Requirement: Position status control
The system SHALL allow positions to be active or inactive, and only active positions SHALL be selectable during personnel assignment.

#### Scenario: Hide inactive positions in assignment
- **WHEN** the personnel assignment UI queries available positions
- **THEN** only positions with `isActive=true` are returned
