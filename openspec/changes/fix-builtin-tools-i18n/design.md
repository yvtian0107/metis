## Context

前端 `builtin-tools-tab.tsx` 使用动态 i18n key 展示内建工具：
- Toolkit 级：`t('ai:tools.toolkits.${toolkit}.name')` / `.description` — **无 fallback**
- Tool 级：`t('ai:tools.toolDefs.${name}.name', displayName)` / `.description` — 有 fallback 到后端字段

当前 locale 文件（`web/src/apps/ai/locales/{zh-CN,en}.json`）只覆盖了 `knowledge`、`network`、`code` 三个 Toolkit 和 5 个工具。后续新增的 4 个 Toolkit（`general`、`itsm`、`decision`、`organization`）及 19 个工具完全缺失。

## Goals / Non-Goals

**Goals:**
- 补全所有内建工具的 zh-CN 和 en locale 条目，消除 raw key 显示

**Non-Goals:**
- 不改前端组件逻辑或 i18n key 拼接方式
- 不改后端 seed 数据
- 不做 i18n key 自动生成机制

## Decisions

1. **翻译来源**：直接参考后端 seed 中的 `DisplayName` 和 `Description`，保持语义一致；英文翻译在中文基础上意译。
2. **key 命名**：沿用现有约定，Toolkit 用 `tools.toolkits.<toolkit>.{name,description}`，Tool 用 `tools.toolDefs.<tool.name>.{name,description}`。

## Risks / Trade-offs

- [新增工具时仍需手动补 locale] → 这是已有的手动维护模式，本次不改变流程，只补齐缺口。
