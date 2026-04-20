## 1. 补全 zh-CN locale

- [x] 1.1 在 `web/src/apps/ai/locales/zh-CN.json` 的 `tools.toolkits` 中添加 `general`、`itsm`、`decision`、`organization` 四个 Toolkit 的 `name` 和 `description`
- [x] 1.2 在 `web/src/apps/ai/locales/zh-CN.json` 的 `tools.toolDefs` 中添加 19 个缺失工具的 `name` 和 `description`

## 2. 补全 en locale

- [x] 2.1 在 `web/src/apps/ai/locales/en.json` 的 `tools.toolkits` 中添加 `general`、`itsm`、`decision`、`organization` 四个 Toolkit 的 `name` 和 `description`
- [x] 2.2 在 `web/src/apps/ai/locales/en.json` 的 `tools.toolDefs` 中添加 19 个缺失工具的 `name` 和 `description`

## 3. 验证

- [x] 3.1 运行 `cd web && bun run lint` 确认 JSON 格式无误
- [x] 3.2 运行 `cd web && bun run build` 确认构建通过
