## Why

Code Review 发现供应商管理模块（后端 `internal/app/ai/` + 前端 `web/src/apps/ai/`）存在一批质量问题，涵盖数据一致性风险（SetDefault 缺事务 + 作用域过大）、静默错误吞没、UX 不合理（编辑即重置状态）、以及前端可维护性短板。这些问题在当前用量下未暴露，但随着接入供应商和模型数量增长，会造成数据错误和用户困惑。需要集中修复。

## What Changes

### 后端 — 严重
- **B1** Handler 层 `strconv.Atoi(c.Param("id"))` 错误被静默忽略，非数字 ID 应返回 400 而不是查 id=0
- **B2** `ProviderService.Update` 每次都强制重置 status 为 `inactive`，改为仅在 BaseURL 或 APIKey 变化时重置
- **B3** `ModelService.SetDefault` 作用域改为 **(provider_id, type)** 维度；两步操作（clear + set）用事务包裹

### 后端 — 中等
- **B4** `ProviderHandler.List` 中 `ModelCountsForProviders` / `TypeCountsForProviders` 错误被静默吞掉，补 log warning
- **B5** `testAnthropicConnection` 硬编码 `claude-haiku-3-5-20241022`，改为从已同步模型中取或使用更轻量探测
- **B6** `guessModelType` 未匹配时返回空字符串，改为返回 `"other"`
- **B7** `ProviderRepo.List` 的 LIKE 关键词未转义 `%` 和 `_`
- **B8** `MaskAPIKey` 每次列表都做 AES 解密，可优化但优先级低（暂标记，不在本轮实现）

### 前端 — 中等
- **F1** 详情页拉模型 `pageSize=100` 可能截断，改为足够大值或后端不分页模式
- **F2** `[id].tsx` 中 140 行 IIFE 提取为独立 `ModelTypePanel` 组件
- **F3** 搜索无结果与无数据共用同一文案，三目运算无意义，区分两种空状态
- **F6** `formatRelativeTime` 硬编码英文时间后缀，改为走 i18n
- **F7** 详情页正常状态缺少返回列表的导航

## Capabilities

### New Capabilities
（无新增能力）

### Modified Capabilities
- `ai-model`: Default model 作用域从全局 type 改为 (provider_id, type) 维度；`guessModelType` 未匹配类型改为 `"other"`
- `ai-provider`: Update 时 status 重置条件从"总是"改为"仅 BaseURL/APIKey 变化时"；连接测试模型选择策略调整

## Impact

- **后端文件**：`provider_handler.go`、`provider_service.go`、`provider_repository.go`、`model_handler.go`、`model_service.go`、`model_repository.go`、`model.go`
- **前端文件**：`pages/providers/[id].tsx`、`components/provider-card.tsx`、`locales/zh-CN.json`、`locales/en.json`
- **测试**：`provider_service_test.go`、`model_service_test.go` 需补充跨 provider SetDefault 隔离用例
- **下游**：`knowledge_compile_service.go` 的 `FindDefaultByType` 调用不受影响（全局查询保持不变）
- **API 契约**：无 breaking change，响应结构不变
