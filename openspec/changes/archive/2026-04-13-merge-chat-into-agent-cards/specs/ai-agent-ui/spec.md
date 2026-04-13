## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: Chat agent selection page
**Reason**: The standalone `/ai/chat` page that displays agent cards for starting a chat is redundant — the chat entry point is now embedded directly in each agent's card on the `/ai/agents` page.
**Migration**: Users start chats from the Agent management page. The chat interface at `/ai/chat/:sid` remains unchanged.

## ADDED Requirements

### Requirement: Remove chat menu from navigation
The system SHALL remove the "对话" (`ai:chat`) menu item from the AI module's navigation sidebar. The `seed.Sync()` function SHALL soft-delete the `ai:chat` menu entry on startup. The `seed.Install()` function SHALL no longer create this menu item.

#### Scenario: Upgrade removes chat menu
- **WHEN** the system starts after upgrading to this version
- **THEN** `seed.Sync()` SHALL soft-delete the menu record with permission `ai:chat`
- **AND** the "对话" item SHALL no longer appear in the sidebar navigation

#### Scenario: Fresh install has no chat menu
- **WHEN** the system runs `seed.Install()` for the first time
- **THEN** the AI module navigation SHALL NOT contain a "对话" menu item
