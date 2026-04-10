## Context

Metis 编译为单二进制分发，目前没有任何版本标识。运维无法确认运行的版本，用户也看不到版本信息。需要在编译时注入版本号，通过已有的 `site-info` API 暴露，在前端两个位置展示。

现有基础设施：
- `GET /api/v1/site-info` 已返回 `appName` 和 `hasLogo`
- Makefile 已有 `build` / `release` 目标，但未使用 `ldflags`
- TopNav 用户下拉菜单和登录页已就绪

## Goals / Non-Goals

**Goals:**
- 编译时自动从 git 状态生成版本号并注入二进制
- 通过 site-info API 暴露版本元数据
- 在用户菜单底部和登录页右下角展示版本号

**Non-Goals:**
- 不做 CHANGELOG / Release Note 展示
- 不做 About 弹窗或独立页面
- 不做版本更新检查或通知

## Decisions

### D1: 版本号生成策略
使用 Makefile 在编译时检测 git 状态：
- `git describe --tags --exact-match` 成功 → 使用 tag（如 `v1.2.0`）
- 失败 → `nightly-YYYYMMDD-<7位commit hash>`

**理由**: 不依赖运行时 git，单二进制部署时版本号已嵌入。开发模式(`make dev`)变量保持零值，显示 `dev`。

### D2: 注入方式 — Go ldflags
新建 `internal/version/version.go`，定义包级变量 `Version`、`GitCommit`、`BuildTime`，通过 `-ldflags "-X ..."` 覆盖。

**理由**: Go 标准做法，零运行时开销，不引入额外依赖。

### D3: 复用 site-info API
在现有 `GET /api/v1/site-info` 响应中增加 `version`、`gitCommit`、`buildTime` 字段。

**备选**: 新建 `/api/v1/version` 端点。
**理由**: site-info 已是公开端点，前端已在 TopNav 和登录页获取它，加字段最简单，无需额外请求。

### D4: 前端展示方式
- 用户菜单：在"退出登录"下方加一行灰色小字版本号
- 登录页：右下角固定定位灰色小字版本号

两处只显示 `version` 字段（如 `nightly-20260410-abc1234`），不显示 buildTime 等详细信息。

**理由**: 简洁，满足当前需求，后续可扩展为可点击的 About 弹窗。

## Risks / Trade-offs

- **开发模式显示 `dev`**: `make dev` 不经过 ldflags，版本号为默认值 `dev`。这是预期行为，开发者可以区分。
- **tag 格式无校验**: 直接使用 git tag 值，不校验是否符合 semver。低风险——tag 由项目维护者控制。
