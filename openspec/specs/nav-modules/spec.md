# Capability: nav-modules

## Purpose
Defines modular navigation configuration organized as one file per App in the lib/nav/ directory, enabling clean separation and easy extensibility.

## Requirements

### Requirement: Navigation config split by App
Navigation configuration SHALL be organized as one file per App in lib/nav/ directory, and the sidebar SHALL support both flat second-level items and grouped third-level navigation within an app. The navigation SHALL NOT include a standalone "首页" app entry.

#### Scenario: Adding a new app
- **WHEN** a developer creates a new app file in lib/nav/ and imports it in index.ts
- **THEN** the new app SHALL appear in the Icon Rail and Nav Panel

#### Scenario: Adding a nav item to existing app
- **WHEN** a developer adds a NavItemDef to an app file
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
The lib/nav/index.ts SHALL export the assembled apps array, findActiveApp helper, and breadcrumbLabels.

#### Scenario: All apps assembled
- **WHEN** the application imports from lib/nav
- **THEN** it SHALL receive the complete apps[] array with all registered apps

#### Scenario: Grouped navigation metadata available to sidebar
- **WHEN** the sidebar resolves the active app definition
- **THEN** it SHALL be able to read both flat items and grouped nested items from the registry without hardcoding app-specific logic

#### Scenario: Breadcrumb labels aggregated
- **WHEN** the header component renders breadcrumbs
- **THEN** breadcrumbLabels SHALL include labels from all app modules, including grouped third-level items

### Requirement: Task center menu entry
The seed data SHALL include a "任务中心" menu entry under "系统管理" directory with path `/tasks`, icon `Clock`, permission `system:task:list`, and sort order 5.

#### Scenario: Menu seeded
- **WHEN** the database is seeded
- **THEN** a menu item "任务中心" SHALL exist under "系统管理" with type=menu, path=/tasks, permission=system:task:list

### Requirement: Task center button permissions
The seed data SHALL include button-level permissions under "任务中心": "暂停任务" (system:task:pause), "恢复任务" (system:task:resume), "触发任务" (system:task:trigger).

#### Scenario: Button permissions seeded
- **WHEN** the database is seeded
- **THEN** three button menu items SHALL exist under "任务中心" with the specified permissions

### Requirement: Admin Casbin policies for task APIs
The admin role seed SHALL include Casbin policies for all task API endpoints: GET /api/v1/tasks, GET /api/v1/tasks/stats, GET /api/v1/tasks/:name, GET /api/v1/tasks/:name/executions, POST /api/v1/tasks/:name/pause, POST /api/v1/tasks/:name/resume, POST /api/v1/tasks/:name/trigger.

#### Scenario: Admin policies seeded
- **WHEN** the database is seeded
- **THEN** the admin role SHALL have Casbin policies for all 7 task API endpoints
