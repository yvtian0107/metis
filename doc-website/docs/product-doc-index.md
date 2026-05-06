---
title: 产品文档索引（自动维护）
unlisted: true
---

# 产品文档索引（自动维护）

最后更新：2026-05-06 18:40 (+08:00)
本轮范围：系统管理

## 已发现结构

### 业务域：系统管理

- 模块：用户管理（/users）
- 模块：角色管理（/roles）
- 模块：菜单管理（/menus）
- 模块：会话管理（/sessions）
- 模块：系统设置（/settings）
- 模块：任务管理（/tasks）
- 模块：公告管理（/announcements）
- 模块：通知渠道（/channels）
- 模块：认证源（/auth-providers）
- 模块：审计日志（/audit-logs）
- 模块：身份源（/identity-sources）

## 已生成文档

- 系统管理 / [用户管理](./system-management/user-management.md)
- 系统管理 / [角色管理](./system-management/role-management.md)
- 系统管理 / [菜单管理](./system-management/menu-management.md)（本轮新增）
- 系统管理 / [会话管理](./system-management/session-management.md)
- 系统管理 / [系统设置](./system-management/system-settings.md)
- 系统管理 / [任务管理](./system-management/task-management.md)（本轮新增）
- 系统管理 / [公告管理](./system-management/announcement-management.md)（本轮新增）
- 系统管理 / [通知渠道](./system-management/channel-management.md)（本轮新增）
- 系统管理 / [认证源](./system-management/auth-provider-management.md)
- 系统管理 / [身份源管理](./system-management/identity-source-management.md)（本轮新增）
- 系统管理 / [审计日志](./system-management/audit-log-management.md)

## 待补齐（按优先级）

当前无待补齐模块。

## 本轮处理说明

- 复核范围：系统设置（`/settings`）。
- 证据对照：前端路由（`web/src/App.tsx`）、页面结构（`web/src/pages/settings/index.tsx`、`web/src/pages/settings/security-card.tsx`）、中文文案（`web/src/i18n/locales/zh-CN/settings.json`）。
- 结论：当前文档与代码一致，本轮无正文改动；继续保持“功能概览 / 功能清单 / 常见任务操作 / 风险与注意事项”四段结构与模块互链。
- 覆盖状态：系统管理域 11 个模块均已覆盖，待补齐列表保持为空。