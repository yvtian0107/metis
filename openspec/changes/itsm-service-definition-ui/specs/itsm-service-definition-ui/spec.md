## ADDED Requirements

### Requirement: 服务定义列表页

系统 SHALL 提供服务定义管理列表页（路由 `/itsm/services`），以表格形式展示所有服务定义，支持筛选和操作。

#### Scenario: 默认展示
- **WHEN** 管理员进入 `/itsm/services` 页面
- **THEN** 系统 SHALL 展示服务定义表格，列包含：编码（code）、服务名称（name）、所属分类（catalog 名称）、引擎类型（classic/smart 标签）、SLA（模板名称）、状态（启用/停用）、操作按钮

#### Scenario: 按分类筛选
- **WHEN** 管理员在分类下拉中选择某个二级分类
- **THEN** 系统 SHALL 仅展示该分类下的服务定义

#### Scenario: 按引擎类型筛选
- **WHEN** 管理员在引擎类型下拉中选择 "经典" 或 "智能"
- **THEN** 系统 SHALL 仅展示对应 engine_type 的服务定义

#### Scenario: 按状态筛选
- **WHEN** 管理员在状态下拉中选择 "启用" 或 "停用"
- **THEN** 系统 SHALL 仅展示对应 is_active 状态的服务定义

#### Scenario: 关键词搜索
- **WHEN** 管理员在搜索框输入关键词
- **THEN** 系统 SHALL 按服务名称和编码模糊匹配，展示匹配结果

#### Scenario: 分页
- **WHEN** 服务定义数量超过单页显示量
- **THEN** 系统 SHALL 展示分页控件，支持翻页

#### Scenario: 点击进入详情
- **WHEN** 管理员点击某行的查看/编辑操作
- **THEN** 系统 SHALL 导航到 `/itsm/services/:id` 详情页

### Requirement: 服务定义创建流程

管理员 SHALL 能够从列表页通过 Sheet（侧边抽屉）创建新的服务定义，创建成功后自动跳转到详情页继续配置。

#### Scenario: 打开创建 Sheet
- **WHEN** 管理员点击列表页的 "新增服务" 按钮
- **THEN** 系统 SHALL 打开 Sheet 表单，包含字段：服务名称（必填）、服务编码（必填）、所属分类（必填，下拉选择二级分类）、引擎类型（必填，默认 "smart"）、描述（可选）

#### Scenario: 创建成功跳转
- **WHEN** 管理员填写表单并提交，API 返回成功
- **THEN** 系统 SHALL 关闭 Sheet，显示成功提示，并自动导航到新创建服务的详情页 `/itsm/services/:id`

#### Scenario: 编码冲突
- **WHEN** 管理员提交的编码已存在
- **THEN** 系统 SHALL 显示错误提示 "服务编码已存在"

### Requirement: 服务定义详情页三 Tab 布局

服务定义详情页（路由 `/itsm/services/:id`）SHALL 采用独立页面（非 Sheet），包含三个 Tab：基础信息、工作流、动作。

#### Scenario: 页面加载
- **WHEN** 管理员访问 `/itsm/services/:id`
- **THEN** 系统 SHALL 加载服务定义数据，展示页面标题（服务名称）、返回列表按钮、三个 Tab 切换。默认展示 "基础信息" Tab

#### Scenario: 服务不存在
- **WHEN** 管理员访问不存在的服务 ID
- **THEN** 系统 SHALL 展示 404 状态或自动跳转回列表页

#### Scenario: Tab 切换
- **WHEN** 管理员点击不同的 Tab
- **THEN** 系统 SHALL 切换展示对应 Tab 的内容，保持其他 Tab 状态不丢失

### Requirement: 基础信息 Tab

基础信息 Tab SHALL 展示服务定义的核心属性表单，支持编辑和保存。

#### Scenario: 表单字段展示
- **WHEN** 管理员进入基础信息 Tab
- **THEN** 系统 SHALL 展示以下字段：服务名称（text）、服务编码（text，创建后只读）、描述（textarea）、所属分类（下拉选择二级分类）、SLA 模板（下拉选择，可选）、引擎类型（下拉选择，创建后只读）、状态（开关）。当 engine_type 为 "smart" 时，额外展示协作规范（CollaborationSpec，多行文本编辑器）

#### Scenario: 保存修改
- **WHEN** 管理员修改表单字段并点击保存
- **THEN** 系统 SHALL 调用 `PUT /api/v1/itsm/services/:id` 保存修改，显示成功提示

#### Scenario: 智能模式协作规范编辑
- **WHEN** 服务为 smart 类型
- **THEN** 系统 SHALL 展示协作规范（CollaborationSpec）多行文本编辑区域，高度至少 8 行

#### Scenario: 经典模式隐藏智能字段
- **WHEN** 服务为 classic 类型
- **THEN** 系统 SHALL 隐藏协作规范字段

### Requirement: 工作流 Tab 只读查看器

工作流 Tab SHALL 使用 ReactFlow 只读模式渲染服务定义的 workflowJSON。支持平移和缩放，不可编辑节点和连线。

#### Scenario: 有工作流数据时渲染
- **WHEN** 管理员切换到工作流 Tab，且服务的 workflowJSON 不为空
- **THEN** 系统 SHALL 使用 ReactFlow 渲染工作流图，节点不可拖拽（nodesDraggable=false）、不可连线（nodesConnectable=false）、不可选中（elementsSelectable=false），支持画布平移和缩放

#### Scenario: 无工作流数据时空状态
- **WHEN** 管理员切换到工作流 Tab，且服务的 workflowJSON 为空或 null
- **THEN** 系统 SHALL 展示空状态提示："工作流尚未配置"

#### Scenario: 按需加载
- **WHEN** 工作流 Tab 组件加载
- **THEN** 系统 SHALL 通过 React.lazy 动态导入 @xyflow/react，不影响列表页和其他 Tab 的首屏加载

### Requirement: 动作 Tab 管理

动作 Tab SHALL 展示当前服务的 ServiceAction 列表，支持增删改操作。

#### Scenario: 动作列表展示
- **WHEN** 管理员切换到动作 Tab
- **THEN** 系统 SHALL 展示该服务下所有动作的表格，列包含：编码、名称、类型、状态、操作按钮

#### Scenario: 新增动作
- **WHEN** 管理员点击 "新增动作" 按钮
- **THEN** 系统 SHALL 打开 Sheet 表单，包含字段：动作名称、编码、描述、说明（prompt）、动作类型（默认 http）、配置（URL、Method、Headers JSON、Body Template、Timeout）、状态开关

#### Scenario: 编辑动作
- **WHEN** 管理员点击某动作的编辑按钮
- **THEN** 系统 SHALL 打开 Sheet 表单，预填充该动作的现有数据

#### Scenario: 删除动作
- **WHEN** 管理员点击某动作的删除按钮并确认
- **THEN** 系统 SHALL 调用 `DELETE /api/v1/itsm/services/:id/actions/:actionId` 删除动作

#### Scenario: 无动作时空状态
- **WHEN** 当前服务没有任何动作
- **THEN** 系统 SHALL 展示空状态提示，引导管理员创建动作
