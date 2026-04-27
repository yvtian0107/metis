# Capability: itsm-form-permission-enforcement

## Purpose

ITSM 表单字段权限后端约束与设计器编辑能力。

## Requirements

### Requirement: Backend field permission enforcement
writeFormBindings() in variable_writer.go SHALL accept a currentNodeID parameter. For each form field being written, the function SHALL check field.Permissions[currentNodeID]. If the permission is `"readonly"` or `"hidden"`, the field value SHALL be silently skipped. If the permissions map is absent or has no entry for currentNodeID, the field SHALL default to `"editable"`. A warning log SHALL be emitted when a field write is skipped due to permission.

#### Scenario: Editable field written normally
- **WHEN** writeFormBindings() processes a field where `permissions[currentNodeID] = "editable"`
- **THEN** the field value is written to the process variable

#### Scenario: Readonly field silently skipped
- **WHEN** writeFormBindings() processes a field where `permissions[currentNodeID] = "readonly"`
- **THEN** the field value is NOT written to process variables and a warning log is emitted

#### Scenario: Hidden field silently skipped
- **WHEN** writeFormBindings() processes a field where `permissions[currentNodeID] = "hidden"`
- **THEN** the field value is NOT written to process variables and a warning log is emitted

#### Scenario: No permissions map defaults to editable
- **WHEN** writeFormBindings() processes a field with no permissions map defined
- **THEN** the field value is written normally

#### Scenario: No entry for current nodeID defaults to editable
- **WHEN** writeFormBindings() processes a field where permissions exists but has no entry for the current nodeID
- **THEN** the field value is written normally

### Requirement: Permission editor panel in FormDesigner
The field property editor in FormDesigner SHALL include a `"节点权限"` section. This section SHALL display a list of workflow node IDs loaded from the current workflow context with a dropdown for each: `editable | readonly | hidden`. Changes SHALL update the field's permissions map. The section SHALL only be visible when the designer is opened in workflow-node context.

#### Scenario: Permission editor visible in workflow context
- **WHEN** FormDesigner is opened from a workflow node's form binding panel
- **THEN** the field property editor shows a `"节点权限"` section with workflow node IDs

#### Scenario: Permission editor hidden outside workflow context
- **WHEN** FormDesigner is opened standalone
- **THEN** the field property editor does NOT show the `"节点权限"` section

#### Scenario: Set field to readonly for a node
- **WHEN** user selects `"readonly"` for node `"approve_1"` on a field
- **THEN** `field.permissions` is updated to include `{ "approve_1": "readonly" }`

#### Scenario: Remove permission entry
- **WHEN** user clears the permission setting for a node
- **THEN** the entry is removed from `field.permissions`
