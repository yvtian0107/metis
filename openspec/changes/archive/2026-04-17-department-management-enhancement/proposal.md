## Why

部门管理当前缺少两项关键组织建模能力：部门负责人展示/选择（后端字段已有但前端未暴露）、部门可用职位约束（任何职位可在任何部门使用，缺乏管控）。同时表单中排序字段对用户无意义，需要清理。

## What Changes

- **移除排序字段显示**：前端部门表单中移除 sort 输入框，后端字段保留但不再暴露给用户
- **部门负责人**：前端表单增加负责人用户选择器（从全局用户列表选择），列表树表增加负责人列展示，Tree API response 附带负责人用户名
- **部门可用职位**：新增 `DepartmentPosition` 多对多关联表，定义每个部门允许使用的职位；前端部门表单增加可用职位多选管理；人员分配时强制校验职位是否在目标部门的可用职位列表中（未配置可用职位的部门视为不限制，向后兼容）

## Capabilities

### New Capabilities
- `dept-allowed-positions`: 部门可用职位关联管理——新增 DepartmentPosition model、API、前端多选管理、人员分配强制校验

### Modified Capabilities
- `org-department`: Tree API response 增加 managerName 字段
- `org-department-ui`: 表单移除 sort 字段、增加负责人选择器和可用职位多选；列表增加负责人列
- `org-assignment`: 人员分配时增加职位-部门可用性校验逻辑

## Impact

- **Model 层**: 新增 `DepartmentPosition` struct，注册到 App.Models() 进行 AutoMigrate
- **API**: 新增 `GET/PUT /api/v1/org/departments/:id/positions`；修改 Tree API 返回结构
- **Service 层**: DepartmentService 增加可用职位管理方法和 Tree 附带负责人信息；AssignmentService 增加职位校验
- **前端**: department-sheet.tsx（表单重构）、departments/index.tsx（列表列调整）、国际化文本
- **Seed**: 为现有部门配置默认可用职位
- **Casbin**: 新增 API 路由的权限策略
