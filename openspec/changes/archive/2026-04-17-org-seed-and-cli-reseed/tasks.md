## 1. App 接口变更

- [x] 1.1 修改 `internal/app/app.go` 中 `App` 接口的 `Seed` 方法签名，增加 `install bool` 参数
- [x] 1.2 更新 AI App (`internal/app/ai/`) 的 `Seed` 方法签名
- [x] 1.3 更新 APM App (`internal/app/apm/`) 的 `Seed` 方法签名
- [x] 1.4 更新 ITSM App (`internal/app/itsm/`) 的 `Seed` 方法签名
- [x] 1.5 更新 License App (`internal/app/license/`) 的 `Seed` 方法签名
- [x] 1.6 更新 Node App (`internal/app/node/`) 的 `Seed` 方法签名
- [x] 1.7 更新 Observe App (`internal/app/observe/`) 的 `Seed` 方法签名
- [x] 1.8 更新 Org App (`internal/app/org/`) 的 `Seed` 方法签名

## 2. 调用方适配

- [x] 2.1 修改 `cmd/server/main.go` 正常启动流程中的 `App.Seed()` 调用，传入 `install=false`
- [x] 2.2 修改 `internal/handler/install.go` hotSwitch 中的 `App.Seed()` 调用，传入 `install=true`

## 3. Org App 内置种子数据

- [x] 3.1 在 `internal/app/org/seed.go` 中添加 `seedDepartments(db)` 函数：按 code 幂等创建 7 个部门（先建总部根节点，再建 6 个子部门）
- [x] 3.2 在 `internal/app/org/seed.go` 中添加 `seedPositions(db)` 函数：按 code 幂等创建 7 个岗位
- [x] 3.3 在 `seedOrg()` 中根据 `install` 参数条件调用 `seedDepartments` 和 `seedPositions`

## 4. CLI seed 子命令

- [x] 4.1 在 `cmd/server/main.go` 中添加 `os.Args` 子命令检测逻辑（seed 分支 vs 正常启动）
- [x] 4.2 实现 `seed` 子命令分支：加载 config → 连接 DB → AutoMigrate → 初始化 Casbin → 执行 seed.Sync() → 执行 App.Seed(install=true) → 退出
- [x] 4.3 处理错误情况：config 不存在、DB 连接失败、未知子命令

## 5. 验证

- [x] 5.1 `go build -tags dev ./cmd/server/` 编译通过
- [x] 5.2 `go test ./internal/app/org/... -v` 测试通过
- [x] 5.3 `go test ./... -count=1` 全量测试通过
