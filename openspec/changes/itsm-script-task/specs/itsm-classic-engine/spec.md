## MODIFIED Requirements

### Requirement: Workflow JSON Schema 校验
系统 SHALL 在保存 workflow_json 时进行完整性校验。校验规则包括：有且仅有一个 start 节点；至少一个 end 节点；start 节点有且仅有一条出边；end 节点无出边；所有边的 source 和 target 引用存在的节点 ID；无孤立节点（每个非 start 节点至少有一条入边）；**exclusive** 节点的每条非默认出边 SHALL 配置条件；**exclusive** 节点至少有两条出边；节点类型 SHALL 是合法的已注册类型之一。

**Parallel/Inclusive 校验新增规则：**
- parallel/inclusive 节点 SHALL 移出 UnimplementedNodeTypes（不再输出"未实现"warning）
- parallel 和 inclusive 的 fork 节点 SHALL 至少有两条出边
- parallel 和 inclusive 的 join 节点 SHALL 至少有两条入边且恰好一条出边
- inclusive fork 节点的每条非默认出边 SHALL 配置条件（复用 exclusive 的校验逻辑）
- parallel/inclusive 节点 SHALL 有 gateway_direction 属性（"fork" 或 "join"），缺失时报错

**Script 节点校验规则（⑤a 新增）：**
- `script` 节点 SHALL 从 UnimplementedNodeTypes 中移除（不再输出"未实现"warning）
- `script` 节点 SHALL 有且仅有一条出边（自动节点，单一出路）

对 `subprocess`、`timer`、`signal`、`b_timer`、`b_error` 等已注册但未实现执行逻辑的节点类型，ValidateWorkflow SHALL 通过校验但返回 **warning** 级别的 ValidationError。

#### Scenario: 校验通过
- **WHEN** 管理员保存 workflow_json，内容包含 1 个 start、1 个 end、合法的边关系
- **THEN** 校验通过，workflow_json 保存成功

#### Scenario: exclusive 出边缺少条件
- **WHEN** exclusive 节点的某条非默认出边没有配置 condition
- **THEN** 校验失败，返回错误"排他网关节点 {node_id} 的出边 {edge_id} 缺少条件配置"

#### Scenario: exclusive 出边不足
- **WHEN** exclusive 节点只有一条出边
- **THEN** 校验失败，返回错误"排他网关节点 {node_id} 至少需要两条出边"

#### Scenario: parallel fork 出边不足
- **WHEN** parallel fork 节点只有一条出边
- **THEN** 校验失败，返回错误"并行网关 fork 节点 {node_id} 至少需要两条出边"

#### Scenario: parallel join 入边不足
- **WHEN** parallel join 节点只有一条入边
- **THEN** 校验失败，返回错误"并行网关 join 节点 {node_id} 至少需要两条入边"

#### Scenario: parallel join 出边数量不为一
- **WHEN** parallel join 节点有 0 条或 2+ 条出边
- **THEN** 校验失败，返回错误"并行网关 join 节点 {node_id} 必须有且仅有一条出边"

#### Scenario: inclusive fork 出边缺少条件
- **WHEN** inclusive fork 节点的某条非默认出边没有配置 condition
- **THEN** 校验失败，返回错误"包含网关 fork 节点 {node_id} 的出边 {edge_id} 缺少条件配置"

#### Scenario: parallel/inclusive 缺少 gateway_direction
- **WHEN** parallel 或 inclusive 节点没有配置 gateway_direction 属性
- **THEN** 校验失败，返回错误"节点 {node_id} 类型 {type} 必须配置 gateway_direction（fork 或 join）"

#### Scenario: script 节点校验通过
- **WHEN** workflow_json 包含 type="script" 的节点，且有一条出边
- **THEN** 校验通过，不输出 warning

#### Scenario: script 节点出边数量不为一
- **WHEN** script 节点有 0 条或 2+ 条出边
- **THEN** 校验失败，返回错误"脚本节点 {node_id} 必须有且仅有一条出边"

#### Scenario: 非法节点类型
- **WHEN** 节点的 type 不在已注册的合法类型中
- **THEN** 校验失败，返回错误"节点 {node_id} 的类型 {type} 不合法"

#### Scenario: 未实现节点类型的 warning
- **WHEN** workflow_json 中包含 type="subprocess" 的节点
- **THEN** 校验返回 warning 级别信息"节点 {node_id} 类型 subprocess 已注册但执行逻辑尚未实现，当前版本不支持运行"
