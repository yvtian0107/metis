## MODIFIED Requirements

### Requirement: 内置智能服务定义种子数据

系统 SHALL 在 Seed 阶段通过 `seedServiceDefinitions()` 函数内置 5 个智能服务定义。所有服务 engine_type 为 "smart"，workflowJSON 为空（后续处理），通过 catalog_code 和 sla_code 关联分类和 SLA。使用 code 字段做幂等检查。内置服务的 collaborationSpec/participant 配置 SHALL 与 built-in Org position seed 和 install-time admin identity 对齐，使 fresh install 下的 `validate_participants` 结果与真实路由行为一致，不因 seed 失配产生假失败。

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
3. **db-backup-whitelist-action-e2e**: 进入申请节点时执行预检动作，提交后交 it 部与 built-in Org 种子一致的数据库管理员岗位审批，通过后执行白名单放行动作。
4. **prod-server-temporary-access**: 收集访问服务器、时段、目的、原因。按原因路由到 ops_admin / network_admin / security_admin 审批。
5. **vpn-access-request**: 收集 VPN 账号、设备用途、访问原因。按原因路由到 network_admin / security_admin 审批。

#### Scenario: 首次安装种子服务
- **WHEN** 系统首次安装，数据库无服务定义数据
- **THEN** 系统 SHALL 创建全部 5 个服务定义，每个关联正确的 catalog 和 SLA

#### Scenario: Fresh install participant validation succeeds for built-in services
- **WHEN** fresh install 完成后对内置 5 个智能服务执行 `validate_participants`
- **THEN** 使用 built-in Org seed 和 install-time admin 默认身份时，验证结果 SHALL 与服务设计的真实参与人路由一致
- **AND** 不得仅因 seed 中岗位编码不一致而失败

#### Scenario: 幂等重复执行
- **WHEN** 系统重启执行 Sync，数据库已有 seed 创建的服务定义
- **THEN** 系统 SHALL 跳过已存在的记录（按 code 匹配），不覆盖用户修改

#### Scenario: 关联分类不存在
- **WHEN** seed 执行时某服务引用的 catalog_code 在数据库中不存在
- **THEN** 系统 SHALL 记录 slog.Error 日志并跳过该服务，不中断其他服务的 seed

#### Scenario: 关联 SLA 不存在
- **WHEN** seed 执行时某服务引用的 sla_code 在数据库中不存在
- **THEN** 系统 SHALL 记录 slog.Warn 日志，将 sla_id 设为 nil，继续创建服务
