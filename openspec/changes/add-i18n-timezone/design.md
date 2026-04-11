## Context

Metis 当前所有 UI 文本硬编码中文，后端错误消息中英混杂，时间显示固定 `zh-CN` 且无时区转换。需要实现 WordPress 风格的多语言多时区支持：系统级默认 + 用户级覆盖，数据库永远存 UTC。

前端是 React 19 SPA（Vite + TypeScript），后端是 Go（Gin + GORM），架构已有可插拔 App 模块。翻译文件需要与 App 模块就近存放，随模块裁剪自动排除。

## Goals / Non-Goals

**Goals:**
- 前端所有 UI 字符串可翻译，初期支持 zh-CN 和 en
- 后端错误消息改为 error key 体系，前端负责翻译展示
- 后端邮件/通知场景支持服务端翻译
- 安装向导第一步选语言和时区（纯前端，无需数据库）
- 系统设置可修改默认语言和时区
- 用户个人可设置偏好语言和时区
- 翻译文件按 App 就近存放，构建裁剪时自动排除
- 所有时间显示根据用户时区动态转换

**Non-Goals:**
- 不做内容翻译（用户自建的菜单名、角色名、公告等保持原文）
- 不做 RTL（右到左）布局支持
- 不做超过 zh-CN / en 的语言支持（架构支持扩展，但初期只做两种）
- 不做后端 API 路径的国际化
- 不做数据库存储时区字段（始终 UTC）

## Decisions

### D1: 前端翻译库选 react-i18next

**选择**: `react-i18next` + `i18next`

**备选**:
- `react-intl` (FormatJS): API 更 verbose，社区生态不如 i18next
- 自研轻量方案: 维护成本高，缺少复数/插值/namespace 等特性

**理由**: react-i18next 是 React 生态最成熟的 i18n 方案，支持 namespace（与 App 模块对应）、lazy loading、fallback、插值、复数，TypeScript 支持好。WordPress 前端用 `wp.i18n`（类 gettext），但 SPA 生态中 i18next 是对等的标准选择。

### D2: 翻译 key 使用结构化 key（非 gettext 风格）

**选择**: `t('users.title')` 结构化 key

**备选**:
- gettext 风格 `t('User Management')` 以英文原文做 key: WordPress 传统方案

**理由**: 结构化 key 在 TypeScript + JSON 体系下更优：key 稳定不随文案变化，JSON namespace 天然分组，IDE 自动补全友好。WordPress 用 gettext 风格是历史原因（PHP 生态），现代 SPA 普遍采用结构化 key。

### D3: 翻译文件按模块就近存放

**结构**:
```
web/src/i18n/
├── index.ts                    # i18next 初始化
└── locales/                    # 内核翻译
    ├── zh-CN/
    │   ├── common.json         # 通用词（按钮、状态、确认）
    │   ├── auth.json           # 登录/注册/2FA
    │   ├── install.json        # 安装向导
    │   ├── users.json          # 用户管理
    │   ├── roles.json          # 角色管理
    │   ├── menus.json          # 菜单管理
    │   ├── settings.json       # 系统设置
    │   ├── tasks.json          # 任务调度
    │   ├── audit.json          # 审计日志
    │   └── errors.json         # 后端 error key 翻译
    └── en/
        └── ... (同结构)

web/src/apps/<name>/locales/    # App 翻译
├── zh-CN/<name>.json
└── en/<name>.json
```

**理由**: App 模块自包含翻译，`gen-registry.sh` 裁剪 App 时翻译自动排除。内核翻译按功能页面拆分 namespace，单文件不会过大。

### D4: 后端错误消息改为 error key

**选择**: API 返回 `{"code": -1, "message": "error.auth.invalid_credentials"}`，前端在 `errors.json` 中查翻译

**备选**:
- 后端根据 Accept-Language 返回翻译后的消息: 增加后端复杂度，且 SPA 不走服务端渲染
- 保持中文消息 + 前端不翻译错误: 非中文用户无法理解

**理由**: 借鉴 WordPress 的 "翻译离用户最近" 原则。SPA 已经有完整的翻译基础设施，error key 只是多一个 namespace。后端保持无状态（不需要知道用户语言），代码更简洁。

**error key 命名规范**: `error.<domain>.<specific>`，如 `error.auth.invalid_credentials`、`error.user.not_found`、`error.user.username_exists`。

### D5: Locale 解析优先级

```
用户 user.locale
    ↓ 空则
系统 system_config["system.locale"]
    ↓ 空则
浏览器 navigator.language
    ↓ 不在支持列表中则
"zh-CN" (硬编码 fallback)
```

时区同理: `user.timezone → system.timezone → Intl.DateTimeFormat().resolvedOptions().timeZone → "UTC"`

与 WordPress 的 `user meta locale → WPLANG → en_US` 完全一致。

### D6: 内置菜单/角色翻译策略

**选择**: 数据库中菜单 title 保持中文（现状不变），前端用 permission key 映射翻译 key

```
DB: { title: "用户管理", permission: "system:user:list" }
前端: t(`menu.${permission.replace(/:/g, '.')}`, { defaultValue: title })
→ 有翻译用翻译，无翻译 fallback 到 DB 原文
```

**理由**: 不改数据库 schema，不改 seed 逻辑。内置菜单数量固定（~15 个），前端维护翻译映射表很轻量。用户自建菜单自然 fallback 到 DB 原文，零额外处理。

### D7: 后端翻译仅限通知场景

**选择**: 引入 `go-i18n` v2，仅用于 channel（邮件/通知）发送时翻译

**理由**: 当前后端通知 channel 有 email driver，未来可能有更多。通知内容需要根据接收者的 locale 翻译，这是唯一需要后端翻译的场景。App 接口扩展可选的 `Locales() embed.FS` 方法。

### D8: 安装向导语言选择在第一步

**选择**: 在数据库配置之前加一步纯前端的语言 + 时区选择

**理由**: 借鉴 WordPress 安装流程。语言选择不需要数据库，纯前端切换 i18next 的 `changeLanguage()`。选完后后续步骤立即用该语言显示。时区默认从浏览器检测 `Intl.DateTimeFormat().resolvedOptions().timeZone`，用户可修改。

## Risks / Trade-offs

- **[工作量大]** 所有现有页面都需要替换硬编码文本 → 按模块逐步迁移，优先迁移安装向导和登录页，其余页面可以分批完成
- **[BREAKING API]** error message 从中文变为 key → 需要前端同步更新，否则用户看到 raw key。通过 errors.json fallback 机制降低风险：如果 key 在翻译文件中找不到，直接显示 key 值
- **[翻译遗漏]** 某些字符串可能被遗漏未提取 → ESLint 可以加规则检测 JSX 中的中文字面量（后续优化）
- **[seed 数据迁移]** 已有部署的系统菜单 title 是中文 → 不需要迁移，前端 fallback 策略兼容
- **[复数/性别等高级 i18n]** 初期只做简单翻译 → i18next 架构天然支持复数，后续按需启用

## Open Questions

- 日期格式是否需要用户自定义（如 WordPress 的 `date_format` / `time_format`）？初期建议跟随 locale 自动选择格式，不做自定义配置。
