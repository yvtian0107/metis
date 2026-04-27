## MODIFIED Requirements

### Requirement: 服务定义列表页

系统 SHALL 在路由 `/itsm/services` 提供一体化服务目录工作区（由 `itsm-unified-catalog-workspace` 定义），取代原有的独立表格列表页。原有的表格展示、分页、搜索栏、分类下拉筛选由一体化工作区的 Group Section 导航 + 卡片网格替代。

#### Scenario: 默认展示
- **WHEN** 管理员进入 `/itsm/services` 页面
- **THEN** 系统 SHALL 展示一体化工作区：左侧 Group Section 目录导航面板 + 右侧服务卡片网格，默认选中"全部"，按 root 分组展示

#### Scenario: 按分类筛选
- **WHEN** 管理员在左侧导航面板点击某个 child 目录
- **THEN** 系统 SHALL 在右侧卡片网格中仅展示该目录下的服务

#### Scenario: 点击进入详情
- **WHEN** 管理员点击某个服务卡片
- **THEN** 系统 SHALL 导航到 `/itsm/services/:id` 详情页

### Requirement: 服务定义创建流程

管理员 SHALL 能够从一体化工作区通过 Sheet（侧边抽屉）创建新的服务定义，创建成功后自动跳转到详情页继续配置。

#### Scenario: 打开创建 Sheet
- **WHEN** 管理员点击工作区顶部的"新建服务"按钮或卡片网格末尾的引导卡片
- **THEN** 系统 SHALL 打开 Sheet 表单，包含字段：服务名称（必填）、服务编码（必填）、所属分类（必填，下拉选择 child 目录，若当前已选中某目录则预填）、引擎类型（必填，默认 "smart"）、描述（可选）

#### Scenario: 创建成功跳转
- **WHEN** 管理员填写表单并提交，API 返回成功
- **THEN** 系统 SHALL 关闭 Sheet，显示成功提示，刷新服务列表，并自动导航到新创建服务的详情页 `/itsm/services/:id`

#### Scenario: 编码冲突
- **WHEN** 管理员提交的编码已存在
- **THEN** 系统 SHALL 显示错误提示"服务编码已存在"

### Requirement: 参考路径生成结果展示

服务定义详情页 SHALL 将智能服务的参考路径生成视为协作式草图生成。管理员点击生成后，只要后端返回 workflowJson，页面 SHALL 刷新服务定义并展示生成出来的工作流图；validation issues SHALL 以非阻断方式提示，并由发布健康检查区域承载详细风险。

#### Scenario: 生成无问题的参考路径
- **WHEN** 管理员在智能服务详情页点击生成参考路径，API 返回 workflowJson 且 errors 为空
- **THEN** 页面 SHALL 显示成功提示
- **AND** 页面 SHALL 刷新当前服务定义数据
- **AND** 工作流图 SHALL 展示新生成的 workflow_json

#### Scenario: 生成存在 validation issues 的参考路径
- **WHEN** 管理员点击生成参考路径，API 返回 workflowJson 且 errors 非空
- **THEN** 页面 SHALL 显示“已生成但需确认”的非阻断提示
- **AND** 页面 SHALL 刷新当前服务定义数据
- **AND** 工作流图 SHALL 展示新生成的 workflow_json
- **AND** 发布健康检查区域 SHALL 展示 fail 或 warn 状态及对应问题

#### Scenario: 生成失败且无 workflowJson
- **WHEN** 管理员点击生成参考路径，但 API 因协作规范为空、引擎未配置、LLM 上游失败或 JSON 提取失败返回错误
- **THEN** 页面 SHALL 显示错误提示
- **AND** 页面 SHALL NOT 切换为新的工作流图状态

#### Scenario: 生成后同步服务缓存
- **WHEN** API 响应包含 service
- **THEN** 页面 SHALL 使用响应中的 service 更新当前服务详情缓存
- **AND** 页面 SHALL 刷新服务列表缓存，确保卡片、健康状态和详情数据一致

## REMOVED Requirements

### Requirement: 关键词搜索
**Reason**: 服务数量通常 < 30，左侧目录导航已提供充分的筛选能力，关键词搜索不再需要
**Migration**: 使用左侧 Group Section 目录导航按分类浏览服务

### Requirement: 按引擎类型筛选
**Reason**: 卡片上引擎类型通过品牌色条和 Badge 直观可辨，无需独立筛选器
**Migration**: 视觉扫描卡片品牌色即可区分

### Requirement: 按状态筛选
**Reason**: 卡片底部状态圆点直观标识启用/停用状态，服务数量少无需筛选
**Migration**: 视觉扫描卡片底部状态圆点

### Requirement: 分页
**Reason**: 卡片网格全量加载（pageSize=100），服务数量少不需要分页
**Migration**: 全量展示，无分页
