## Context

ITSM 模块后端已完成服务目录（ServiceCatalog）、服务定义（ServiceDefinition）、服务动作（ServiceAction）的完整 CRUD API 和双引擎（classic/smart）架构。前端仅完成了服务目录的左右面板管理 UI（`/itsm/catalogs`），服务定义管理界面尚未实现。

当前 Seed 只初始化了 6 大类 + 18 子分类的目录结构、5 个优先级、2 个 SLA 模板，没有任何内置服务定义。参考 bklite-cloud 项目的 `buildin/init.yml`，我们需要内置 5 个典型的智能服务场景作为示例数据。

## Goals / Non-Goals

**Goals:**
- Seed 阶段内置 5 个智能服务定义 + 3 个 SLA 模板，安装后即可体验完整服务目录
- 提供服务定义列表页和详情页（方案 B：独立路由页面，非 Sheet 模式）
- 详情页支持三 Tab 布局：基础信息编辑、工作流只读查看、动作管理
- 工作流 Tab 使用 ReactFlow 只读渲染（可平移缩放拖拽，不可编辑）

**Non-Goals:**
- 不实现工作流可视化编辑器（仅只读查看）
- 不在 Seed 阶段写入 `workflowJSON`（后续由 AI 生成或手动导入）
- 不实现 AI 工作流生成功能
- 不修改 ServiceDefinition 数据模型（现有字段已满足需求）
- 不实现用户端服务目录浏览/申请页面

## Decisions

### D1: 详情页使用独立路由而非 Sheet

**决定**: 服务定义详情/编辑使用 `/itsm/services/:id` 独立页面，而非 metis 惯例的 Sheet 抽屉。

**原因**: 服务定义包含多维度配置（基础信息 + 工作流 + 动作），Sheet 空间不足以承载三 Tab 布局和 ReactFlow 画布。bklite-cloud 也采用了独立详情页模式。

**创建流程**: 新建服务时使用 Sheet 收集基础信息（名称、编码、分类、引擎类型），创建成功后自动跳转到详情页继续配置。

### D2: Seed 服务的 collaborationSpec 直接从 bklite-cloud 复制

**决定**: 5 个内置服务的 `collaborationSpec` 文本直接搬运自 bklite-cloud 的 `init.yml`。

**原因**: 这些文本是经过验证的真实 ITSM 场景描述，涵盖了简单审批、串签、动作编排、条件路由四种核心模式。复用已验证的内容比重写更可靠。

### D3: Seed 按 Code 幂等，服务关联 SLA 和 Catalog 通过 Code 查找

**决定**: `seedServiceDefinitions()` 通过 `code` 字段判重，通过 `catalog_code` 和 `sla_code` 查找关联记录的 ID。

**原因**: 与现有 `seedCatalogs()`、`seedPriorities()` 保持一致的幂等模式。Seed 数据之间通过 code 建立软关联，避免硬编码 ID。

### D4: 工作流 Tab 使用 @xyflow/react 只读模式

**决定**: 引入 `@xyflow/react` 依赖，配置为只读模式（`nodesDraggable={false}`, `nodesConnectable={false}`, `elementsSelectable={false}`），保留默认 pan/zoom。

**原因**: ReactFlow 是 metis workflow 引擎已采用的 JSON 格式标准（ReactFlow 风格的 nodes/edges），使用官方库渲染最为自然。只读模式下包体积增量约 ~150KB gzipped，在可接受范围内。

**备选**: 自定义 SVG 渲染 —— 工作量大且不兼容 ReactFlow 格式，放弃。

### D5: 列表页筛选维度

**决定**: 支持四个筛选维度 —— 分类（下拉，仅二级分类）、引擎类型（classic/smart）、状态（启用/停用）、关键词搜索。

**原因**: 覆盖管理员最常见的查找场景。分类筛选只展示二级分类（叶子节点），因为服务只能挂载在二级分类下。

### D6: 内置 5 个服务全部为 smart 引擎类型

**决定**: 所有 Seed 服务的 `engineType` 设为 `"smart"`。

**原因**: 这些服务的核心是 `collaborationSpec`（协作规范），由 Smart Engine + Agent 驱动流程决策。`workflowJSON` 后续作为参考辅助生成，而非流程执行的主驱动。

## Risks / Trade-offs

- **ReactFlow 依赖体积**: 引入 `@xyflow/react` 增加前端包体积 ~150KB。→ 使用动态 import (`React.lazy`) 按需加载工作流 Tab，不影响列表页首屏。
- **Seed 服务无 workflowJSON**: 工作流 Tab 打开时可能为空白。→ 显示友好的空状态提示（"工作流尚未生成"），不影响其他功能。
- **独立详情页打破 Sheet 惯例**: 与其他模块的交互模式不一致。→ 服务定义的复杂度确实需要更大空间，这是有理由的例外。列表页创建仍使用 Sheet 保持一致性。
