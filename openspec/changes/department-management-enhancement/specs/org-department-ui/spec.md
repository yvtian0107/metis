## MODIFIED Requirements

### Requirement: Department management page with tree display
The system SHALL provide a page at `/org/departments` that displays departments in a tree-structured table with create, edit, and delete actions.

#### Scenario: View department tree
- **WHEN** an authorized user navigates to `/org/departments`
- **THEN** the page renders a table showing departments in hierarchical order with columns: name (with indentation), code, manager name, status, and actions

### Requirement: Department form uses a Sheet with parent department selection
The system SHALL open a Sheet drawer for creating or editing a department. The form SHALL include: name, code, parent department TreeSelect, manager user selector (from global user list), allowed positions multi-select (from active positions list), and description. The form SHALL NOT include a sort field.

#### Scenario: Create a sub-department with manager and allowed positions
- **WHEN** an admin clicks "新增部门", selects a parent department, picks a manager user, selects allowed positions, and submits
- **THEN** the form submits successfully, the new department appears in the tree table with the selected manager displayed

#### Scenario: Edit department allowed positions
- **WHEN** an admin edits an existing department and changes the allowed positions selection
- **THEN** the system calls `PUT /api/v1/org/departments/:id/positions` with the updated position IDs
