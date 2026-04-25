## ADDED Requirements

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
