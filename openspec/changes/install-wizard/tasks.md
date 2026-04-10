## 1. 配置文件基础设施

- [ ] 1.1 创建 `internal/config/config.go`：MetisConfig 结构体、Load()、Save()、DefaultSQLiteConfig()、GenerateSecrets() 函数
- [ ] 1.2 移除 `godotenv` 依赖：删除 `.env.example`，从 `go.mod` 移除 `github.com/joho/godotenv`，运行 `go mod tidy`
- [ ] 1.3 在 `.gitignore` 中添加 `metis.yaml` 和 `metis.db`

## 2. 启动流程改造

- [ ] 2.1 改造 `cmd/server/main.go`：移除 godotenv.Load()，移除 seed/create-admin CLI 子命令，新增 metis.yaml 检测逻辑
- [ ] 2.2 实现安装模式启动路径：无 metis.yaml 或 app.installed!=true 时，只注册 install 路由 + SPA 静态资源
- [ ] 2.3 实现正常模式启动路径：从 metis.yaml 读取 DB + secrets 配置，从 DB SystemConfig 读取 server_port
- [ ] 2.4 改造 `internal/database/database.go`：接收 MetisConfig 参数代替读取环境变量

## 3. Seed 逻辑拆分

- [ ] 3.1 将 `internal/seed/seed.go` 的 `Run()` 拆分为 `Install()` 和 `Sync()` 两个函数
- [ ] 3.2 `Install()`: 全量初始化（roles + menus + policies + default configs + auth providers + 新增 server_port/otel.*/site.name/app.installed 配置）
- [ ] 3.3 `Sync()`: 增量同步（只补充新的 roles/menus/policies，不覆盖已有 SystemConfig）
- [ ] 3.4 删除 `internal/seed/migrate.go`（不再需要 legacy user role 迁移）

## 4. OTel 改造

- [ ] 4.1 改造 `internal/telemetry/telemetry.go`：从 SystemConfig 读取 otel.enabled/otel.exporter_endpoint/otel.service_name/otel.sample_rate，不再读环境变量
- [ ] 4.2 调整 main.go 中 OTel 初始化时机：移到 DB 连接之后

## 5. SystemConfig 扩展

- [ ] 5.1 在 seed Install() 中新增默认配置项：server_port=8080, otel.enabled=false, otel.exporter_endpoint, otel.service_name=metis, otel.sample_rate=1.0, site.name=Metis
- [ ] 5.2 新增 app.installed 标记的读写逻辑（在 SysConfigService 或 InstallService 中）

## 6. 安装向导后端 API

- [ ] 6.1 创建 `internal/handler/install.go`：InstallHandler 结构体，注册到 IOC
- [ ] 6.2 实现 `GET /api/v1/install/status`：检查 app.installed 标记，返回安装状态
- [ ] 6.3 实现 `POST /api/v1/install/check-db`：接收 PostgreSQL 连接参数，测试连接，返回成功/失败
- [ ] 6.4 实现 `POST /api/v1/install/execute`：执行完整安装流程（生成 secrets → 写 metis.yaml → AutoMigrate → seed.Install → 创建管理员 → 设置 app.installed=true → 热切换到正常模式）
- [ ] 6.5 实现热切换逻辑：安装完成后在同一进程内注册所有业务 providers、routes、启动 scheduler

## 7. 安装向导前端页面

- [ ] 7.1 创建 `web/src/pages/install/index.tsx`：安装向导主页面，管理步骤状态
- [ ] 7.2 实现数据库选择步骤组件：SQLite/PostgreSQL 切换，PostgreSQL 连接表单 + 测试连接按钮
- [ ] 7.3 实现站点信息步骤组件：站点名称输入
- [ ] 7.4 实现管理员账号步骤组件：用户名/密码/确认密码/邮箱表单，Zod 校验
- [ ] 7.5 实现完成步骤组件：安装摘要 + "开始安装" 按钮 + loading 状态 + 成功后"进入系统"跳转
- [ ] 7.6 实现步骤指示器组件：水平步骤条，当前步骤高亮，已完成步骤打勾
- [ ] 7.7 使用 AuthShell 布局 + 玻璃态卡片，匹配现有登录页视觉风格

## 8. 前端路由集成

- [ ] 8.1 在 App.tsx / 路由配置中添加 `/install` 路由
- [ ] 8.2 实现安装状态检测：应用启动时调用 `GET /api/v1/install/status`，未安装则重定向到 `/install`
- [ ] 8.3 实现已安装保护：访问 `/install` 时如已安装则重定向到 `/login`

## 9. Makefile 和构建清理

- [ ] 9.1 删除 Makefile 中的 `init-dev-user` target
- [ ] 9.2 删除 `.env.example` 文件
- [ ] 9.3 更新 CLAUDE.md 文档：移除 .env 相关说明，更新开发流程描述
