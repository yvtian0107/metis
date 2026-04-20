## Why

内建工具（Builtin Tools）管理页面中，`general`、`itsm`、`decision`、`organization` 四个 Toolkit 分组以及其下共 19 个工具的名称/描述显示为原始 i18n key（如 `tools.toolkits.decision.name`），因为 locale 文件只覆盖了最初的 3 个 Toolkit 和 5 个工具，后续由 ITSM、org 等 App 注入的工具从未补录翻译。

## What Changes

- 在 `web/src/apps/ai/locales/zh-CN.json` 的 `tools.toolkits` 和 `tools.toolDefs` 中补全 4 个 Toolkit + 19 个工具的中文翻译
- 在 `web/src/apps/ai/locales/en.json` 同步补全对应英文翻译

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `i18n-frontend`: 补全内建工具的 locale 条目

## Impact

- 仅影响前端 locale JSON 文件：`web/src/apps/ai/locales/zh-CN.json` 和 `en.json`
- 无后端改动、无 API 变更、无依赖变更
