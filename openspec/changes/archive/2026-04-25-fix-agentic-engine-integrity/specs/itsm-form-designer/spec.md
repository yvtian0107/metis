## ADDED Requirements

### Requirement: Field permission editor panel
The field property editor in FormDesigner SHALL include a "节点权限" section when opened in workflow-node context. The section SHALL list available workflow node IDs (passed via designer props from the workflow editor). For each node, a dropdown with options "editable" (default), "readonly", "hidden" SHALL be shown. Changes SHALL update the field's permissions map in real-time. The section SHALL NOT appear when FormDesigner is used standalone (no workflow context).

#### Scenario: Permission section visible in workflow context
- **WHEN** FormDesigner is opened from workflow editor's form binding panel with workflowNodes prop
- **THEN** the field property editor displays a "节点权限" section listing all workflow node IDs with permission dropdowns

#### Scenario: Permission section hidden standalone
- **WHEN** FormDesigner is opened without workflowNodes prop
- **THEN** the field property editor does NOT display the "节点权限" section

#### Scenario: Set permission for a node
- **WHEN** user selects "readonly" in the dropdown for node "approve_manager"
- **THEN** the field's permissions map is updated to {"approve_manager": "readonly"}

#### Scenario: Clear permission for a node
- **WHEN** user sets the dropdown back to "editable" (default) for a node
- **THEN** the entry is removed from the field's permissions map
