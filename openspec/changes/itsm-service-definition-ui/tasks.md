## 1. 后端 Seed 数据补充

- [x] 1.1 在 `seed.go` 的 `seedSLATemplates()` 中补充 3 个 SLA 模板：rapid-workplace（15/120min）、critical-business（10/60min）、infra-change（60/480min）
- [x] 1.2 在 `seed.go` 中新增 `seedServiceDefinitions()` 函数，内置 5 个智能服务定义（engine_type=smart），collaborationSpec 从 bklite-cloud init.yml 复制，workflowJSON 为空
- [x] 1.3 在 `seedServiceDefinitions()` 中为 `db-backup-whitelist-action-e2e` 服务创建 2 个 ServiceAction（backup_whitelist_precheck、backup_whitelist_apply）
- [x] 1.4 在 `seedITSM()` 中调用 `seedServiceDefinitions()`（在 seedSLATemplates 和 seedCatalogs 之后）

## 2. 前端 i18n 补充

- [x] 2.1 在 `web/src/apps/itsm/locales/zh-CN.json` 和 `en.json` 中补充服务定义列表页和详情页所需的翻译 key（列名、Tab 标题、按钮文案、空状态提示等）

## 3. 前端 API 层

- [x] 3.1 在 `web/src/apps/itsm/api.ts` 中补充服务定义 CRUD 的 TypeScript 类型定义（ServiceDefinition、ServiceDefinitionListParams、CreateServiceDefRequest、UpdateServiceDefRequest）
- [x] 3.2 补充服务定义列表、详情、创建、更新、删除的 API 函数
- [x] 3.3 补充服务动作 CRUD 的类型和 API 函数（ServiceAction、CreateActionRequest）
- [x] 3.4 补充 SLA 模板列表查询 API 函数（用于详情页下拉选择）

## 4. 前端服务定义列表页

- [x] 4.1 创建 `web/src/apps/itsm/pages/services/index.tsx` 列表页组件，使用 `useListPage` hook 实现分页表格
- [x] 4.2 实现筛选栏：分类下拉（仅二级分类）、引擎类型下拉（classic/smart）、状态下拉、关键词搜索
- [x] 4.3 实现表格列：编码、名称、所属分类、引擎类型标签、SLA、状态、操作
- [x] 4.4 实现新增服务 Sheet（侧边抽屉），包含基础字段（名称、编码、分类、引擎类型、描述），创建成功后跳转详情页

## 5. 前端服务定义详情页

- [x] 5.1 创建 `web/src/apps/itsm/pages/services/[id]/index.tsx` 详情页组件，加载服务数据，展示页面标题和返回按钮
- [x] 5.2 实现 Tab 切换组件（基础信息、工作流、动作）
- [x] 5.3 实现基础信息 Tab：表单字段（名称、编码只读、描述、分类下拉、SLA 下拉、引擎类型只读、状态开关），smart 类型额外展示 CollaborationSpec 多行文本编辑区
- [x] 5.4 实现保存按钮，调用 `PUT /api/v1/itsm/services/:id` 更新服务

## 6. 前端工作流 Tab

- [x] 6.1 `@xyflow/react` 依赖已安装
- [x] 6.2 创建 `workflow-preview.tsx` 只读查看器组件，使用 React.lazy 动态加载，配置 nodesDraggable=false、nodesConnectable=false、elementsSelectable=false，保留 pan/zoom
- [x] 6.3 实现空状态：workflowJSON 为空时展示 "工作流尚未配置" 提示

## 7. 前端动作 Tab

- [x] 7.1 实现动作列表表格（编码、名称、类型、操作按钮）
- [x] 7.2 实现新增/编辑动作 Sheet（名称、编码、类型、配置 JSON）
- [x] 7.3 实现删除动作（确认弹窗 + API 调用）
- [x] 7.4 实现空状态提示

## 8. 路由与菜单注册

- [x] 8.1 在前端路由配置中注册 `/itsm/services` 列表页和 `/itsm/services/:id` 详情页路由（lazy-loaded）
- [x] 8.2 列表页添加"查看"按钮导航到详情页，Seed 中已有菜单指向正确
