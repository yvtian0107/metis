## 1. 后端 — Version 包

- [x] 1.1 创建 `internal/version/version.go`，定义 `Version`（默认 `"dev"`）、`GitCommit`、`BuildTime` 包级变量

## 2. 构建 — Makefile ldflags

- [x] 2.1 在 Makefile 中添加 `VERSION`、`GIT_COMMIT`、`BUILD_TIME` 变量检测逻辑
- [x] 2.2 在 `build` 和 `release` 目标中增加 `-ldflags "-X ..."` 注入三个变量
- [x] 2.3 验证 `make build` 后二进制包含正确版本号（`go build -tags dev` 快速验证）

## 3. 后端 — 扩展 site-info API

- [x] 3.1 修改 `internal/handler/site_info.go`，在 `GetSiteInfo` 响应中增加 `version`、`gitCommit`、`buildTime` 字段（从 `version` 包读取）

## 4. 前端 — 类型与数据

- [x] 4.1 修改 `web/src/lib/api.ts`，`SiteInfo` 接口增加 `version`、`gitCommit`、`buildTime` 字段

## 5. 前端 — UI 展示

- [x] 5.1 修改 `web/src/components/layout/top-nav.tsx`，在用户下拉菜单底部添加版本号文本（灰色小字，不可点击）
- [x] 5.2 修改登录页组件，在右下角固定位置显示版本号（灰色小字）
