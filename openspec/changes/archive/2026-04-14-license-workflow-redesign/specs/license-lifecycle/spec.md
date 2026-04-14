## ADDED Requirements

### Requirement: License 生命周期状态机
系统 SHALL 为 License 维护一个生命周期状态字段 `lifecycle_status`，取值包括：`pending`（待交付）、`active`（生效中）、`expired`（已过期）、`suspended`（已暂停）、`revoked`（已吊销）。原有 `status` 字段继续保留以兼容旧逻辑，`lifecycle_status` 为新增细粒度状态。

#### Scenario: 新签发许可的初始状态
- **WHEN** 系统成功签发一条 License
- **THEN** `lifecycle_status` MUST 设为 `active`（若 `validFrom <= now`）或 `pending`（若 `validFrom > now`）

#### Scenario: 过期检测
- **WHEN** 调度任务每日执行或用户查询 License 详情时
- **THEN** 对于 `lifecycle_status` 为 `active` 或 `pending` 且 `validUntil < now` 的记录，系统 MUST 将其 `lifecycle_status` 更新为 `expired`

#### Scenario: 暂停许可
- **WHEN** 用户请求暂停一条状态为 `active` 的 License
- **THEN** 系统 MUST 将 `lifecycle_status` 更新为 `suspended`，并记录 `suspended_at` 和 `suspended_by`

#### Scenario: 恢复许可
- **WHEN** 用户请求恢复一条状态为 `suspended` 的 License
- **THEN** 系统 MUST 将 `lifecycle_status` 恢复为 `active`（若未过期）或 `expired`（若已过期），并清空 `suspended_at` 和 `suspended_by`

#### Scenario: 吊销许可
- **WHEN** 用户请求吊销任意非 `revoked` 状态的 License
- **THEN** 系统 MUST 将 `lifecycle_status` 和 `status` 均更新为 `revoked`，并记录 `revoked_at` 和 `revoked_by`

### Requirement: 许可续期（Renewal）
系统 SHALL 提供 License 续期功能，允许延长 License 的有效期。Renewal 操作修改原 License 记录，不创建新记录。

#### Scenario: 成功续期
- **WHEN** 用户对 `active`、`expired` 或 `suspended` 状态的 License 提交 Renewal 请求，并提供新的 `validUntil`
- **THEN** 系统 MUST 更新原记录的 `validUntil`，若新日期大于当前时间则将 `lifecycle_status` 恢复为 `active`

#### Scenario: 续期 revoked 许可
- **WHEN** 用户尝试对 `revoked` 状态的 License 执行 Renewal
- **THEN** 系统 MUST 返回错误 "已吊销的许可不能续期"

### Requirement: 许可升级（Upgrade）
系统 SHALL 提供 License 升级功能，允许变更 License 的 Plan 或 ConstraintValues。Upgrade 创建一条新的 License 记录，并在新记录中通过 `original_license_id` 指向旧记录。旧记录保持原状态（通常自动变为 `revoked` 或保留为历史）。

#### Scenario: 成功升级
- **WHEN** 用户对 `active` 状态的 License 提交 Upgrade 请求，并提供新的 `planId` 和/或 `constraintValues`
- **THEN** 系统 MUST 创建新 License 记录，重新签名生成新的 `activationCode`，新记录的 `original_license_id` 指向旧记录 ID，同时旧记录 MUST 被标记为 `revoked`

#### Scenario: 升级非 active 许可
- **WHEN** 用户尝试对非 `active` 状态的 License 执行 Upgrade
- **THEN** 系统 MUST 返回错误 "只能对生效中的许可执行升级"

### Requirement: 生命周期 API 路由
系统 SHALL 在 `/api/v1/license/licenses/:id` 下提供生命周期管理 REST API，所有接口受 JWT + Casbin 保护。

#### Scenario: API 路由清单
- **WHEN** license app 注册路由
- **THEN** 以下路由可用：
  - `POST /api/v1/license/licenses/:id/renew` — 续期
  - `POST /api/v1/license/licenses/:id/upgrade` — 升级
  - `PATCH /api/v1/license/licenses/:id/suspend` — 暂停
  - `PATCH /api/v1/license/licenses/:id/reactivate` — 恢复
