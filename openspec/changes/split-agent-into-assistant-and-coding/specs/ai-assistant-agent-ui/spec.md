## ADDED Requirements

### Requirement: Assistant agent list page
The system SHALL provide an assistant agent management page at `/ai/assistant-agents`. The page SHALL display only `type=assistant` agents in a responsive card grid. Each card SHALL show: type icon (Bot), name, description (truncated), status indicator, and a "聊天" button. The page SHALL support keyword search and pagination.

#### Scenario: View assistant agent list
- **WHEN** admin navigates to `/ai/assistant-agents`
- **THEN** system SHALL display only assistant-type agents as cards, with no coding agents visible

#### Scenario: Empty state
- **WHEN** no assistant agents exist
- **THEN** the page SHALL display a centered empty state with hint text specific to assistant agents (e.g., "创建一个面向对话和任务执行的智能体")

#### Scenario: Start chat from assistant card
- **WHEN** admin clicks the "聊天" button on an active assistant agent card
- **THEN** system SHALL create a session and navigate to `/ai/chat/:sid`

### Requirement: Assistant agent creation page
The system SHALL provide a creation page at `/ai/assistant-agents/create`. The page SHALL NOT display a type selector — the type is fixed to `assistant` by the route. The creation form SHALL display only assistant-relevant fields.

#### Scenario: Create assistant agent form fields
- **WHEN** admin navigates to `/ai/assistant-agents/create`
- **THEN** the form SHALL display: name, description, visibility, provider selector, model selector, strategy, temperature, max tokens, max turns, system prompt, tool bindings, skill bindings, MCP server bindings, knowledge base bindings, and instructions
- **AND** the form SHALL NOT display any coding-specific fields (runtime, exec mode, workspace, node)

#### Scenario: No type selector
- **WHEN** admin views the assistant agent creation form
- **THEN** there SHALL be no type dropdown or type selection step

#### Scenario: Create from template
- **WHEN** admin creates from an assistant template
- **THEN** the form SHALL pre-fill with template configuration and only show assistant-relevant fields

### Requirement: Assistant agent detail page
The system SHALL provide a detail page at `/ai/assistant-agents/:id` with tabs for configuration and sessions. The configuration view SHALL display only assistant-relevant fields (model config, strategy, tools, knowledge bases). Navigation links (back, edit) SHALL use assistant-agents paths.

#### Scenario: View assistant detail
- **WHEN** admin navigates to `/ai/assistant-agents/:id`
- **THEN** system SHALL display the agent detail with assistant-specific configuration sections (model config, tools, knowledge bases) and no coding-specific sections (runtime, exec mode)

#### Scenario: Edit navigation
- **WHEN** admin clicks "编辑" on the assistant detail page
- **THEN** system SHALL navigate to `/ai/assistant-agents/:id/edit`

### Requirement: Assistant agent edit page
The system SHALL provide an edit page at `/ai/assistant-agents/:id/edit`. The form SHALL display the same assistant-specific fields as the creation form, pre-filled with the agent's current configuration. The type SHALL NOT be changeable.

#### Scenario: Edit assistant agent
- **WHEN** admin navigates to `/ai/assistant-agents/:id/edit`
- **THEN** system SHALL display the edit form with assistant-relevant fields pre-filled
- **AND** there SHALL be no way to change the agent's type to coding
