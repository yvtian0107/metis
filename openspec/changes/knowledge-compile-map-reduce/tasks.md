## 1. LLM 客户端超时修复

- [x] 1.1 将 `internal/llm/openai_client.go` 的 `http.Client{Timeout}` 从 60s 改为 300s
- [x] 1.2 将 `internal/llm/anthropic_client.go` 的 `http.Client{Timeout}` 从 60s 改为 300s

## 2. Map 阶段实现

- [ ] 2.1 定义 Map 阶段中间结构 `mapNodeOutput` 和 `mapSourceResult`（每个 source 的提取结果）
- [ ] 2.2 编写 Map 阶段 system prompt（`mapSystemPrompt`），指导 LLM 从单个文档提取概念节点和关系
- [ ] 2.3 实现 `mapSource(ctx, llmClient, modelID, source)` 方法：调用 LLM 提取单个 source 的概念，解析 JSON 输出
- [ ] 2.4 实现 `runMapPhase(ctx, llmClient, modelID, sources, progress)` 方法：逐个 source 调用 mapSource，失败容忍（记录 warning 跳过），更新进度

## 3. Reduce 阶段实现

- [ ] 3.1 实现 `buildReducePrompt(mapResults, existingNodes, cascadeAnalysis)` 方法：将 Map 结果的结构化摘要 + existing nodes + cascade analysis 拼接为 Reduce prompt
- [ ] 3.2 复用现有 `compileSystemPrompt` 作为 Reduce 的 system prompt（已包含 cascade update rules）
- [ ] 3.3 实现 `runReducePhase(ctx, llmClient, modelID, mapResults, existingNodes, cascadeAnalysis, progress)` 方法：调用 LLM 合并去重，输出 `compileOutput`

## 4. 编译管线重组

- [ ] 4.1 新增 `CompileStageMapping` 进度阶段常量
- [ ] 4.2 重写 `HandleCompile`：Prepare → Map → Reduce → Write，替换原有的单次 LLM 调用逻辑
- [ ] 4.3 将原有 `buildCompilePrompt` 标记为废弃或删除（被 `buildReducePrompt` 替代）

## 5. 验证

- [ ] 5.1 `go build -tags dev ./cmd/server/` 编译通过
- [ ] 5.2 手动测试：创建知识库，添加 2-3 个 source，触发编译，确认 Map-Reduce 流程正常完成
