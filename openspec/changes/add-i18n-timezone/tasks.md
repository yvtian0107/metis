## 1. 前端 i18n 基础设施

- [x] 1.1 安装 `i18next` 和 `react-i18next` 依赖（bun add）
- [x] 1.2 创建 `web/src/i18n/index.ts` — i18next 初始化（fallbackLng: zh-CN, supported: [zh-CN, en], defaultNS: common）
- [x] 1.3 创建内核翻译文件目录结构 `web/src/i18n/locales/{zh-CN,en}/`，创建空的 `common.json`
- [x] 1.4 实现 `registerTranslations(ns, resources)` 函数供 App 模块注册翻译
- [x] 1.5 在 `web/src/main.tsx` 中导入 i18n 初始化（在 React 渲染之前）
- [x] 1.6 实现 locale 解析优先级逻辑：user.locale → system.locale → navigator.language → zh-CN

## 2. 后端 error key 体系

- [x] 2.1 将 `internal/service/auth.go` 中的 sentinel error 改为 error key 格式（`error.auth.*`）
- [x] 2.2 将 `internal/service/user.go` 中的 sentinel error 改为 error key 格式（`error.user.*`）
- [x] 2.3 扫描并转换所有其他 service 中的硬编码错误消息为 error key
- [x] 2.4 确保 `handler.Fail()` 传递 error key 而非翻译后的文本
- [x] 2.5 创建前端 `errors.json` 翻译文件（zh-CN 和 en），覆盖所有 error key

## 3. 后端 User 模型扩展

- [x] 3.1 User 模型新增 `Locale string` 和 `Timezone string` 字段（GORM tag: size:10 / size:50）
- [x] 3.2 `ToResponse()` 方法输出 locale 和 timezone 字段
- [x] 3.3 用户创建/更新 API 接受 locale 和 timezone 参数
- [x] 3.4 实现用户个人 profile 更新端点（`PUT /api/v1/auth/profile`），支持 locale 和 timezone
- [x] 3.5 确保 `GET /api/v1/auth/me` 返回 locale 和 timezone，前端 User 接口已包含

## 4. 后端 SystemConfig 扩展

- [x] 4.1 `seed.Install()` 中新增 `system.locale` 和 `system.timezone` 配置项的创建
- [x] 4.2 `seed.Sync()` 中对新配置项做幂等检查（只在不存在时创建，不覆盖）
- [x] 4.3 Install API (`POST /api/v1/install/execute`) 接收 locale 和 timezone 参数并写入 SystemConfig
- [x] 4.4 Site info API 响应中包含 `locale` 和 `timezone` 字段

## 5. 前端时区支持

- [x] 5.1 重构 `web/src/lib/utils.ts` 中的 `formatDateTime`：接受 locale 和 timezone 参数，使用 `Intl.DateTimeFormat` 动态格式化
- [x] 5.2 创建 timezone 解析逻辑：user.timezone → system.timezone → browser timezone → UTC
- [x] 5.3 更新 auth store：登录后从用户信息中读取 locale 和 timezone 并存储
- [x] 5.4 替换所有组件中 hardcoded 的 `toLocaleString("zh-CN")` / `toLocaleDateString("zh-CN")` 为 `formatDateTime`

## 6. 安装向导 i18n

- [x] 6.1 创建 `web/src/i18n/locales/zh-CN/install.json` 和 `en/install.json`（提取所有安装向导文本）
- [x] 6.2 新增安装向导第一步组件：语言选择（支持语言列表 + 原生名称）+ 时区选择（IANA 时区列表，按区域分组，浏览器检测默认值）
- [x] 6.3 重构安装向导步骤指示器为 5 步（语言 → 数据库 → 站点 → 管理员 → 完成）
- [x] 6.4 替换安装向导所有硬编码中文为 `t('install:...')` 调用
- [x] 6.5 语言选择联动：选完语言后 `i18next.changeLanguage()` 立即切换后续步骤语言

## 7. 登录/注册/2FA 页面 i18n

