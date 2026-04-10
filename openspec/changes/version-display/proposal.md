## Why

系统当前没有任何版本号展示，运维和用户无法确认正在运行的是哪个版本。需要在 UI 中展示构建版本号，便于问题排查和版本确认。

## What Changes

- 新增 `internal/version/` 包，定义 `Version`、`GitCommit`、`BuildTime` 变量（通过 `ldflags` 编译时注入）
- Makefile 增加版本号自动检测逻辑：有 git tag 用 tag，无 tag 用 `nightly-YYYYMMDD-<commit>`
- 扩展 `GET /api/v1/site-info` 响应，增加 `version`、`gitCommit`、`buildTime` 字段
- 用户菜单（TopNav 下拉）底部展示版本号（灰色小字）
- 登录页右下角展示版本号

## Capabilities

### New Capabilities
- `build-version`: 编译时版本号注入机制（Makefile + Go ldflags + version 包）

### Modified Capabilities
- `site-info-api`: 响应增加 `version`、`gitCommit`、`buildTime` 字段

## Impact

- **构建**: Makefile 增加 ldflags，不影响现有构建流程
- **后端**: 新增 `internal/version/` 包，修改 `site_info.go` handler
- **前端**: 修改 `api.ts` 类型定义、`top-nav.tsx` 用户菜单、登录页组件
- **API**: `GET /api/v1/site-info` 响应增加字段（向后兼容，仅新增）
