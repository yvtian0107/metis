## MODIFIED Requirements

### Requirement: ITSM 引擎配置聚合 API

系统 SHALL 提供聚合 API `GET /api/v1/itsm/engine/config` 和 `PUT /api/v1/itsm/engine/config`，统一读写 ITSM 智能引擎的全部配置。API 受 JWT + Casbin 权限保护。

响应/请求结构：
```json
{
  "generator": {
    "model_id": 1,
    "provider_id": 1,
    "provider_name": "DeepSeek",
    "model_name": "deepseek-v3",
    "temperature": 0.3
  },
  "runtime": {
    "model_id": 2,
    "provider_id": 1,
    "provider_name": "DeepSeek",
    "model_name": "deepseek-r1",
    "temperature": 0.1,
    "decision_mode": "direct_first"
  },
  "general": {
    "max_retries": 3,
    "timeout_seconds": 30,
    "reasoning_log": "full",
    "fallback_assignee": 1
  }
}
```

#### Scenario: 读取引擎配置
- **WHEN** 管理员调用 `GET /api/v1/itsm/engine/config`
- **THEN** 系统 SHALL 读取 `itsm.generator` 和 `itsm.runtime` 两个 internal Agent 的配置（model_id 关联的 provider_id、provider_name、model_name、temperature），以及 SystemConfig 中 `itsm.engine.*` 前缀的运维参数（含 `itsm.engine.general.fallback_assignee`），合并为统一的 JSON 结构返回

#### Scenario: 保存引擎配置
- **WHEN** 管理员调用 `PUT /api/v1/itsm/engine/config` 提交完整配置
- **THEN** 系统 SHALL 更新 `itsm.generator` Agent 的 model_id 和 temperature，更新 `itsm.runtime` Agent 的 model_id 和 temperature，更新 SystemConfig 中的 `itsm.engine.runtime.decision_mode`、`itsm.engine.general.max_retries`、`itsm.engine.general.timeout_seconds`、`itsm.engine.general.reasoning_log`、`itsm.engine.general.fallback_assignee`

#### Scenario: Agent 未绑定模型时读取
- **WHEN** 读取配置且 `itsm.generator` Agent 的 model_id 为空（0 或 null）
- **THEN** 系统 SHALL 在 generator 区块返回 model_id=0、provider_id=0、provider_name=""、model_name=""，前端据此展示未配置状态

#### Scenario: 无效 model_id
- **WHEN** 保存配置时提交的 model_id 对应的 AIModel 不存在或已停用
- **THEN** 系统 SHALL 返回 400 错误 "模型不存在或已停用"

#### Scenario: 保存无效 fallback_assignee
- **WHEN** 保存配置时提交的 `fallback_assignee` 用户 ID 对应的用户不存在或 `is_active=false`
- **THEN** 系统 SHALL 返回 400 错误 "兜底处理人不存在或已停用"

#### Scenario: fallback_assignee 为 0 时清除配置
- **WHEN** 保存配置时 `fallback_assignee` 为 0
- **THEN** 系统 SHALL 将 `itsm.engine.general.fallback_assignee` 设为 "0"，表示未配置兜底处理人
