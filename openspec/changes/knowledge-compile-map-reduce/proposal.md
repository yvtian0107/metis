## Why

知识库编译在 source 数量多或内容量大时，单次 LLM 调用的 input tokens 过高（N x 8000 chars），导致首 token 延迟超过 HTTP 客户端的 60s 硬编码超时，编译任务必然失败。同时 scheduler 给的 300s context deadline 因为 HTTP 层 60s 限制形同虚设。

## What Changes

- **Map-Reduce 编译管线**：将单次大 LLM 调用拆分为 Map（逐 source 提取概念）+ Reduce（LLM 合并去重 + 关联已有图谱）两阶段，单次调用 input 从 N x 8k chars 降低到 ~8k chars（Map）和结构化摘要（Reduce）
- **LLM 客户端超时修复**：HTTP 客户端超时从硬编码 60s 改为尊重 context deadline，让 scheduler 的 300s 超时生效
- **细粒度进度追踪**：Map 阶段按 source 报告进度（"分析来源 3/10: xxx"），替代当前的大黑箱 "AI 正在分析..."

## Capabilities

### New Capabilities

（无新增能力）

### Modified Capabilities

- `ai-knowledge`: 编译流程从 one-shot 改为 map-reduce 两阶段，失败粒度从全量重试变为单 source 级别
- `ai-llm-client`: HTTP 客户端超时行为变更，从硬编码 60s 改为尊重调用方传入的 context deadline

## Impact

- `internal/app/ai/knowledge_compile_service.go` — 核心改动：拆分 HandleCompile 为 map/reduce 两阶段，新增 map prompt 和 reduce prompt
- `internal/llm/openai_client.go` — HTTP timeout 改为 context-aware
- `internal/llm/anthropic_client.go` — 同上
- 无 API 变更，无数据库 schema 变更，无前端变更
- `compileOutput` 结构和 `writeCompileOutput` 等下游逻辑不变（Reduce 输出格式兼容现有）
