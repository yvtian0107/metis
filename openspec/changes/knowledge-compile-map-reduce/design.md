## Context

当前知识库编译（`knowledge_compile_service.go`）将所有 source 内容拼接为一个大 prompt 发送给 LLM。当 source 数量多或内容量大时，单次 LLM 调用的 input tokens 过高（N x 8000 chars），首 token 延迟超过 HTTP 客户端硬编码的 60s 超时限制，导致编译必然失败。

同时，LLM HTTP 客户端（`openai_client.go` / `anthropic_client.go`）在构造时硬编码 `http.Client{Timeout: 60s}`，无视调用方传入的 context deadline（scheduler 给的 300s），使得 scheduler 层的超时控制形同虚设。

## Goals / Non-Goals

**Goals:**
- 消除大知识库编译的超时失败问题
- 降低单次 LLM 调用的 input token 量，使首 token 延迟可控
- 失败粒度从全量重试变为单 source 级别
- 编译进度追踪更细粒度（按 source 报告）

**Non-Goals:**
- Map 阶段不做并发（避免 API rate limit 问题）
- 不改变编译的最终输出格式（`compileOutput` / `writeCompileOutput` 保持不变）
- 不涉及前端 UI 变更
- 不做分层 reduce（当前 reduce input 为结构化摘要，token 量可控）

## Decisions

### Decision 1: Map-Reduce 两阶段编译

**选择**：将 `HandleCompile` 拆分为 Map → Reduce → Write 三步。

**Map 阶段**：逐个 source 调用 LLM 提取概念节点和关系。每次 input ≤ 8k chars + system prompt，首 token 延迟通常 < 10s。输出为中间结构 `[]mapNodeOutput`。

**Reduce 阶段**：将所有 Map 结果的结构化摘要（title + summary + relations）+ existing nodes + cascade analysis 发送给 LLM，合并去重、建立跨来源关系。输出格式与现有 `compileOutput` 完全兼容。

**替代方案**：
- 纯代码 Reduce（title 匹配 + merge）—— 省一次 LLM 调用，但无法处理同义概念（如 "React Hooks" vs "Hooks in React"），质量不够
- 并发 Map —— 加速明显，但引入 rate limit 问题，暂不需要

### Decision 2: Map 阶段单 source 失败容忍

**选择**：单个 source 的 Map 调用失败时，记录 warning 并跳过该 source，继续处理其余 source。只有全部 source 都失败时才返回 error。

**原因**：编译 10 个 source 时，1 个 source 超时不应导致整体失败。Reduce 阶段用已成功的 Map 结果继续编译，比全量重试更合理。

### Decision 3: LLM 客户端 HTTP 超时改为 5 分钟

**选择**：将 `openai_client.go` 和 `anthropic_client.go` 的 HTTP client timeout 从 60s 改为 300s（5 分钟）。

**原因**：
- 60s 对知识编译场景太短，但 context deadline 才是正确的超时控制机制
- Go 的 `http.Client.Timeout` 是硬限制，会无视 context deadline
- 设为 300s 作为安全网，与 compile task 的 scheduler timeout 对齐
- 对 agent chat 场景，agent executor 有自己的 context deadline，300s 的 HTTP timeout 不会影响正常超时行为

**替代方案**：
- 设为 0（无超时）—— 完全依赖 context，但丢失了对未设 deadline 调用的安全网
- 参数化传入 —— 需要改 `NewClient` 签名，影响面过大

### Decision 4: Map/Reduce Prompt 设计

**Map Prompt**（每个 source 独立）：
```
System: 知识提取器，从单个文档提取概念和关系
User:   ## Source: {title}
        {content ≤ 8000 chars}
```
输出格式：`{nodes: [{title, summary, content, related: [{concept, relation}], source: "Source Title"}]}`

**Reduce Prompt**（合并所有 Map 结果）：
```
System: 现有的 compileSystemPrompt（保持不变）
User:   ## Extracted concepts (from Map phase)
        ### From "Source 1": [{title, summary}...]
        ### From "Source 2": [{title, summary}...]

        ## Existing knowledge nodes (同现有逻辑)
        ## Impact Analysis (同现有逻辑)
```
输出格式：与现有 `compileOutput` 完全一致（`{nodes, updated_nodes}`）。

### Decision 5: 进度追踪

Map 阶段新增细粒度进度：
- `CompileStageMapping` 新阶段
- `progress.Sources.Done` 按 Map 完成数递增
- `progress.CurrentItem` = "分析来源 3/10: {source title}"

Reduce 阶段复用现有 `CompileStageCallingLLM`。

## Risks / Trade-offs

- **Token 费用略增** → Map 阶段每个 source 独立调用，system prompt 重复 N 次。但 Reduce 阶段 input 是结构化摘要而非原文，总体 token 增幅不大（预计 10-20%）。
- **编译总时间可能增加** → Map 是串行的，N 个 source 意味着 N 次 LLM 调用。但每次调用都很快（< 10s），10 个 source 约 1-2 分钟，仍在 300s scheduler timeout 内。
- **Map 结果同义概念** → 不同 source 可能对同一概念使用不同 title。Reduce 阶段 LLM 负责识别和合并，prompt 中需要明确指示。如果 LLM 合并效果不好，后续可在 Map prompt 中提供已有概念列表作为 hint。
