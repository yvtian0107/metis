## Why

Org App 当前只 seed 菜单和 Casbin 策略，没有内置部门和岗位数据。新安装后组织架构是空的，用户体验不够开箱即用。同时 `App.Seed()` 接口不区分首次安装和日常启动，无法支持"仅首次安装时创建"的种子数据模式。开发阶段也没有 CLI 命令来手动重跑 seed，调试不方便。

## What Changes

- **`App` 接口扩展**：`Seed(db, enforcer)` 改为 `Seed(db, enforcer, install bool)`，所有 App 实现同步更新签名
- **Org App 补充种子数据**：首次安装时创建 7 个内置部门（总部 + 6 个子部门）和 7 个内置岗位，数据来源于 bklite-cloud 已验证的配置
- **CLI `seed` 子命令**：`./server seed` 手动触发完整 seed（kernel + 所有 App），开发调试用
- **Kernel `seed.Sync()` 同步调用** `App.Seed(db, enforcer, false)` 以保持一致性

## Capabilities

### New Capabilities

- `org-builtin-seed`: Org App 内置部门和岗位的种子数据定义与幂等创建逻辑
- `cli-seed-command`: CLI `seed` 子命令，手动触发完整 seed 流程

### Modified Capabilities

- `seed-init`: App.Seed 接口增加 `install bool` 参数，区分首次安装和日常同步

## Impact

- **接口变更**：所有实现 `app.App` 的模块（AI、Org、ITSM、Node、APM、Observe、License）需更新 `Seed` 方法签名
- **入口文件**：`cmd/server/main.go` 增加子命令解析 + `seed` 分支
- **Install handler**：`internal/handler/install.go` hotSwitch 中调用 `App.Seed` 传 `install=true`
- **Org seed**：`internal/app/org/seed.go` 增加部门和岗位创建函数
- **无 breaking change**：现有数据库不受影响，seed 逻辑全部幂等
