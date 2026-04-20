## ADDED Requirements

### Requirement: AI management sidebar hierarchy
The system SHALL expose AI management as a three-level sidebar hierarchy composed of the AI app entry, second-level capability groups, and third-level resource menu items. The second-level groups SHALL be ordered as `智能体`, `知识`, `工具`, `模型接入`.

#### Scenario: AI sidebar shows grouped hierarchy
- **WHEN** a user with AI management permissions opens the AI app in the sidebar
- **THEN** the sidebar SHALL render the second-level groups in the order `智能体`, `知识`, `工具`, `模型接入`
- **THEN** each visible AI page entry SHALL appear under its configured second-level group instead of being flattened into a single list

#### Scenario: Agent group stays first
- **WHEN** the AI app contains multiple second-level groups
- **THEN** the `智能体` group SHALL be rendered before `知识`, `工具`, and `模型接入`

### Requirement: Tooling pages become third-level navigation items
The system SHALL promote AI tooling resources from page-level tabs into independent third-level sidebar navigation items under the `工具` group. The tooling group SHALL contain `内建工具`, `MCP 服务`, and `技能包` as separate navigable items.

#### Scenario: Tooling resources appear as sidebar items
- **WHEN** a user enters AI management
- **THEN** the `工具` group SHALL show `内建工具`, `MCP 服务`, and `技能包` as distinct sidebar items
- **THEN** the user SHALL be able to navigate directly to each tooling resource without switching a page-level tab first

#### Scenario: Tooling pages are directly addressable
- **WHEN** a user opens a direct tooling URL such as `/ai/tools/builtin`, `/ai/tools/mcp`, or `/ai/tools/skills`
- **THEN** the matching third-level item in the `工具` group SHALL be marked active

### Requirement: AI management default routes remain stable after hierarchy change
The system SHALL preserve stable entry behavior for existing AI management URLs while moving tooling resources into third-level navigation.

#### Scenario: Legacy tools entry redirects to default tooling page
- **WHEN** a user navigates to `/ai/tools`
- **THEN** the system SHALL redirect to the default tooling page defined for the `工具` group

#### Scenario: Existing non-tool AI entry points remain direct
- **WHEN** a user navigates to existing AI entry pages such as Agents, Knowledge, or Providers
- **THEN** the system SHALL open the target page directly and highlight the corresponding second-level group and third-level item

### Requirement: AI menu seed matches sidebar hierarchy
The seeded AI menu tree SHALL persist the same hierarchy used by the sidebar, using second-level directories for capability groups and third-level menu nodes for resource pages.

#### Scenario: AI menu tree is seeded as directories plus menus
- **WHEN** the system seeds AI management menus
- **THEN** it SHALL create or update second-level directory nodes for `智能体`, `知识`, `工具`, and `模型接入` under the `AI 管理` directory
- **THEN** it SHALL place concrete page menus such as Agents, Knowledge Base, Builtin Tools, MCP Servers, Skills, and Providers under their matching second-level directory

#### Scenario: Button permissions stay attached to leaf menus
- **WHEN** the system seeds button-level AI permissions
- **THEN** each button permission SHALL remain attached to its final third-level resource menu node rather than to the second-level grouping directory
