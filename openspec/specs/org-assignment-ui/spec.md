# Purpose

Define the personnel assignment UI for managing user department/position assignments from a department-centric view.

## Requirements

### Requirement: Personnel assignment page with department-centric view
The system SHALL provide a page at `/org/assignments` with a department tree on the left and a member list on the right.

#### Scenario: View members by department
- **WHEN** an authorized user selects a department from the left tree
- **THEN** the right panel displays all users assigned to that department along with their positions and primary status

### Requirement: Add and remove members from a department
The system SHALL allow admins to assign users to the selected department and remove existing assignments.

#### Scenario: Assign a user to a department
- **WHEN** an admin clicks "添加成员", selects a user, chooses a position, and confirms
- **THEN** the user appears in the department’s member list with the assigned position

#### Scenario: Remove a user from a department
- **WHEN** an admin clicks the remove action on a member row
- **THEN** the assignment is deleted and the user no longer appears in that department’s member list

### Requirement: Toggle primary position in member list
The system SHALL allow admins to set a user’s primary position directly from the member list.

#### Scenario: Set primary position
- **WHEN** an admin clicks the primary badge/action on a member row
- **THEN** that assignment becomes the user’s primary position and any previous primary is demoted
