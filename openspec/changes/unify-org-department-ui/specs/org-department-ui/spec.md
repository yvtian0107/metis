## MODIFIED Requirements

### Requirement: Department management page with tree display
The system SHALL provide a page at `/org/departments` that displays departments in a tree-structured table with columns: name (with indentation and expand/collapse chevron), code, manager name, member count badge, status, and a navigation arrow. Clicking the chevron area SHALL expand/collapse the subtree. Clicking the row (outside the chevron) SHALL navigate to `/org/departments/:id`. The page title SHALL be "组织架构" (Organization Structure).

#### Scenario: View department tree with member counts
- **WHEN** an authorized user navigates to `/org/departments`
- **THEN** the page renders a tree table showing departments with name, code, manager name, member count, status, and a navigation indicator per row

#### Scenario: Navigate to department detail by clicking row
- **WHEN** a user clicks on a department row (not on the chevron)
- **THEN** the browser navigates to `/org/departments/:id`

#### Scenario: Expand/collapse subtree
- **WHEN** a user clicks the chevron icon on a department with children
- **THEN** the child departments expand or collapse without navigation

### Requirement: Department form uses a Sheet with parent department selection
The system SHALL open a Sheet drawer for creating or editing a department. The form SHALL include: name, code, parent department TreeSelect, manager user selector (from global user list), and description. The form SHALL NOT include allowed positions selection (moved to detail page) or a sort field.

#### Scenario: Create a sub-department
- **WHEN** an admin clicks "新增部门", selects a parent department, picks a manager user, and submits
- **THEN** the form submits successfully, the new department appears in the tree table

#### Scenario: Edit department basic info
- **WHEN** an admin edits an existing department via the Sheet
- **THEN** only basic fields (name, code, parent, manager, description) are editable; allowed positions are managed separately on the detail page

### Requirement: Delete confirmation for departments
The system SHALL display a confirmation dialog before deleting a department and show an error toast if deletion is blocked.

#### Scenario: Attempt to delete department with members
- **WHEN** an admin tries to delete a department that still has members
- **THEN** the backend rejects the request and the UI displays a toast with the error message
