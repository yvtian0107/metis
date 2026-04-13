## ADDED Requirements

### Requirement: Assignments page MUST be decomposed into sub-components
The `assignments/index.tsx` page SHALL be split into independent sub-components: a department tree panel, a member list panel, and an add-member sheet. The page container SHALL coordinate shared state via props.

#### Scenario: Component file structure
- **WHEN** the assignments page is loaded
- **THEN** each sub-component (DepartmentTreePanel, MemberListPanel, AddMemberSheet) SHALL be in its own file under `pages/assignments/`

### Requirement: User picker MUST use Command component
The user selection UI in the add-member form SHALL use the shadcn/ui `Command` component (based on cmdk) instead of a hand-built Popover+Input+button list.

#### Scenario: Keyboard navigation
- **WHEN** the user picker is open and the user presses arrow down/up
- **THEN** the selection SHALL move between items with visible focus indicator

#### Scenario: Already assigned users
- **WHEN** the user picker shows users already assigned to the current department
- **THEN** those users SHALL be visually disabled and non-selectable

### Requirement: User search MUST be debounced
The user search input in the add-member sheet SHALL debounce API calls by at least 300ms.

#### Scenario: Rapid typing
- **WHEN** the user types "john" quickly (4 keystrokes within 200ms)
- **THEN** the system SHALL make at most 1 API request (after 300ms debounce), not 4

### Requirement: Position loading MUST NOT use pageSize=9999
The position list query SHALL use a dedicated "load all" pattern (e.g., `pageSize=0` convention) instead of an arbitrary large page size.

#### Scenario: Fetch all positions
- **WHEN** the assignments page loads positions for the dropdown
- **THEN** the API request SHALL use `pageSize=0` (meaning "return all") instead of `pageSize=9999`

### Requirement: Add Member form MUST use React Hook Form + Zod
The add-member form (user selection, position selection, isPrimary checkbox) SHALL use React Hook Form with Zod validation, consistent with all other forms in the project.

#### Scenario: Submit without selecting user
- **WHEN** the user clicks confirm without selecting a user
- **THEN** the form SHALL show a Zod validation error on the user field

### Requirement: Tree initialization MUST NOT use ref mutation in queryFn
The initial tree expansion state SHALL be set via a `useEffect` watching the query data, not by mutating a ref and calling setState inside the queryFn callback.

#### Scenario: First data load
- **WHEN** the department tree data is loaded for the first time
- **THEN** the system SHALL auto-expand the first 2 levels using a useEffect that runs when data changes from undefined to a value
