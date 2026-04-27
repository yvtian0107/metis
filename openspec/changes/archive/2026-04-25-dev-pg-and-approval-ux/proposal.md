## Why

开发模式默认使用 SQLite（单连接、不支持真正的行锁），导致工单审批等并发场景下 `SELECT FOR UPDATE` 退化为全库串行，HTTP 请求阻塞，前端按钮一直处于等待状态。同时每次 `seed-dev` 都要手动调整 ClickHouse / FalkorDB 配置，浪费时间。

需要将开发基线切换到与生产一致的 PostgreSQL，并同时 seed 好 ClickHouse 和 FalkorDB 连接配置；同时优化审批按钮的前端交互，点击后立即关闭 Sheet 进入"决策中"状态，不再阻塞等待 API 响应。

## What Changes

- `seed-dev` 默认生成 PostgreSQL 配置（指向 `support-files/dev/docker-compose.yml` 中的 PG 实例），同时写入 ClickHouse 和 FalkorDB 连接配置
- 新增 `make reset-pg` 命令：DROP 并重建 PG 数据库，方便开发者快速重置
- 更新 `make clean` 以适配 PG 模式（不再只删 SQLite 文件）
- 前端审批 Sheet 在 `onMutate` 阶段立即关闭、重置表单，不再等待 `onSuccess`

## Capabilities

### New Capabilities

- `dev-postgres-seed`: 开发模式默认使用 PostgreSQL + ClickHouse + FalkorDB 的 seed 配置和 reset 命令

### Modified Capabilities

- `itsm-approval-ui`: 审批 Sheet 交互改为乐观关闭，点击即进入决策中状态

## Impact

- **后端**: `internal/config/config.go`（新增 DefaultDevConfig）、`cmd/server/seed_dev.go`（改用 PG 默认配置）、`Makefile`（新增 reset-pg、更新 clean）
- **前端**: `web/src/apps/itsm/pages/tickets/[id]/index.tsx`（progressMut 的 onMutate 逻辑）
- **无 Breaking Change**: 已有 `config.yml` 的开发者不受影响（`loadOrCreateSeedDevConfig` 仅在首次无配置时生成默认值）
