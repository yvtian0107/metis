## MODIFIED Requirements

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
- **WHEN** a field with type "unknown_type" is encountered
- **THEN** it renders as a basic Input component with a warning indicator
