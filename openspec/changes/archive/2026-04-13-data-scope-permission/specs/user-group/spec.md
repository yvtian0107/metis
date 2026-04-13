## ADDED Requirements

### Requirement: UserGroup model
The system SHALL provide a `UserGroup` model in the Org App with fields: `id`, `name`, `code` (unique), `description`, `isActive`. UserGroup represents a flat, non-hierarchical team (e.g., on-call group, change advisory board) distinct from the department tree.

#### Scenario: Create user group
- **WHEN** POST /api/v1/org/groups with `{name: "运维 On-Call 组", code: "oncall-ops"}`
- **THEN** the system SHALL create the group and return it with `isActive: true`

#### Scenario: Duplicate group code
- **WHEN** POST /api/v1/org/groups with a code that already exists
- **THEN** the system SHALL return 400 with message "group code already exists"

#### Scenario: List groups
- **WHEN** GET /api/v1/org/groups
- **THEN** the system SHALL return paginated group list with member count per group

### Requirement: UserGroupMember model
The system SHALL provide a `UserGroupMember` join model linking UserGroup and User. A user MAY belong to multiple groups. The same user SHALL NOT appear twice in the same group.

#### Scenario: Add member to group
- **WHEN** POST /api/v1/org/groups/:id/members with `{userIds: [1, 2, 3]}`
- **THEN** the system SHALL add the specified users to the group, skipping already-present members

#### Scenario: Remove member from group
- **WHEN** DELETE /api/v1/org/groups/:id/members/:userId
- **THEN** the system SHALL remove the user from the group

#### Scenario: List group members
- **WHEN** GET /api/v1/org/groups/:id/members
- **THEN** the system SHALL return paginated member list with user avatar, username, email, and primary department

#### Scenario: Get user's groups
- **WHEN** GET /api/v1/org/users/:id/groups
- **THEN** the system SHALL return all groups the user belongs to

### Requirement: UserGroup management UI
The system SHALL provide a group management page in the Org App at `/org/groups`.

#### Scenario: View group list
- **WHEN** user navigates to /org/groups
- **THEN** the page SHALL display a table of groups with columns: name, code, member count, status, actions

#### Scenario: Manage group members
- **WHEN** user clicks "管理成员" on a group row
- **THEN** a Sheet SHALL open showing current members and a user search/add interface

#### Scenario: Group seeded in Org App
- **WHEN** the Org App is installed (seed runs)
- **THEN** the system SHALL create menu entries for "用户组" under 组织管理 directory, and add corresponding Casbin policies for the admin role
