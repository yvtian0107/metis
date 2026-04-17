## ADDED Requirements

### Requirement: Department detail page with info card
The system SHALL provide a detail page at `/org/departments/:id` that displays a department's full information in a card layout: name, code, status indicator, manager name, parent department name, member count, description, and creation time. The card SHALL follow DESIGN.md's detail page info card pattern with a brand-color stripe at the top.

#### Scenario: View department detail
- **WHEN** an authorized user navigates to `/org/departments/:id`
- **THEN** the page displays the department's info card with all fields populated from `GET /api/v1/org/departments/:id`

#### Scenario: Navigate back to department list
- **WHEN** the user clicks the back button on the detail page
- **THEN** the browser navigates to `/org/departments`

### Requirement: Edit department basic info from detail page
The system SHALL provide an "Edit" button on the detail page info card that opens the existing DepartmentSheet for editing the department's basic information (name, code, parent, manager, description).

#### Scenario: Edit department from detail page
- **WHEN** an admin with `org:department:update` permission clicks "Edit" on the info card
- **THEN** the DepartmentSheet opens pre-filled with the current department data, and on save the info card refreshes

### Requirement: Allowed positions management section
The system SHALL display the department's allowed positions as chips below the info card, with a "Manage" button that opens a popover for adding/removing positions. The popover SHALL use a Command (searchable list) with checkboxes, matching the existing position multi-select pattern.

#### Scenario: View allowed positions
- **WHEN** a user views a department detail page
- **THEN** the allowed positions section displays all positions from `GET /api/v1/org/departments/:id/positions` as chips

#### Scenario: Add an allowed position
- **WHEN** an admin with `org:department:update` permission clicks "Manage", selects a new position, and the popover closes
- **THEN** the system calls `PUT /api/v1/org/departments/:id/positions` with the updated position ID list and the chips refresh

#### Scenario: Remove an allowed position
- **WHEN** an admin clicks the remove (X) button on a position chip
- **THEN** the system calls `PUT /api/v1/org/departments/:id/positions` with that position removed and the chip disappears

### Requirement: Department member list section
The system SHALL display the department's members in a table within the detail page, with columns: user (avatar + username + email), position badges (with primary star), assigned date, and actions. The member list SHALL support pagination and keyword search. The table SHALL use the same visual patterns as the current assignments member-list.

#### Scenario: View department members
- **WHEN** a user views a department detail page
- **THEN** the member section displays members from `GET /api/v1/org/users?departmentId=:id` with pagination

#### Scenario: Search members
- **WHEN** a user types a keyword in the member search input and submits
- **THEN** the member list filters by the keyword parameter

### Requirement: Add member to department from detail page
The system SHALL provide an "Add Member" button in the member section that opens the AddMemberSheet, pre-configured for the current department.

#### Scenario: Add a member
- **WHEN** an admin with `org:assignment:create` permission clicks "Add Member", selects a user and positions, and confirms
- **THEN** the new member appears in the member list and the info card's member count updates

### Requirement: Member actions from detail page
The system SHALL provide a dropdown menu on each member row with actions: edit positions (opens EditPositionsSheet), view organization info (opens UserOrgSheet), and remove from department (with confirmation dialog). Actions SHALL respect `org:assignment:update` and `org:assignment:delete` permissions.

#### Scenario: Edit member positions
- **WHEN** an admin clicks "Edit Positions" on a member row
- **THEN** the EditPositionsSheet opens with the member's current positions pre-selected

#### Scenario: Remove member from department
- **WHEN** an admin clicks "Remove" on a member row and confirms
- **THEN** the member is removed from the department and the member count updates

### Requirement: Sub-departments section
The system SHALL display the department's direct child departments as a list at the bottom of the detail page. Each child SHALL show name, code, member count, and manager name. Clicking a child SHALL navigate to its detail page.

#### Scenario: Navigate to child department
- **WHEN** a user clicks on a child department row
- **THEN** the browser navigates to `/org/departments/:childId`

#### Scenario: Department with no children
- **WHEN** a department has no child departments
- **THEN** the sub-departments section is not rendered

### Requirement: Delete department from detail page
The system SHALL provide a delete action in the info card's overflow menu (⋯). Deletion SHALL show a confirmation dialog and navigate back to the department list on success.

#### Scenario: Delete department from detail
- **WHEN** an admin with `org:department:delete` permission clicks delete and confirms
- **THEN** the department is deleted and the user is navigated to `/org/departments`
