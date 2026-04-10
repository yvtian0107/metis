## Why

Metis 当前依赖 `.env` 文件手动配置数据库、密钥等基础设施参数，并通过 CLI 命令（`seed`、`create-admin`）初始化系统。这对非技术用户不友好，也无法支撑未来作为产品交付的体验。需要一个 WordPress 风格的安装向导，让用户下载二进制后"打开即用"——浏览器引导完成数据库选择、管理员创建等全部初始化工作。

## What Changes

- **BREAKING** 删除 `.env` / `.env.example` / `godotenv` 依赖，不再从环境变量读取配置
- **BREAKING** 删除 `seed` CLI 子命令和 `create-admin` CLI 子命令
- **BREAKING** 删除 `make init-dev-user` Makefile target
- 新增 `metis.yaml` 配置文件（类似 WordPress 的 `wp-config.php`），仅存放数据库连接和安全密钥，由安装向导自动生成
- 新增安装向导后端 API（`/api/v1/install/*`），处理数据库测试、配置写入、系统初始化
- 新增安装向导前端页面（`/install`），多步骤表单：数据库选择 → 站点信息 → 管理员账号 → 完成
- 改造 `main.go` 启动流程：检测 `metis.yaml` 是否存在 + DB 中 `app.installed` 标记，未安装时进入安装模式（只暴露安装相关路由）
- 原 `.env` 中的 `SERVER_PORT`、`OTEL_*` 等应用级配置迁入 DB `SystemConfig` 表
- `JWT_SECRET` 和 `LICENSE_KEY_SECRET` 改为安装时随机生成并写入 `metis.yaml`
- Seed 逻辑拆分：安装时全量初始化（roles + menus + policies + configs + admin），后续启动增量同步（只补新增的 roles/menus/policies）

## Capabilities

### New Capabilities

- `install-wizard-api`: 安装向导后端 API——检测安装状态、测试数据库连接、执行安装、生成 metis.yaml
- `install-wizard-ui`: 安装向导前端页面——多步骤引导表单，复用 AuthShell 玻璃态风格
- `config-file`: metis.yaml 配置文件的读取、生成、结构定义（替代 .env）

### Modified Capabilities

- `server-bootstrap`: 启动流程改为两阶段检测（metis.yaml 存在性 + app.installed 标记），未安装时进入安装模式
- `seed-init`: 删除 CLI 入口，拆分为安装时全量初始化和启动时增量同步两个路径
- `database`: 不再从环境变量读取 DB_DRIVER/DB_DSN，改为从 metis.yaml 读取
- `telemetry`: 不再从环境变量读取 OTEL_* 配置，改为从 DB SystemConfig 读取（初始化时机推迟到 DB 连接之后）
- `system-config`: 新增 server_port、otel_* 等配置项的默认值和管理

## Impact

- **后端**: `cmd/server/main.go`（启动流程重构）、`internal/seed/`（拆分逻辑）、`internal/database/`（配置源变更）、`internal/telemetry/`（初始化时机）、新增 `internal/config/`（metis.yaml 读写）、新增安装 handler/service
- **前端**: 新增 `web/src/pages/install/` 安装向导页面、修改路由配置和 App.tsx 增加安装状态检测
- **API**: 新增 `GET /api/v1/install/status`、`POST /api/v1/install/check-db`、`POST /api/v1/install/execute`
- **依赖**: 移除 `github.com/joho/godotenv`，新增 `gopkg.in/yaml.v3`（已在 go.sum 中作为间接依赖）
- **文件**: 删除 `.env.example`，新增运行时生成的 `metis.yaml`（应加入 `.gitignore`）
- **开发流程**: 开发者首次 `make dev` 后在浏览器完成安装向导（约 30 秒），替代原来的 `make init-dev-user`
