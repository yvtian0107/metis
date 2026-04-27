## Context

当前 `seed-dev` 在首次运行时通过 `loadOrCreateSeedDevConfig()` 生成默认 SQLite 配置。SQLite 在开发中有两个问题：

1. `MaxOpenConns(1)` 导致所有 DB 操作串行化，Smart 引擎的 `SELECT FOR UPDATE`（`ensureContinuation`）在并发时阻塞 HTTP 响应
2. 与生产环境（PostgreSQL）行为不一致，某些 SQL 差异只在部署后才暴露

`docker-compose.yml` 已经提供了 PG/ClickHouse/FalkorDB，但 seed-dev 不会自动使用它们。

前端审批 Sheet 在 `onSuccess` 才关闭，用户感知为"按钮卡住"。

## Goals / Non-Goals

**Goals:**
- `make seed-dev` 首次生成配置时默认连接 docker-compose 中的 PG、ClickHouse、FalkorDB
- 提供 `make reset-pg` 快速重置 PG 数据库（DROP + CREATE + 重新 seed）
- 审批 Sheet 点击确认后立即关闭，进入乐观"决策中"状态

**Non-Goals:**
- 不改动 Classic 引擎的同步逻辑（本次只关注 Smart 引擎）
- 不删除 SQLite 支持（生产、测试仍可用 SQLite）
- 不改动 install wizard 的 DB 选择逻辑

## Decisions

### D1: `DefaultDevConfig()` 新函数 vs 修改 `loadOrCreateSeedDevConfig`

**选择：新增 `DefaultDevConfig()` 函数**

在 `internal/config/config.go` 新增 `DefaultDevConfig()` 返回 PG + ClickHouse + FalkorDB 配置，`loadOrCreateSeedDevConfig` 调用它替代 `DefaultSQLiteConfig()`。

理由：`DefaultSQLiteConfig()` 仍保留给 install wizard 和非 dev 场景使用，职责清晰。

### D2: `reset-pg` 的实现方式

**选择：Makefile target 调用 psql**

```makefile
reset-pg:
	PGPASSWORD=password psql -h localhost -U postgres -c "DROP DATABASE IF EXISTS postgres WITH (FORCE);"
	PGPASSWORD=password psql -h localhost -U postgres -d template1 -c "CREATE DATABASE postgres;"
	$(MAKE) seed-dev
```

用 `psql` 直接 DROP/CREATE，然后重新 seed。简单直接，不需要额外 Go 代码。

### D3: 前端审批 Sheet 乐观关闭

**选择：在 `onMutate` 中关闭 Sheet 并重置表单**

将 `setApprovalOpen(false)`、`setApprovalActivityId(null)`、`approvalForm.reset()` 从 `onSuccess` 移到 `onMutate`。`onSuccess` 只保留 `invalidateTicket()` 和 toast。`onError` 增加 toast 错误提示，但不重新打开 Sheet（用户可重新点击按钮）。

理由：`markSmartDecisioning()` 已经在 `onMutate` 中执行，Sheet 关闭应该与之同步，保持一致的乐观更新语义。

## Risks / Trade-offs

- **[psql 依赖]** → `make reset-pg` 需要本机安装 `psql` 客户端。macOS 上 `brew install libpq` 即可，Docker 用户也可以 `docker exec` 进 PG 容器。可接受，开发者机器通常已有。
- **[乐观关闭后 API 失败]** → Sheet 已关闭，用户需要通过 toast 感知错误并重新操作。实际场景中审批失败极少发生（权限不足、工单已终结），且 `invalidateTicket()` 会刷新状态，风险可控。
- **[已有 config.yml 的开发者]** → `loadOrCreateSeedDevConfig` 优先读取已有配置，不会覆盖。需要切换的开发者删除 `config.yml` 后重新 `make seed-dev` 即可。
