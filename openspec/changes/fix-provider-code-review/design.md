## Context

供应商管理模块（`internal/app/ai/` + `web/src/apps/ai/`）是 Code Review 发现问题最集中的区域。当前代码可以工作，但存在数据一致性风险、静默错误、UX 反直觉等问题。本次修复是一批 surgical fix，不引入新功能，不改 API 契约。

核心代码量不大：后端 ~1000 行（6 个文件），前端 ~1600 行（7 个文件）。修改均为局部替换，风险可控。

## Goals / Non-Goals

**Goals:**
- 修复 B1-B7、F1-F3、F6-F7 共 12 个 Code Review issue
- 补充对应单元测试（尤其 B3 跨 provider 隔离）
- 保持 API 响应结构不变，不影响现有前端消费方

**Non-Goals:**
- B8（MaskAPIKey 解密优化）暂不实现，需要加数据库字段，收益不大
- F4（useEffect 依赖）React Compiler 下无实际问题，跳过
- F5（列表页编辑交互）当前跳转到详情页编辑是合理流程，不改
- 不重构 ai app 的整体架构

## Decisions

### D1: 提取 `parseUintParam` helper 统一处理路径参数解析（B1）

在 `internal/handler/` 包新增 `ParseUintParam(c *gin.Context, name string) (uint, bool)` 工具函数，解析失败直接写 400 响应并返回 `false`。所有 handler 里的 `strconv.Atoi(c.Param(...))` 替换为此函数。

**替代方案**：在每个 handler 里各自处理 — 重复代码多；用 gin 中间件预解析 — 过度设计。

### D2: Provider Update 条件性重置 status（B2）

`ProviderService.Update` 对比旧值，仅当 `BaseURL` 或 `APIKey` 实际发生变化时才重置 `status = inactive`。名称、类型等变更不影响连接状态，无需重置。

### D3: SetDefault 作用域改为 (provider_id, type)，事务保护（B3）

新增 `ClearDefaultByProviderAndType(tx *gorm.DB, providerID uint, modelType string)` 替换原有 `ClearDefaultByType`。
`SetDefault` 用 `db.Transaction` 包裹 clear + set 两步。

**`FindDefaultByType` 保持不变** — 知识库编译需要"系统级默认 LLM"（全局任意一个 `is_default=true` 的），不受影响。两个维度共存：
- 供应商级默认：UI 控制，`SetDefault` 操作
- 系统级查询：`FindDefaultByType` 返回全局第一个匹配

### D4: Anthropic 测试连接改用已同步模型（B5）

`testAnthropicConnection` 先查该 provider 下已有的 `is_default=true` 或第一个 active 模型的 `model_id`，如果没有模型则 fallback 到 `claude-haiku-3-5-20241022`。避免硬编码模型被下线时连接测试永远失败。

### D5: guessModelType 未匹配返回 `"other"`（B6）

新增 `ModelTypeOther = "other"` 常量，`ValidModelTypes` 中加入。前端 `TYPE_ORDER` 末尾加 `"other"` 面板。

### D6: 前端 IIFE 提取为 `ModelTypePanel`（F2）

将 `[id].tsx` 中 140 行的 IIFE 块提取为 `<ModelTypePanel>` 独立组件，props 接收 type、items、分页状态、权限标志、mutation 回调。

### D7: 前端详情页添加返回导航（F7）

在详情页顶部 `ProviderInfoSection` 上方加一行面包屑式返回链接：`← 供应商列表 / {provider.name}`。

## Risks / Trade-offs

- **D3 多默认共存**：改为 provider 级默认后，`FindDefaultByType` 可能返回多条 `is_default=true` 的同 type 模型（来自不同 provider）。当前 `First()` 只取一条，行为不变。但语义上"系统级默认"变成了"碰巧第一个"。→ 暂可接受，未来如需显式系统级默认可加 `is_system_default` 字段。
- **D5 新增 `other` 类型**：已有数据中 type 为空字符串的模型不会自动迁移为 `"other"`。→ 只影响后续新同步的模型，旧空字符串数据前端已有"未分类"面板兜底。
- **D1 helper 函数**：新增 `handler` 包公共函数，其他 app 也可以用，但不强制迁移，只改 ai app 的 handler。
