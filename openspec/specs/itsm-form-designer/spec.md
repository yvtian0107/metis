## ADDED Requirements

### Requirement: Three-panel designer layout
The Form Designer page SHALL display three panels: a left field type palette (200px), a center form canvas (flex-1), and a right property editor (320px). The top SHALL include a toolbar with form name, code, section management, and save button.

#### Scenario: Initial empty state
- **WHEN** a user creates a new form definition
- **THEN** the designer SHALL show an empty canvas with a prompt to add fields from the left palette

#### Scenario: Toolbar displays form info
- **WHEN** the designer loads an existing form
- **THEN** the toolbar SHALL show the form name and code as editable fields, and the current version number as read-only

### Requirement: Field type palette
The left panel SHALL display field types grouped into categories: "基础输入" (text, textarea, number, email, url), "选择器" (select, multi_select, radio, checkbox, switch), "日期" (date, datetime, date_range), "高级" (user_picker, dept_picker, rich_text). Clicking a field type SHALL add a new field of that type to the current section in the canvas.

#### Scenario: Add text field
- **WHEN** the user clicks "text" in the palette
- **THEN** a new text field SHALL be added to the end of the currently active section with default label "新建文本字段" and auto-generated key

#### Scenario: Auto-generated key
- **WHEN** a new field is added
- **THEN** its key SHALL be auto-generated as `field_<index>` (e.g., `field_1`, `field_2`) and editable in the property panel

### Requirement: Canvas field display
The center canvas SHALL display fields grouped by sections. Each field SHALL render as a card showing: field type icon, label, key, required indicator, and width indicator. Fields SHALL support selection (click to select and show properties), reordering (up/down buttons), and deletion (delete button).

#### Scenario: Select field to edit properties
- **WHEN** the user clicks a field card in the canvas
- **THEN** the right panel SHALL update to show that field's properties

#### Scenario: Move field up
- **WHEN** the user clicks the "up" button on a field that is not the first in its section
- **THEN** the field SHALL swap position with the field above it

#### Scenario: Delete field
- **WHEN** the user clicks the delete button on a field and confirms
- **THEN** the field SHALL be removed from the schema and canvas

### Requirement: Section management
The toolbar SHALL include section management: add section, rename section, delete section (with confirmation if it contains fields), reorder sections. Fields belong to exactly one section.

#### Scenario: Add section
- **WHEN** the user clicks "添加分区"
- **THEN** a new section SHALL be added with default title "新分区" and no fields

#### Scenario: Delete non-empty section
- **WHEN** the user deletes a section containing 3 fields
- **THEN** a confirmation dialog SHALL appear warning that 3 fields will be removed

### Requirement: Property editor panel
The right panel SHALL display properties for the selected field. It SHALL include: common properties (key, label, placeholder, description, required, disabled, width, binding) applicable to all field types, and type-specific properties (options editor for select/multi_select/radio/checkbox, min/max for number, rows for textarea, format for date/datetime).

#### Scenario: Edit select options
- **WHEN** a select field is selected
- **THEN** the property editor SHALL show an "选项" section with an editable list of label+value pairs and add/remove buttons

#### Scenario: Edit validation rules
- **WHEN** any field is selected
- **THEN** the property editor SHALL show a "校验规则" section allowing the user to add rules from the supported rule list with custom error messages

### Requirement: Visibility rule editor
The property editor SHALL include a "条件显隐" section allowing the user to configure visibility conditions. The user SHALL select a source field (from other fields in the form), an operator, and a comparison value. Multiple conditions SHALL support AND/OR logic toggle.

#### Scenario: Configure visibility
- **WHEN** the user adds a visibility condition: field="category", operator="equals", value="incident"
- **THEN** the schema SHALL update with `visibility: {"conditions":[{"field":"category","operator":"equals","value":"incident"}],"logic":"and"}`

### Requirement: Preview mode
The designer SHALL include a "预览" toggle button. When activated, the canvas SHALL render the form using FormRenderer in create mode, allowing the designer to see the actual form appearance.

#### Scenario: Toggle preview
- **WHEN** the user clicks "预览"
- **THEN** the canvas SHALL switch from field cards to a live FormRenderer rendering of the current schema

### Requirement: Save form
The save button SHALL submit the current schema to the backend API. On success, it SHALL display a success toast. On validation error from the backend, it SHALL display the error messages.

#### Scenario: Save success
- **WHEN** the user clicks save and the backend accepts the schema
- **THEN** a success toast SHALL appear and the version number SHALL increment

#### Scenario: Save validation error
- **WHEN** the user clicks save and the schema has duplicate field keys
- **THEN** the backend error message SHALL be displayed in a toast

### Requirement: ITSM menu entry
The ITSM sidebar SHALL include a "表单管理" menu item that navigates to the form definition list page. The list page SHALL display all form definitions with name, code, version, scope, status columns and support keyword search.

#### Scenario: Navigate to form designer
- **WHEN** the user clicks a form definition in the list
- **THEN** the system SHALL navigate to the Form Designer page for that form

### Requirement: Field permission editor panel
The field property editor in FormDesigner SHALL include a "节点权限" section when opened in workflow-node context. The section SHALL list available workflow node IDs passed via designer props from the workflow editor. For each node, a dropdown with options `editable` (default), `readonly`, `hidden` SHALL be shown. Changes SHALL update the field's permissions map in real-time. The section SHALL NOT appear when FormDesigner is used standalone.

#### Scenario: Permission section visible in workflow context
- **WHEN** FormDesigner is opened from workflow editor's form binding panel with `workflowNodes` prop
- **THEN** the field property editor displays a "节点权限" section listing all workflow node IDs with permission dropdowns

#### Scenario: Permission section hidden standalone
- **WHEN** FormDesigner is opened without `workflowNodes` prop
- **THEN** the field property editor does NOT display the "节点权限" section

#### Scenario: Set permission for a node
- **WHEN** user selects `readonly` in the dropdown for node `approve_manager`
- **THEN** the field's permissions map is updated to include `{"approve_manager": "readonly"}`

#### Scenario: Clear permission for a node
- **WHEN** user sets the dropdown back to `editable` for a node
- **THEN** the entry is removed from the field's permissions map
