## MODIFIED Requirements

### Requirement: AI management sidebar hierarchy
The system SHALL expose AI management as a three-level sidebar hierarchy composed of the AI app entry, second-level capability groups, and third-level resource menu items. The second-level groups SHALL be ordered as `智能体`, `知识`, `工具`, `模型接入`. The `智能体` group SHALL contain two third-level menu items: `助手智能体` and `编码智能体`.

#### Scenario: AI sidebar shows grouped hierarchy
- **WHEN** a user with AI management permissions opens the AI app in the sidebar
- **THEN** the sidebar SHALL render the second-level groups in the order `智能体`, `知识`, `工具`, `模型接入`
- **THEN** each visible AI page entry SHALL appear under its configured second-level group

#### Scenario: Agent group contains two menu items
- **WHEN** a user with both `ai:assistant-agent:list` and `ai:coding-agent:list` permissions views the sidebar
- **THEN** the `智能体` group SHALL show `助手智能体` and `编码智能体` as two separate third-level menu items

#### Scenario: Agent group with partial permission
- **WHEN** a user has `ai:assistant-agent:list` but NOT `ai:coding-agent:list`
- **THEN** the `智能体` group SHALL show only `助手智能体`

#### Scenario: Agent group stays first
- **WHEN** the AI app contains multiple second-level groups
- **THEN** the `智能体` group SHALL be rendered before `知识`, `工具`, and `模型接入`

### Requirement: AI menu seed matches sidebar hierarchy
The seeded AI menu tree SHALL persist the same hierarchy used by the sidebar, using second-level directories for capability groups and third-level menu nodes for resource pages.

#### Scenario: AI menu tree is seeded as directories plus menus
- **WHEN** the system seeds AI management menus
- **THEN** it SHALL create or update second-level directory nodes for `智能体`, `知识`, `工具`, and `模型接入` under the `AI 管理` directory
- **THEN** it SHALL place `助手智能体` (path: `/ai/assistant-agents`, permission: `ai:assistant-agent:list`) and `编码智能体` (path: `/ai/coding-agents`, permission: `ai:coding-agent:list`) under the `智能体` directory

#### Scenario: Old unified Agent menu is removed
- **WHEN** the system seeds menus after upgrade
- **THEN** the old menu with permission `ai:agent:list` SHALL be soft-deleted

#### Scenario: Button permissions on new menus
- **WHEN** the system seeds button-level permissions
- **THEN** `ai:assistant-agent:create/update/delete` buttons SHALL be attached to the `助手智能体` menu
- **AND** `ai:coding-agent:create/update/delete` buttons SHALL be attached to the `编码智能体` menu
