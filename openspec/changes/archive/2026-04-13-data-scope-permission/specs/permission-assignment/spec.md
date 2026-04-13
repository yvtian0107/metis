## ADDED Requirements

### Requirement: Data scope configuration in permission assignment UI
The system SHALL integrate data scope configuration into the permission assignment interface, accessible from the role management page.

#### Scenario: Data scope tab in permission drawer
- **WHEN** user opens the permission assignment Sheet for a role
- **THEN** the Sheet SHALL include a "数据权限" tab alongside the existing menu permission tree

#### Scenario: Select data scope type
- **WHEN** user clicks the "数据权限" tab
- **THEN** the tab SHALL display a radio group with options: 全部数据、本部门及下级、仅本部门、仅本人、自定义

#### Scenario: Custom scope department selector
- **WHEN** user selects "自定义" in the data scope radio group
- **THEN** a department multi-select tree SHALL appear, allowing selection of specific departments; previously configured departments SHALL be pre-selected

#### Scenario: Save data scope configuration
- **WHEN** user selects a scope type and clicks "保存"
- **THEN** the system SHALL call PUT /api/v1/roles/:id/data-scope with the selected scope and deptIds (if custom), and display a success toast

#### Scenario: Data scope displayed for system admin role
- **WHEN** user opens permission assignment for the system admin role
- **THEN** the data scope tab SHALL display "全部数据" as read-only and the save button SHALL be disabled
