# apm-clickhouse-datasource Specification

## Purpose
TBD - created by archiving change 2026-04-13-apm-phase1-trace-explorer. Update Purpose after archive.
## Requirements
### Requirement: ClickHouse 数据源配置
系统 SHALL 支持在安装向导和 `config.yml` 中配置 ClickHouse 连接 DSN。

#### Scenario: 安装向导配置
- **WHEN** 用户在安装向导 Step 2 高级设置中启用 ClickHouse 并填入 DSN
- **THEN** DSN 写入 `config.yml` 的 `clickhouse.dsn` 字段

#### Scenario: 未配置降级
- **WHEN** `config.yml` 中无 `clickhouse` 配置段
- **THEN** ClickHouseClient 返回 nil，APM App 的 API 返回 HTTP 503，系统其他功能正常运行

#### Scenario: 连接验证
- **WHEN** 应用启动且 ClickHouse DSN 已配置
- **THEN** 客户端尝试连接并执行 `SELECT 1` 验证可达性，失败时记录 warning 日志但不阻止启动

### Requirement: ClickHouse 连接池管理
系统 SHALL 维护 ClickHouse 连接池，支持优雅关闭。

#### Scenario: 应用关闭
- **WHEN** 应用收到 shutdown 信号
- **THEN** ClickHouseClient 关闭连接池，释放资源

