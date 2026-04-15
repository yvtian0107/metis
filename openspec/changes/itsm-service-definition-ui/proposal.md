## Why

ITSM 模块目前只有服务目录管理（分类树）的 UI，缺少服务定义的管理界面和内置服务数据。管理员无法创建、查看和编辑服务定义，也没有开箱即用的示例服务。需要补齐服务定义管理 UI，并通过 Seed 内置 5 个典型智能服务场景（来自 bklite-cloud 参考实现），让系统安装后即有可用的服务目录。

## What Changes

- **Seed 内置 5 个智能服务定义**：Copilot 账号申请、高风险变更协同申请(Boss)、DB 备份白名单放行申请、生产服务器临时访问申请、VPN 开通申请。每个带 `collaborationSpec`，`workflowJSON` 暂不写入（后续处理）。DB 白名单场景附带 2 个 ServiceAction。
- **Seed 补充 3 个 SLA 模板**：快速办公支持（15/120min）、关键业务（10/60min）、基础设施变更（60/480min），与内置服务关联。
- **新增服务定义列表页**（`/itsm/services`）：表格展示，支持按分类、引擎类型、状态、关键词筛选。
- **新增服务定义详情页**（`/itsm/services/:id`）：独立路由页面（非 Sheet），包含三个 Tab：
  - 基础信息 Tab：表单编辑服务属性（名称、编码、分类、SLA、引擎类型、协作规范等）
  - 工作流 Tab：ReactFlow 只读查看器（可平移缩放，不可编辑节点/连线）
  - 动作 Tab：ServiceAction 列表管理（增删改）
- **新增服务定义创建流程**：从列表页通过 Sheet 创建基础信息，创建后跳转到详情页。

## Capabilities

### New Capabilities
- `itsm-service-definition-ui`: 服务定义的前端管理界面，包含列表页、详情页（三 Tab）、创建流程。

### Modified Capabilities
- `itsm-service-catalog`: 补充 Seed 内置智能服务定义和 SLA 模板数据。

## Impact

- **Backend**: `internal/app/itsm/seed.go` 新增 `seedSLATemplates`（补充 3 个）和 `seedServiceDefinitions`（5 个服务 + actions）。
- **Frontend**: 新增 `web/src/apps/itsm/pages/services/` 目录，包含列表页和详情页组件。新增路由 `/itsm/services` 和 `/itsm/services/:id`。
- **依赖**: 前端需要 `@xyflow/react`（ReactFlow）用于工作流只读渲染。
- **API**: 复用已有的服务定义 CRUD API（`/api/v1/itsm/services/*`），无需新增后端接口。
