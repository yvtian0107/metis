## ADDED Requirements

### Requirement: Subprocess node validation
The validator SHALL validate subprocess nodes for structural integrity including SubProcessDef presence, exactly one outgoing edge, and recursive validation of the embedded workflow.

#### Scenario: SubProcessDef missing
- **WHEN** a subprocess node has no SubProcessDef or it is empty
- **THEN** a validation error SHALL be returned: "子流程节点 {nodeID} 必须配置 subprocess_def"

#### Scenario: SubProcessDef parse failure
- **WHEN** a subprocess node's SubProcessDef cannot be parsed as valid workflow JSON
- **THEN** a validation error SHALL be returned indicating parse failure

#### Scenario: Subprocess outgoing edge count
- **WHEN** a subprocess node does not have exactly one outgoing edge
- **THEN** a validation error SHALL be returned: "子流程节点 {nodeID} 必须有且仅有一条出边"

#### Scenario: Recursive validation of SubProcessDef
- **WHEN** a subprocess node has a valid SubProcessDef
- **THEN** the validator SHALL recursively validate the SubProcessDef using the same rules (start/end nodes, edges, gateway constraints, etc.)
- **AND** validation errors from the subprocess SHALL include the subprocess node ID as context prefix

#### Scenario: Nested subprocess rejected in v1
- **WHEN** a SubProcessDef contains a subprocess node (nested subprocess)
- **THEN** a validation error SHALL be returned: "当前版本不支持嵌套子流程"

### Requirement: resolveWorkflowContext helper
The engine SHALL provide a resolveWorkflowContext function that returns the correct WorkflowDef/nodeMap/outEdges for a given token, automatically resolving subprocess context.

#### Scenario: Main flow token
- **WHEN** resolveWorkflowContext is called with a token of type "main" or "parallel"
- **THEN** it SHALL return the def/maps parsed from ticket.WorkflowJSON

#### Scenario: Subprocess token
- **WHEN** resolveWorkflowContext is called with a token of type "subprocess"
- **THEN** it SHALL load the parent token, find the subprocess node in the main workflow, parse its SubProcessDef, and return the subprocess def/maps
