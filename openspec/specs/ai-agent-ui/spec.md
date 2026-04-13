# ai-agent-ui Specification

## Purpose
TBD - created by archiving change ai-agent-runtime. Update Purpose after archive.
## Requirements
### Requirement: Agent list page
The system SHALL provide an Agent management page at `/ai/agents` accessible to admins. The page SHALL display agents in a responsive card grid layout (1 col mobile, 2 col sm, 3 col lg, 4 col xl). Each card SHALL show: type icon with gradient background, name, type badge, description (truncated to 1 line), status indicator, and action controls.

#### Scenario: View agent card grid
- **WHEN** admin navigates to `/ai/agents`
- **THEN** system SHALL display all agents as cards in a responsive grid with keyword search and pagination

#### Scenario: Active agent card
- **WHEN** an agent has `isActive: true`
- **THEN** the card SHALL display normally with a "聊天" button that creates a new session and navigates to `/ai/chat/:sid`

#### Scenario: Inactive agent card
- **WHEN** an agent has `isActive: false`
- **THEN** the card SHALL display with reduced opacity and the "聊天" button SHALL be disabled

#### Scenario: Card action menu
- **WHEN** admin clicks the more-options button (⋯) on a card
- **THEN** a dropdown menu SHALL appear with: "编辑" (opens AgentSheet), "详情" (navigates to `/ai/agents/:id`), and "删除" (shows confirmation dialog)

#### Scenario: Start chat from card
- **WHEN** admin clicks the "聊天" button on an active agent card
- **THEN** the system SHALL call `POST /api/v1/ai/sessions` with the agent ID, then navigate to `/ai/chat/:sid` with the new session ID

#### Scenario: Search agents
- **WHEN** admin enters a keyword and submits search
- **THEN** the card grid SHALL filter to show only matching agents

#### Scenario: Empty state
- **WHEN** no agents exist
- **THEN** the page SHALL display a centered empty state with icon and hint text

### Requirement: Agent creation wizard
The system SHALL provide a multi-step creation wizard via Sheet (drawer). Step 1: choose type (AI 助手 / 编程助手) with visual cards. Step 2: basic info (name, description, avatar). Step 3: type-specific configuration. Step 4: review and create.

#### Scenario: Create assistant agent
- **WHEN** admin selects "AI 助手" type
- **THEN** step 3 SHALL show: model selector, strategy dropdown (ReAct / Plan & Execute), system prompt textarea, temperature/max_tokens/max_turns inputs, and capability binding sections (knowledge bases, tools, MCP servers, skills)

#### Scenario: Create coding agent
- **WHEN** admin selects "编程助手" type
- **THEN** step 3 SHALL show: runtime selector (Claude Code / Codex / OpenCode / Aider), runtime-specific config form (dynamic based on runtime), execution mode radio (本机 / 远程节点), workspace path input, and node selector (shown only when remote mode selected)

#### Scenario: Create from template
- **WHEN** admin clicks a template card on the creation page
- **THEN** wizard SHALL pre-fill all configuration from the template, starting at step 2

### Requirement: Agent edit page
The system SHALL provide an Agent detail/edit page at `/ai/agents/:id` with tabs: Overview (basic info + config), Bindings (tools/knowledge/MCP/skills for assistant), Sessions (list of recent sessions), and Settings (visibility, danger zone with delete).

#### Scenario: Edit assistant configuration
- **WHEN** admin edits an assistant agent's model or tools
- **THEN** changes SHALL take effect for new sessions; existing running sessions continue with old config

#### Scenario: Delete agent
- **WHEN** admin clicks delete and confirms
- **THEN** system SHALL soft-delete the agent if no running sessions exist

### Requirement: Agent test dialog
The system SHALL provide a "测试对话" button on the agent detail page that opens an inline chat panel. This allows admins to test the agent's behavior before publishing.

#### Scenario: Test conversation
- **WHEN** admin clicks "测试对话"
- **THEN** system SHALL create a temporary session and display the chat interface inline

### Requirement: Remove chat menu from navigation
The system SHALL remove the "对话" (`ai:chat`) menu item from the AI module's navigation sidebar. The `seed.Sync()` function SHALL soft-delete the `ai:chat` menu entry on startup. The `seed.Install()` function SHALL no longer create this menu item.

#### Scenario: Upgrade removes chat menu
- **WHEN** the system starts after upgrading to this version
- **THEN** `seed.Sync()` SHALL soft-delete the menu record with permission `ai:chat`
- **AND** the "对话" item SHALL no longer appear in the sidebar navigation

#### Scenario: Fresh install has no chat menu
- **WHEN** the system runs `seed.Install()` for the first time
- **THEN** the AI module navigation SHALL NOT contain a "对话" menu item

### Requirement: Menu and permission seeding
The system SHALL seed a menu entry "Agents" under the AI module menu group, and corresponding Casbin policies for agent CRUD operations.

#### Scenario: Menu seed
- **WHEN** AI App seed runs
- **THEN** "Agents" menu item SHALL appear in the AI module navigation with icon and correct route

