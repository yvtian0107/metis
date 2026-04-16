# Capability: itsm-subprocess

## Purpose
Provides subprocess execution support for the ITSM classic workflow engine. Enables embedded subprocess nodes that execute an isolated inner workflow within a parent flow, with variable scope isolation and automatic resumption of the parent flow upon subprocess completion.

## Requirements

### Requirement: Subprocess execution
The engine SHALL execute embedded subprocess nodes by parsing the SubProcessDef from node data, creating a subprocess token with isolated scope, and recursively processing the subprocess's internal workflow.

#### Scenario: Basic subprocess execution
- **WHEN** processNode encounters a `subprocess` node with valid SubProcessDef
- **THEN** the parent token status SHALL be set to `waiting`
- **AND** a new token SHALL be created with `token_type = "subprocess"`, `parent_token_id = parent token ID`, `scope_id = subprocess node ID`
- **AND** the subprocess's start node SHALL be found and its outgoing target processed via processNode with the subprocess's own def/nodeMap/outEdges

#### Scenario: SubProcessDef missing or invalid
- **WHEN** processNode encounters a `subprocess` node with empty or unparseable SubProcessDef
- **THEN** the engine SHALL return an error indicating the subprocess definition is invalid

### Requirement: Subprocess completion resumes parent flow
When a subprocess's end node is reached, the engine SHALL complete the subprocess token, reactivate the parent token, and continue the parent flow past the subprocess node.

#### Scenario: Subprocess end node reached
- **WHEN** processNode encounters a `NodeEnd` and the current token has `token_type = "subprocess"`
- **THEN** a completed activity SHALL be created for the end node
- **AND** the subprocess token status SHALL be set to `completed`
- **AND** the parent token status SHALL be changed from `waiting` to `active`
- **AND** the parent flow SHALL continue by processing the subprocess node's outgoing edge target using the main workflow's def/nodeMap/outEdges

#### Scenario: Subprocess end with parent also a child token
- **WHEN** a subprocess token reaches NodeEnd and the parent token also has a ParentTokenID (e.g., subprocess inside a parallel branch)
- **THEN** the parent token SHALL be reactivated and the flow SHALL continue past the subprocess node in the parent's workflow context

### Requirement: Workflow context resolution for subprocess activities
All entry points that load workflow context from ticket.WorkflowJSON SHALL detect subprocess tokens and resolve to the correct subprocess workflow definition.

#### Scenario: Progress called for activity inside subprocess
- **WHEN** Progress() is called for an activity whose token has `token_type = "subprocess"`
- **THEN** the engine SHALL load the parent token, find the subprocess node in the main workflow, parse its SubProcessDef, and use the subprocess def/nodeMap/outEdges for edge matching

#### Scenario: HandleActionExecute for action inside subprocess
- **WHEN** HandleActionExecute triggers for an action node inside a subprocess
- **THEN** the tryHandleBoundaryError and Progress calls SHALL use the subprocess workflow context

#### Scenario: HandleWaitTimer for wait node inside subprocess
- **WHEN** HandleWaitTimer triggers for a wait node inside a subprocess
- **THEN** Progress SHALL use the subprocess workflow context

#### Scenario: HandleBoundaryTimer for boundary inside subprocess
- **WHEN** HandleBoundaryTimer fires for a boundary event inside a subprocess
- **THEN** workflow loading SHALL resolve to the subprocess def

### Requirement: Subprocess variable scope isolation
Subprocess tokens SHALL use the subprocess node ID as their scope_id, ensuring process variables written inside the subprocess are isolated from the parent flow.

#### Scenario: Form binding writes inside subprocess
- **WHEN** a form node inside a subprocess writes variables via writeFormBindings
- **THEN** the variables SHALL be stored with `scope_id = subprocess node ID`
- **AND** parent flow variables with `scope_id = "root"` SHALL NOT be affected

#### Scenario: Script node reads inside subprocess
- **WHEN** a script node inside a subprocess calls buildScriptEnv
- **THEN** only variables with `scope_id = subprocess node ID` SHALL be loaded into the environment
