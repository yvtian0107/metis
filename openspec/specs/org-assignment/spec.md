# Purpose

TBD

## Requirements

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
