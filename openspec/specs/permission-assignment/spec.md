# Capability: permission-assignment

## Purpose
Provides the API and frontend UI for assigning menu-based and API-based permissions to roles via Casbin policies.

## Requirements

### Requirement: Role permission assignment API
The system SHALL provide endpoints to view and assign permissions (Casbin policies) to a role.

#### Scenario: Get role permissions
- **WHEN** GET /api/v1/roles/:id/permissions
- **THEN** the system SHALL return `{code: 0, data: {menuIds: [1,2,3], apiPolicies: [{path, method}]}}` listing the role's assigned menu IDs and API policies

#### Scenario: Set role permissions
- **WHEN** PUT /api/v1/roles/:id/permissions with `{menuIds: [1,2,5], apiPolicies: [{path: "/api/v1/users", method: "GET"}]}`
- **THEN** the system SHALL replace all Casbin policies for this role with policies derived from the menu permissions and the explicit API policies

#### Scenario: Menu permission derivation
- **WHEN** menuIds include a menu with permission "system:user:list"
- **THEN** the system SHALL create a Casbin policy (roleCode, "system:user:list", "read") for that role

#### Scenario: API permission assignment
- **WHEN** apiPolicies include `{path: "/api/v1/users", method: "GET"}`
- **THEN** the system SHALL create a Casbin policy (roleCode, "/api/v1/users", "GET") for that role

#### Scenario: Cannot modify admin role core permissions
- **WHEN** PUT /api/v1/roles/:id/permissions for the "admin" system role
- **THEN** the system SHALL return 400 with message "cannot modify system admin permissions" (admin always has full access)

### Requirement: Permission assignment frontend UI
The system SHALL provide a permission assignment interface within the role management page.

#### Scenario: View permission assignment
- **WHEN** user clicks "分配权限" on a role row
- **THEN** a dialog/drawer SHALL open showing the full menu tree as a checkable tree with current assignments pre-checked

#### Scenario: Toggle menu permission
- **WHEN** user checks/unchecks a menu node in the permission tree
- **THEN** the corresponding permission SHALL be included/excluded; checking a parent SHALL auto-check all children; unchecking all children SHALL auto-uncheck the parent

#### Scenario: Save permission changes
- **WHEN** user clicks "保存" in the permission assignment dialog
- **THEN** the system SHALL call PUT /api/v1/roles/:id/permissions with the selected menuIds and the derived API policies

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
