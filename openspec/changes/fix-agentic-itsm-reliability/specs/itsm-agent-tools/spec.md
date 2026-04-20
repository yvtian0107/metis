## MODIFIED Requirements

### Requirement: IT 服务台 Agent 预置定义
ITSM App 的 Seed 数据 SHALL 包含一个"IT 服务台智能体"Agent 预置定义。Seed 逻辑 SHALL 先删除旧的"IT 服务台"、"ITSM 流程决策"、"ITSM 处理协助"三个预置智能体。Seed 更新时 SHALL 保留管理员自定义的 system_prompt。

#### Scenario: Seed 创建 IT 服务台智能体
- **WHEN** ITSM App 执行 Seed 且 AI App 可用且智能体不存在
- **THEN** 系统 SHALL 创建"IT 服务台智能体"，包含完整 system_prompt 和工具绑定

#### Scenario: Seed 幂等 — 不覆盖已自定义的 prompt
- **WHEN** ITSM App 重复执行 Seed，同名智能体已存在
- **THEN** 系统 SHALL 跳过 system_prompt 的更新
- **AND** 系统 SHALL 同步更新工具绑定（确保新增工具被绑定）
- **AND** 系统 SHALL 记录 info 日志 "智能体已存在，跳过 prompt 更新"

#### Scenario: Tool binding 同步更新
- **WHEN** ITSM App 执行 Seed 且智能体已存在且 ToolNames 中包含新增工具
- **THEN** 系统 SHALL 将新增工具绑定到已有智能体
- **AND** 系统 SHALL 移除不再需要的旧工具绑定

### Requirement: 流程决策 Agent 预置定义
ITSM App 的 Seed 数据 SHALL 包含一个"流程决策智能体"Agent 预置定义。Seed 更新时 SHALL 保留管理员自定义的 system_prompt。

#### Scenario: Seed 创建流程决策智能体
- **WHEN** ITSM App 执行 Seed 且 AI App 可用且智能体不存在
- **THEN** 系统 SHALL 创建"流程决策智能体"，包含完整 system_prompt

#### Scenario: Seed 幂等 — 不覆盖已自定义的 prompt
- **WHEN** ITSM App 重复执行 Seed，同名智能体已存在
- **THEN** 系统 SHALL 跳过 system_prompt 的更新

### Requirement: draft hash 确定性序列化
`hashFormData()` SHALL 使用确定性的 JSON 序列化方式，确保相同内容始终产生相同的 hash 值。

#### Scenario: map 键顺序不影响 hash
- **WHEN** 对 `{"b": 1, "a": 2}` 和 `{"a": 2, "b": 1}` 分别计算 hash
- **THEN** 两次 hash 结果 SHALL 相同

#### Scenario: 实现方式
- **WHEN** 计算 form data hash
- **THEN** SHALL 先对 map keys 排序，再按排序后的顺序序列化为 JSON bytes 进行 hash
