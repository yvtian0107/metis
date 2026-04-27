## 1. 后端：DefaultDevConfig 与 seed-dev 改造

- [x] 1.1 在 `internal/config/config.go` 新增 `DefaultDevConfig()` 函数，返回 PG + ClickHouse + FalkorDB 的默认配置
- [x] 1.2 修改 `cmd/server/seed_dev.go` 的 `loadOrCreateSeedDevConfig()`，将 `DefaultSQLiteConfig()` 替换为 `DefaultDevConfig()`
- [x] 1.3 验证 `make seed-dev` 首次运行生成正确的 PG 配置并成功 seed

## 2. Makefile 更新

- [x] 2.1 新增 `make reset-pg` target：DROP/CREATE postgres 数据库 + 删除 config.yml + 重新 seed-dev
- [x] 2.2 `make clean` 保持兼容（仍删除 SQLite 文件 + config.yml）

## 3. 前端：审批 Sheet 乐观关闭

- [x] 3.1 修改 `web/src/apps/itsm/pages/tickets/[id]/index.tsx` 中 `progressMut` 的 `onMutate`：立即关闭 Sheet、重置表单和 activityId
- [x] 3.2 `onSuccess` 只保留 `invalidateTicket()` + toast；`onError` 保留 `invalidateTicket()` + toast.error
- [x] 3.3 手动验证：点击审批按钮后 Sheet 立即关闭、页面进入决策中状态
