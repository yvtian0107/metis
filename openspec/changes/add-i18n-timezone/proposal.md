## Why

Metis 目前所有 UI 文本硬编码中文，无法服务非中文用户。时间显示固定使用 `zh-CN` locale，无时区转换。需要像 WordPress 一样支持系统级和用户级的多语言与多时区，使 Metis 成为一个可国际化部署的产品。

## What Changes

- 前端引入 `react-i18next`，所有 UI 字符串替换为翻译函数调用 `t('key')`
- 后端错误消息改为返回 error key，前端负责翻译展示
- 后端引入 `go-i18n` 用于邮件/通知等需要服务端翻译的场景
- 翻译文件按模块就近存放：内核在 `web/src/i18n/locales/`，App 在 `web/src/apps/<name>/locales/`
- User 模型新增 `locale` 和 `timezone` 字段
- SystemConfig 新增 `system.locale` 和 `system.timezone` 配置
- 安装向导新增第一步：语言 + 时区选择（在数据库配置之前，纯前端）
- 系统设置页新增语言和时区配置项
- 用户个人设置支持语言和时区偏好
- `formatDateTime` 改为动态读取用户 locale 和 timezone
- 初期支持 `zh-CN` 和 `en` 两种语言
- 内置菜单/角色名称通过 permission key 映射翻译，用户自建内容不翻译
- **BREAKING**: 后端 API 错误消息从人类可读文本改为 error key

## Capabilities

### New Capabilities
- `i18n-frontend`: 前端国际化基础设施 — react-i18next 初始化、翻译文件组织、App 模块翻译注册机制、locale 解析优先级（用户偏好 → 系统默认 → 浏览器 → zh-CN）
- `i18n-backend`: 后端国际化基础设施 — error key 体系、go-i18n 集成（邮件/通知翻译）、App 接口扩展 Locales() 方法
- `timezone-support`: 时区支持 — 数据库存 UTC、前端按用户时区格式化显示、formatDateTime 动态化、Intl.DateTimeFormat 动态 locale/timezone

### Modified Capabilities
- `install-wizard-ui`: 安装向导新增第一步语言 + 时区选择，在数据库配置之前
- `install-wizard-api`: 安装接口接收 locale 和 timezone 参数并写入 SystemConfig
- `user-management`: User 模型新增 locale 和 timezone 字段，个人设置页支持语言/时区偏好
- `settings-page`: 系统设置页新增语言和时区配置区域
- `system-config`: 新增 system.locale 和 system.timezone 配置键
- `seed-init`: 种子数据中内置菜单/角色的 title 保持中文，前端通过 key 映射翻译
- `shared-ui-patterns`: 通用 UI 模式（空状态、加载状态、确认框等）全部走翻译
- `user-auth-frontend`: 登录/注册/2FA 页面全部走翻译

## Impact

- **前端**: 所有页面和组件需要替换硬编码文本为 `t()` 调用，新增 `react-i18next` + 翻译 JSON 文件
- **后端**: User 模型 migration、error 消息重构为 key、新增 `go-i18n` 依赖、App 接口扩展
- **API**: 错误响应 `message` 字段从中文文本变为 error key（如 `"error.auth.invalid_credentials"`），前端需适配
- **数据库**: User 表新增两列（locale, timezone）、SystemConfig 新增两行
- **构建**: 翻译 JSON 文件随 Vite 打包，App 裁剪时翻译跟随裁剪
- **安装流程**: 新增一个步骤，现有步骤序号后移