- [x] 7.1 创建 `auth.json` 翻译文件（zh-CN 和 en），提取登录页所有文本
- [x] 7.2 替换登录页 (`pages/login/`) 所有硬编码文本为 `t()` 调用
- [x] 7.3 替换注册页 (`pages/register/`) 所有硬编码文本为 `t()` 调用
- [x] 7.4 替换 2FA 验证页 (`pages/2fa/`) 所有硬编码文本为 `t()` 调用
- [x] 7.5 替换 2FA 设置对话框 (`components/two-factor-setup-dialog.tsx`) 所有硬编码文本
- [x] 7.6 替换修改密码对话框 (`components/change-password-dialog.tsx`) 所有硬编码文本
- [x] 7.7 在登录/注册/2FA 页面添加语言切换器组件

## 8. 通用 UI 词汇翻译

- [x] 8.1 创建 `common.json` 翻译文件（zh-CN 和 en）：按钮标签、状态词、确认框、分页、表单验证
- [x] 8.2 替换所有页面中通用按钮文本（保存/取消/删除/编辑/新建/搜索/确认/关闭/启用/停用）为 `t('common:...')`
- [x] 8.3 替换所有加载状态文本（保存中/加载中/删除中/登录中/注册中）为 `t('common:...')`
- [x] 8.4 替换 DataTable 组件中的分页和空状态文本

## 9. 内核页面 i18n（逐页迁移）

- [x] 9.1 用户管理页 (`pages/users/`)：创建 `users.json`，替换所有文本，内置角色名通过 role code 查翻译
- [x] 9.2 角色管理页 (`pages/roles/`)：创建 `roles.json`，替换所有文本
- [x] 9.3 菜单管理页 (`pages/menus/`)：创建 `menus.json`，替换所有文本，内置菜单名通过 permission 查翻译
- [x] 9.4 系统设置页 (`pages/settings/`)：创建 `settings.json`，替换所有文本，新增语言和时区设置区域
- [x] 9.5 任务管理页 (`pages/tasks/`)：创建 `tasks.json`，替换所有文本
- [x] 9.6 审计日志页 (`pages/audit-logs/`)：创建 `audit.json`，替换所有文本
- [x] 9.7 会话管理页 (`pages/sessions/`)：复用 `common.json` 或创建专用翻译
- [x] 9.8 公告管理页 (`pages/announcements/`)：创建翻译并替换
- [x] 9.9 通知渠道页 (`pages/channels/`)：创建翻译并替换
- [x] 9.10 认证提供者页 (`pages/auth-providers/`)：创建翻译并替换
- [x] 9.11 身份源管理页 (`pages/identity-sources/`)：创建翻译并替换
- [x] 9.12 首页/Dashboard (`pages/home/`)：创建翻译并替换
- [x] 9.13 侧边栏导航 + 用户菜单（DashboardLayout）：替换所有文本，菜单名通过 permission 映射翻译

## 10. App 模块 i18n

- [x] 10.1 AI App：在 `web/src/apps/ai/locales/` 创建翻译文件，`module.ts` 调用 `registerTranslations`，替换所有页面文本
- [x] 10.2 License App：在 `web/src/apps/license/locales/` 创建翻译文件，`module.ts` 调用 `registerTranslations`，替换所有页面文本

## 11. 后端通知翻译（go-i18n）

- [x] 11.1 添加 `go-i18n` v2 依赖（go get）
- [x] 11.2 创建 `internal/locales/zh-CN.json` 和 `en.json` 内核通知翻译文件
- [x] 11.3 实现翻译 service 并注册到 IOC 容器
- [x] 11.4 App 接口扩展可选 `Locales() embed.FS` 方法，main.go 启动时加载
- [x] 11.5 邮件 channel 发送时根据接收者 locale 选择翻译

## 12. 集成验证

- [ ] 12.1 验证安装向导完整流程：语言选择 → 数据库 → 站点 → 管理员 → 完成（中英文各走一遍）
- [ ] 12.2 验证登录后 locale/timezone 从用户信息正确加载并切换 UI 语言
- [ ] 12.3 验证系统设置中修改默认语言和时区后，新会话生效
- [ ] 12.4 验证用户个人设置中修改语言和时区后，UI 立即切换
- [ ] 12.5 验证时间显示在不同时区下正确转换
- [ ] 12.6 验证 error key 在中英文下均正确翻译显示
- [x] 12.7 验证 App 模块裁剪后（APPS=system），App 翻译不被打包
- [x] 12.8 验证 `go build -tags dev ./cmd/server/` 编译通过
