## ADDED Requirements

### Requirement: FormRenderer component interface
The system SHALL provide a `<FormRenderer>` React component with the following props: `schema` (FormSchema object, required), `data` (Record<string, any>, optional existing values), `mode` ("create"|"edit"|"view", required), `nodeId` (string, optional workflow node ID for permissions), `onSubmit` (callback, optional), `onChange` (callback, optional), `disabled` (boolean, optional global disable).

#### Scenario: Create mode rendering
- **WHEN** FormRenderer is rendered with mode="create" and a schema containing 5 fields
- **THEN** all 5 fields SHALL render as editable with their defaultValue pre-filled

#### Scenario: View mode rendering
- **WHEN** FormRenderer is rendered with mode="view" and data containing submitted values
- **THEN** all fields SHALL render as read-only displaying the provided data values

#### Scenario: Edit mode with permissions
- **WHEN** FormRenderer is rendered with mode="edit", nodeId="node_1", and a field has permissions `{"node_1": "readonly"}`
- **THEN** that field SHALL render as read-only while other fields remain editable

### Requirement: Field type rendering
FormRenderer SHALL map each FormSchema field type to a UI component. Type mappings: text→Input, textarea→Textarea, number→Input[type=number], email→Input[type=email], url→Input[type=url], select→Select, multi_select→Combobox(multi), radio→RadioGroup, checkbox→CheckboxGroup, switch→Switch, date→DatePicker, datetime→DateTimePicker, date_range→DateRangePicker, user_picker→Combobox with user search API, dept_picker→TreeSelect with org department tree API, rich_text→Textarea. Unknown types SHALL render as Input with a warning indicator.

user_picker SHALL render as a Combobox component that searches users via `GET /api/v1/users?keyword=xxx&limit=10` with 300ms debounce. The value stored SHALL be the user ID (number). In readonly/view mode, user_picker SHALL display the user's name resolved from the ID. If the Org App is unavailable, user_picker SHALL fall back to a text Input with a warning tooltip indicating manual ID entry.

dept_picker SHALL render as a TreeSelect component that loads the department tree via `GET /api/v1/org/departments/tree`. The value stored SHALL be the department ID (number). In readonly/view mode, dept_picker SHALL display the department name resolved from the ID. If the Org App is unavailable, dept_picker SHALL fall back to a text Input with a warning tooltip indicating manual ID entry.

#### Scenario: User picker renders with search
- **WHEN** a user_picker field is rendered in create or edit mode
- **THEN** a Combobox is shown that searches users via API with 300ms debounce, stores user ID as value, and displays user name as label

#### Scenario: User picker in view mode shows name
- **WHEN** a user_picker field is rendered in view mode with value 42
- **THEN** the component resolves user ID 42 to display the user's name

#### Scenario: User picker fallback when Org App unavailable
- **WHEN** a user_picker field is rendered and the user search API returns 503
- **THEN** the component falls back to a text Input with a warning tooltip

#### Scenario: Department picker renders tree
- **WHEN** a dept_picker field is rendered in create or edit mode
- **THEN** a TreeSelect is shown with the organization department hierarchy

#### Scenario: Department picker fallback when Org App unavailable
- **WHEN** a dept_picker field is rendered and the department tree API returns 503
- **THEN** the component falls back to a text Input with a warning tooltip

#### Scenario: Unknown field type fallback
- **WHEN** a field with an unrecognized type is encountered
- **THEN** the renderer SHALL display an Input component as fallback with a warning indicator

### Requirement: Layout rendering
FormRenderer SHALL respect the layout configuration. If layout is present, fields SHALL be grouped by sections with section titles and arranged in the specified column count (1, 2, or 3). Each field's `width` property ("full"|"half"|"third") SHALL control its column span within the grid. Collapsible sections SHALL render with a toggle.

#### Scenario: Two-column layout
- **WHEN** layout.columns=2 and fields have width="half"
- **THEN** fields SHALL render side-by-side, two per row

#### Scenario: Mixed widths
- **WHEN** a section has fields with width "full", "half", "half"
- **THEN** the first field SHALL span the full row, the next two SHALL share the second row

#### Scenario: No layout provided
- **WHEN** layout is null or absent
- **THEN** all fields SHALL render in a single column in the order they appear in the fields array

### Requirement: Conditional visibility runtime
FormRenderer SHALL evaluate visibility conditions in real-time as the user changes field values. Hidden fields SHALL not render in the DOM and their values SHALL be excluded from the submitted data.

#### Scenario: Field appears when condition met
- **WHEN** field B has visibility condition `field="A", operator="equals", value="yes"` and the user types "yes" in field A
- **THEN** field B SHALL appear in the form

#### Scenario: Hidden field excluded from submission
- **WHEN** field B is hidden due to visibility condition and the user submits the form
- **THEN** field B's value SHALL NOT be included in the onSubmit data

### Requirement: Form state management
FormRenderer SHALL use React Hook Form for form state management. The component SHALL expose form data through the onSubmit callback on form submission and optionally through the onChange callback on any field change.

#### Scenario: Submit triggers callback
- **WHEN** the user fills all required fields and triggers form submission
- **THEN** onSubmit SHALL be called with a plain object mapping field keys to their values

#### Scenario: onChange fires on field change
- **WHEN** onChange prop is provided and the user changes a field value
- **THEN** onChange SHALL be called with the current form data object
