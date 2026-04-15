## MODIFIED Requirements

### Requirement: SLA 模板管理

系统 SHALL 支持 SLA 模板定义。SLATemplate 模型包含：name、code（唯一）、description、response_minutes（响应时间）、resolution_minutes（解决时间）、is_active，嵌入 BaseModel。

Seed 阶段 SHALL 内置 5 个 SLA 模板（幂等，按 code 判重）：

| code | 名称 | 响应时间 | 解决时间 |
|------|------|---------|---------|
| `standard` | 标准 | 240 min | 1440 min |
| `urgent` | 紧急 | 30 min | 240 min |
| `rapid-workplace` | 快速办公支持 | 15 min | 120 min |
| `critical-business` | 关键业务 | 10 min | 60 min |
| `infra-change` | 基础设施变更 | 60 min | 480 min |

#### Scenario: CRUD SLA 模板
- **WHEN** 管理员创建/编辑/删除 SLA 模板
- **THEN** 系统执行相应操作

#### Scenario: SLA 绑定到服务
- **WHEN** 管理员编辑服务定义，选择 sla_id
- **THEN** 该服务创建的工单使用此 SLA 的时间要求

#### Scenario: Seed 默认 SLA 模板
- **WHEN** 系统首次安装或 Sync 启动
- **THEN** 系统 SHALL 创建全部 5 个 SLA 模板（按 code 幂等），已存在的记录不覆盖

## ADDED Requirements

### Requirement: 内置智能服务定义种子数据

系统 SHALL 在 Seed 阶段通过 `seedServiceDefinitions()` 函数内置 5 个智能服务定义。所有服务 engine_type 为 "smart"，workflowJSON 为空（后续处理），通过 catalog_code 和 sla_code 关联分类和 SLA。使用 code 字段做幂等检查。

| code | 名称 | 分类 code | SLA code | 含 Actions |
|------|------|-----------|----------|-----------|
| `copilot-account-request` | Copilot 账号申请 | `account-access:provisioning` | `rapid-workplace` | 否 |
| `boss-serial-change-request` | 高风险变更协同申请（Boss） | `application-platform:release` | `infra-change` | 否 |
| `db-backup-whitelist-action-e2e` | 生产数据库备份白名单临时放行申请 | `application-platform:database` | `infra-change` | 是（2个） |
| `prod-server-temporary-access` | 生产服务器临时访问申请 | `infra-network:compute` | `critical-business` | 否 |
| `vpn-access-request` | VPN 开通申请 | `infra-network:network` | `standard` | 否 |

每个服务的 collaborationSpec 内容从 bklite-cloud 参考实现的 `buildin/init.yml` 直接复制：

1. **copilot-account-request**: "收集提单用户的Github账号信息和申请理由（可选），交给信息部的IT管理员审批，审批通过后结束流程。"
2. **boss-serial-change-request**: 收集申请主题、类别、风险等级、时间、影响范围、回滚要求、影响模块、变更明细表。先交 serial-reviewer 审批，再交 it 部 ops_admin 岗位审批。
3. **db-backup-whitelist-action-e2e**: 进入申请节点时执行预检动作，提交后交 it 部 dba_admin 岗位审批，通过后执行白名单放行动作。
4. **prod-server-temporary-access**: 收集访问服务器、时段、目的、原因。按原因路由到 ops_admin / network_admin / security_admin 审批。
5. **vpn-access-request**: 收集 VPN 账号、设备用途、访问原因。按原因路由到 network_admin / security_admin 审批。

#### Scenario: 首次安装种子服务
- **WHEN** 系统首次安装，数据库无服务定义数据
- **THEN** 系统 SHALL 创建全部 5 个服务定义，每个关联正确的 catalog 和 SLA

#### Scenario: 幂等重复执行
- **WHEN** 系统重启执行 Sync，数据库已有 seed 创建的服务定义
- **THEN** 系统 SHALL 跳过已存在的记录（按 code 匹配），不覆盖用户修改

#### Scenario: 关联分类不存在
- **WHEN** seed 执行时某服务引用的 catalog_code 在数据库中不存在
- **THEN** 系统 SHALL 记录 slog.Error 日志并跳过该服务，不中断其他服务的 seed

#### Scenario: 关联 SLA 不存在
- **WHEN** seed 执行时某服务引用的 sla_code 在数据库中不存在
- **THEN** 系统 SHALL 记录 slog.Warn 日志，将 sla_id 设为 nil，继续创建服务

### Requirement: 内置服务动作种子数据

系统 SHALL 在 `seedServiceDefinitions()` 中为 `db-backup-whitelist-action-e2e` 服务额外创建 2 个 ServiceAction：

| code | 名称 | HTTP Method | 说明 |
|------|------|-------------|------|
| `backup_whitelist_precheck` | 备份白名单预检 | POST | 校验数据库、时间窗与来源 IP 是否齐备 |
| `backup_whitelist_apply` | 执行备份白名单放行 | POST | 审批通过后自动执行白名单放行 |

每个 Action 的 config_json 包含 url（占位 `/precheck` 和 `/apply`）、method（POST）、timeout_seconds（5）。使用 code 字段做幂等检查。

#### Scenario: 首次安装种子动作
- **WHEN** 系统首次安装
- **THEN** 系统 SHALL 为 db-backup-whitelist 服务创建 2 个 ServiceAction

#### Scenario: 幂等重复执行
- **WHEN** 系统重启执行 Sync
- **THEN** 系统 SHALL 跳过已存在的动作记录（按 service_id + code 匹配）
