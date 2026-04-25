## ADDED Requirements

### Requirement: Cycle detection in workflow graph
ValidateWorkflow() SHALL detect circular paths in the workflow graph using DFS coloring. When a cycle is detected, it SHALL return a blocking-level ValidationError with the cycle path description (e.g., "Cycle detected: nodeA → nodeB → nodeA").

#### Scenario: Workflow with no cycles passes validation
- **WHEN** ValidateWorkflow() is called on a workflow with nodes A→B→C→End (no back-edges)
- **THEN** no cycle-related ValidationError is returned

#### Scenario: Workflow with direct cycle detected
- **WHEN** ValidateWorkflow() is called on a workflow where node B has an edge back to node A (A→B→A)
- **THEN** a blocking ValidationError is returned with message containing the cycle path

#### Scenario: Workflow with indirect cycle detected
- **WHEN** ValidateWorkflow() is called on a workflow where A→B→C→D→B forms a cycle
- **THEN** a blocking ValidationError is returned with message describing the cycle

### Requirement: Dead-end branch detection
ValidateWorkflow() SHALL verify that all non-end nodes can reach at least one end node. Detection SHALL use reverse BFS from the end node. Nodes unreachable from the end node (excluding the start node's incoming path) SHALL produce a blocking ValidationError.

#### Scenario: All branches reach end node
- **WHEN** ValidateWorkflow() is called on a workflow where every branch leads to the end node
- **THEN** no dead-end ValidationError is returned

#### Scenario: Branch with no path to end detected
- **WHEN** ValidateWorkflow() is called on a workflow with an exclusive gateway where one branch leads to node X with no outgoing edges (and X is not an end node)
- **THEN** a blocking ValidationError is returned identifying node X as a dead-end

#### Scenario: Orphaned node cluster detected
- **WHEN** ValidateWorkflow() is called on a workflow containing nodes D→E that are disconnected from the main graph
- **THEN** blocking ValidationErrors are returned for nodes D and E as unreachable

### Requirement: Participant type validation
ValidateWorkflow() SHALL validate that participant type values in workflow nodes are from the allowed set: "user", "position", "department", "position_department", "requester", "requester_manager". Invalid participant types SHALL produce a blocking ValidationError.

#### Scenario: Valid participant types pass
- **WHEN** ValidateWorkflow() is called on a workflow where all nodes use participant types from the allowed set
- **THEN** no participant-type ValidationError is returned

#### Scenario: Invalid participant type detected
- **WHEN** ValidateWorkflow() is called on a workflow where a node uses participant type "magic_user"
- **THEN** a blocking ValidationError is returned identifying the node and the invalid type

#### Scenario: Node with empty participants array
- **WHEN** ValidateWorkflow() is called on a workflow where an approve node has an empty participants array
- **THEN** a blocking ValidationError is returned for that node indicating participants are required

### Requirement: ValidationError level classification
ValidationError struct SHALL include a Level field with values "blocking" or "warning". Existing structural and topology validations SHALL default to Level="blocking". Existing formSchema reference validations SHALL default to Level="warning".

#### Scenario: Blocking error prevents workflow save
- **WHEN** ValidateWorkflow() returns errors where at least one has Level="blocking"
- **THEN** the caller can distinguish blocking from warning errors to decide whether to persist

#### Scenario: Warning-only errors allow workflow save
- **WHEN** ValidateWorkflow() returns errors where all have Level="warning"
- **THEN** the caller can determine that the workflow is persistable with warnings
