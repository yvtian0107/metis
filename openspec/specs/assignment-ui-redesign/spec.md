# Purpose

TBD

## Requirements

### Requirement: Department tree panel with member count and search
The assignment page left panel SHALL display the department tree with each node showing a member count badge, support keyword filtering, and provide clear visual feedback for the selected department.

#### Scenario: Display member count badges
- **WHEN** the assignment page loads
- **THEN** each department node in the tree displays a badge with the number of direct members

#### Scenario: Filter departments by keyword
- **WHEN** the user types a keyword in the department search input
- **THEN** the tree shows only matching departments and their ancestor chains, with all matching branches expanded

#### Scenario: Select a department
- **WHEN** the user clicks a department node
- **THEN** the node is visually highlighted (accent left border + background) and the right panel loads that department's members

### Requirement: Enhanced member table with rich user info
The right panel member table SHALL display user avatar, name, email in a two-line cell, a position column, a primary/secondary badge column, an assignment date column, and a dropdown actions menu.

#### Scenario: Display member with full info
- **WHEN** a department with members is selected
- **THEN** each row shows: avatar + name (line 1) + email (line 2) | position name | primary/secondary badge | assignment date | action menu

#### Scenario: Action menu options for non-primary member
- **WHEN** the user opens the action menu for a non-primary member
- **THEN** the menu shows: "Set as Primary", "Change Position", "View Org Info", "Remove from Department"

#### Scenario: Action menu options for primary member
- **WHEN** the user opens the action menu for the primary member
- **THEN** the menu shows: "Change Position", "View Org Info", "Remove from Department" (no "Set as Primary" since already primary)

### Requirement: Add member sheet with user search
The "Add Member" sheet SHALL provide a searchable user dropdown that shows avatar, name, and email, grays out users already in the current department, and includes position select and primary checkbox.

#### Scenario: Search and select user
- **WHEN** the user types in the user search field
- **THEN** a dropdown shows matching users with avatar, name, and email; users already assigned to this department are displayed as disabled with "(已分配)" label

#### Scenario: Submit add member
- **WHEN** the user selects a valid user, picks a position, and clicks confirm
- **THEN** the system calls `POST /api/v1/org/users/:id/positions` and refreshes the member list

### Requirement: User organization info sheet
The system SHALL provide a read-only Sheet that displays all department/position assignments for a given user.

#### Scenario: View user org info
- **WHEN** the user clicks "View Org Info" from the member action menu
- **THEN** a Sheet opens showing the user's name, email, and a list of all assignments (department / position / primary badge) fetched from `GET /api/v1/org/users/:id/positions`

### Requirement: Empty states
The assignment page SHALL display contextual empty states when no department is selected or when the selected department has no members.

#### Scenario: No department selected
- **WHEN** the assignment page loads and no department is selected
- **THEN** the right panel shows an icon and text: "请在左侧选择一个部门" with a subtitle "查看该部门下的人员分配"

#### Scenario: Selected department has no members
- **WHEN** a department with zero members is selected
- **THEN** the right panel shows an icon, text "该部门暂无成员", and a prominent "分配成员" button

### Requirement: Remove member with confirmation
Removing a member from a department SHALL require confirmation via AlertDialog.

#### Scenario: Confirm removal
- **WHEN** the user clicks "Remove from Department" and confirms in the AlertDialog
- **THEN** the system calls `DELETE /api/v1/org/users/:id/positions/:assignmentId`, refreshes the member list, and invalidates the department tree (to update member count)

### Requirement: Change position inline
The system SHALL allow changing a member's position within the current department via a Sheet.

#### Scenario: Change position
- **WHEN** the user clicks "Change Position" from the action menu
- **THEN** a Sheet opens with a position select pre-filled with the current position; on submit, the system calls `PUT /api/v1/org/users/:id/positions/:assignmentId` to update the position
