## ADDED Requirements

### Requirement: User direct manager field
The system SHALL add a `ManagerID *uint` self-referencing foreign key to the User model, pointing to the user's direct manager. The field SHALL be nullable (no manager assigned by default). The system SHALL prevent circular manager chains (A→B→A) by validating during update.

#### Scenario: Set user's direct manager
- **WHEN** PUT /api/v1/users/:id with `{managerId: 5}`
- **THEN** the system SHALL update the user's managerID to 5 and return the updated user

#### Scenario: Manager field in user response
- **WHEN** GET /api/v1/users/:id for a user with a manager assigned
- **THEN** the response SHALL include `manager: {id, username, avatar}` (nested object, not just ID)

#### Scenario: Clear manager assignment
- **WHEN** PUT /api/v1/users/:id with `{managerId: null}`
- **THEN** the system SHALL set managerID to null

#### Scenario: Circular chain prevention
- **WHEN** PUT /api/v1/users/:id with managerId that would create a cycle (e.g., A's manager is B, setting B's manager to A)
- **THEN** the system SHALL return 400 with message "circular manager chain detected"

#### Scenario: Manager chain traversal API
- **WHEN** GET /api/v1/users/:id/manager-chain
- **THEN** the system SHALL return an ordered list of managers from direct manager up to root, e.g., `[{id:5,username:"李四"},{id:2,username:"王五"}]`, stopping at nil or max depth of 10

### Requirement: Manager field in user management UI
The system SHALL display and allow editing the direct manager field in the user create/edit Sheet.

#### Scenario: Manager selector in user form
- **WHEN** admin opens create or edit user Sheet
- **THEN** the form SHALL include a searchable user selector for "直属上级" that shows user avatar and username

#### Scenario: Manager displayed in user list
- **WHEN** user navigates to /users
- **THEN** the user table SHALL display the direct manager's name (or "—" if none) in a "直属上级" column
