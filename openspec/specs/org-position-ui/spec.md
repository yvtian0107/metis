# Purpose

Define the position management UI for viewing and editing the position dictionary.

## Requirements

### Requirement: Position management page with DataTable
The system SHALL provide a page at `/org/positions` that lists all positions in a paginated DataTable with search, create, edit, and delete actions.

#### Scenario: View and search positions
- **WHEN** an authorized user navigates to `/org/positions`
- **THEN** the page renders a paginated table of positions and a search input filters by name or code

### Requirement: Position form uses a Sheet drawer
The system SHALL open a Sheet drawer for creating or editing a position, containing fields for name, code, level, description, and active status.

#### Scenario: Edit position level
- **WHEN** an admin opens a position for editing and changes its level
- **THEN** the form submits successfully and the updated level appears in the table

### Requirement: Delete confirmation for positions
The system SHALL display a confirmation dialog before deleting a position and show an error toast if the position is still in use.

#### Scenario: Attempt to delete an assigned position
- **WHEN** an admin tries to delete a position assigned to a user
- **THEN** the backend rejects the request and the UI displays a toast with the error message
