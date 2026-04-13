# Purpose

Define the department management UI for viewing and editing the department tree.

## Requirements

### Requirement: Department management page with tree display
The system SHALL provide a page at `/org/departments` that displays departments in a tree-structured table with create, edit, and delete actions.

#### Scenario: View department tree
- **WHEN** an authorized user navigates to `/org/departments`
- **THEN** the page renders a table showing departments in hierarchical order with indentation

### Requirement: Department form uses a Sheet with parent department selection
The system SHALL open a Sheet drawer for creating or editing a department, including a TreeSelect for choosing the parent department.

#### Scenario: Create a sub-department
- **WHEN** an admin clicks "新增部门" and selects a parent department from the TreeSelect
- **THEN** the form submits successfully and the new department appears in the tree table

### Requirement: Delete confirmation for departments
The system SHALL display a confirmation dialog before deleting a department and show an error toast if deletion is blocked.

#### Scenario: Attempt to delete department with members
- **WHEN** an admin tries to delete a department that still has members
- **THEN** the backend rejects the request and the UI displays a toast with the error message
