## ADDED Requirements

### Requirement: Coding agent list page
The system SHALL provide a coding agent management page at `/ai/coding-agents`. The page SHALL display only `type=coding` agents in a responsive card grid. Each card SHALL show: type icon (SquareTerminal), name, description (truncated), runtime badge, status indicator, and a "聊天" button. The page SHALL support keyword search and pagination.

#### Scenario: View coding agent list
- **WHEN** admin navigates to `/ai/coding-agents`
- **THEN** system SHALL display only coding-type agents as cards, with no assistant agents visible

#### Scenario: Empty state
- **WHEN** no coding agents exist
- **THEN** the page SHALL display a centered empty state with hint text specific to coding agents (e.g., "创建一个可在工作区中执行编码任务的智能体")

#### Scenario: Start chat from coding card
- **WHEN** admin clicks the "聊天" button on an active coding agent card
- **THEN** system SHALL create a session and navigate to `/ai/chat/:sid`

### Requirement: Coding agent creation page
The system SHALL provide a creation page at `/ai/coding-agents/create`. The page SHALL NOT display a type selector — the type is fixed to `coding` by the route. The creation form SHALL display only coding-relevant fields.

#### Scenario: Create coding agent form fields
- **WHEN** admin navigates to `/ai/coding-agents/create`
- **THEN** the form SHALL display: name, description, visibility, runtime selector, execution mode, workspace path, node selector (when remote), MCP server bindings, skill bindings, and instructions
- **AND** the form SHALL NOT display any assistant-specific fields (provider, model, strategy, temperature, max tokens, max turns, system prompt, tool bindings, knowledge base bindings)

#### Scenario: No type selector
- **WHEN** admin views the coding agent creation form
- **THEN** there SHALL be no type dropdown or type selection step

#### Scenario: Remote mode shows node selector
- **WHEN** admin selects exec mode "remote" in the coding agent creation form
- **THEN** a node selector field SHALL appear

#### Scenario: Create from template
- **WHEN** admin creates from a coding template
- **THEN** the form SHALL pre-fill with template configuration (runtime, execMode) and only show coding-relevant fields

### Requirement: Coding agent detail page
The system SHALL provide a detail page at `/ai/coding-agents/:id` with tabs for configuration and sessions. The configuration view SHALL display only coding-relevant fields (runtime, execution mode, workspace, MCP servers). Navigation links (back, edit) SHALL use coding-agents paths.

#### Scenario: View coding detail
- **WHEN** admin navigates to `/ai/coding-agents/:id`
- **THEN** system SHALL display the agent detail with coding-specific configuration sections (runtime, exec mode, workspace) and no assistant-specific sections (model config, strategy, temperature)

#### Scenario: Edit navigation
- **WHEN** admin clicks "编辑" on the coding detail page
- **THEN** system SHALL navigate to `/ai/coding-agents/:id/edit`

### Requirement: Coding agent edit page
The system SHALL provide an edit page at `/ai/coding-agents/:id/edit`. The form SHALL display the same coding-specific fields as the creation form, pre-filled with the agent's current configuration. The type SHALL NOT be changeable.

#### Scenario: Edit coding agent
- **WHEN** admin navigates to `/ai/coding-agents/:id/edit`
- **THEN** system SHALL display the edit form with coding-relevant fields pre-filled
- **AND** there SHALL be no way to change the agent's type to assistant
