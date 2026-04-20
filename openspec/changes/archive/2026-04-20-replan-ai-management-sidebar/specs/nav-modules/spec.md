## MODIFIED Requirements

### Requirement: Navigation config split by App
Navigation configuration SHALL be organized as one file per App in the app module registry, and the sidebar SHALL support both flat second-level items and grouped third-level navigation within an app. The navigation SHALL NOT include a standalone "首页" app entry.

#### Scenario: Adding a new app
- **WHEN** a developer creates a new app module and registers it in the frontend app bootstrap
- **THEN** the new app SHALL appear in the Icon Rail and Nav Panel

#### Scenario: Adding a nav item to existing app
- **WHEN** a developer adds a navigation item to an existing app definition
- **THEN** the new item SHALL appear in that app's Nav Panel

#### Scenario: App defines grouped third-level navigation
- **WHEN** an app declares second-level groups with nested resource items
- **THEN** the sidebar SHALL render the group labels and nested items in the configured order
- **THEN** the active state SHALL resolve against the nested item's route rather than the group container itself

#### Scenario: App defines flat navigation only
- **WHEN** an app defines only flat second-level items without nested groups
- **THEN** the sidebar SHALL continue rendering the existing two-level navigation without requiring grouped metadata

#### Scenario: No home app in navigation
- **WHEN** the navigation apps array is assembled
- **THEN** it SHALL NOT contain a standalone "首页" app entry

#### Scenario: Default active app after login
- **WHEN** the user logs in and lands on the redirected first menu path
- **THEN** the sidebar SHALL highlight the first app in the Icon Rail and show its first available navigation target as active

#### Scenario: Announcement nav item added to system management
- **WHEN** the system management app navigation is loaded
- **THEN** it SHALL include a "公告管理" item pointing to /announcements with the Megaphone icon

#### Scenario: Message channel nav item added to system management
- **WHEN** the system management app navigation is loaded
- **THEN** it SHALL include a "消息通道" item pointing to /channels with the Mail icon, positioned after "公告管理"

### Requirement: Central navigation export
The frontend navigation registry SHALL export the assembled app definitions and the helpers needed to resolve app-level, group-level, and item-level active navigation state.

#### Scenario: All apps assembled
- **WHEN** the application loads registered app modules
- **THEN** it SHALL receive the complete app definition array with all registered apps

#### Scenario: Grouped navigation metadata available to sidebar
- **WHEN** the sidebar resolves the active app definition
- **THEN** it SHALL be able to read both flat items and grouped nested items from the registry without hardcoding app-specific logic

#### Scenario: Breadcrumb labels aggregated
- **WHEN** the header component renders breadcrumbs
- **THEN** breadcrumb labels SHALL continue to resolve correctly for routes contributed by all app modules, including grouped third-level items
