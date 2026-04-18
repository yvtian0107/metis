## 1. 统一 JSON 提取函数

- [x] 1.1 创建 `internal/llm/json.go`，将 `knowledge_compile_longdoc.go` 的 `extractJSON` 提取为导出函数 `llm.ExtractJSON()`，包含 markdown fence 剥离、TrimSpace、json.Valid 快速校验、jsonrepair.Repair 修复
- [x] 1.2 修改 `internal/app/ai/knowledge_compile_longdoc.go`，删除本地 `extractJSON` 和 `truncate` 函数，改为调用 `llm.ExtractJSON()`
- [x] 1.3 修改 `internal/app/ai/knowledge_compile_service.go`，将所有 `extractJSON()` 调用改为 `llm.ExtractJSON()`
- [x] 1.4 修改 `internal/app/itsm/engine/smart.go`，删除本地 `extractJSON` 函数，`parseDecisionPlan` 改为调用 `llm.ExtractJSON()`
- [x] 1.5 为 `llm.ExtractJSON` 编写单元测试（markdown fence、trailing comma、截断 JSON、合法 JSON 直通）

## 2. ChatRequest ResponseFormat 支持

- [x] 2.1 在 `internal/llm/client.go` 新增 `ResponseFormat` 结构体和 `ChatRequest.ResponseFormat` 字段
- [x] 2.2 修改 `internal/llm/openai_client.go`，Chat 方法中翻译 ResponseFormat 为 OpenAI API 的 `response_format` 参数（json_object 和 json_schema 两种模式）
- [x] 2.3 修改 `internal/llm/anthropic_client.go`，Chat 方法中处理 ResponseFormat：json_object 时追加 system prompt 约束 + assistant prefill `{`
- [x] 2.4 修改 `internal/app/itsm/engine/smart_react.go`，ChatRequest 构造时设置 `ResponseFormat: &llm.ResponseFormat{Type: "json_object"}`

## 3. ServiceDefinition 知识库关联

- [x] 3.1 在 ServiceDefinition 模型中新增 `KnowledgeBaseIDs` 字段（JSON TEXT nullable），AutoMigrate 更新表结构
- [x] 3.2 更新 ServiceDefinition 的 Create/Update handler 和 Response，支持 `knowledgeBaseIds` 字段读写
- [x] 3.3 修改 `internal/app/itsm/engine/smart_tools.go` 中 `decision.knowledge_search` 的 handler，从 ServiceDefinition 读取 `knowledge_base_ids` 传递给 KnowledgeSearcher
- [x] 3.4 修改 `decisionToolContext` 结构体，新增 `knowledgeBaseIDs []uint` 字段，在 `agenticDecision` 构建 toolCtx 时从服务定义填充

## 4. direct_first 模式 workflow_hints 注入

- [x] 4.1 在 `internal/app/itsm/engine/smart_react.go` 新增 `extractWorkflowHints(workflowJSON string) string` 函数，从 WorkflowJSON 提取结构化步骤摘要（遍历 nodes/edges，提取 label+participantType+conditions）
- [x] 4.2 修改 `buildAgenticSystemPrompt`，在 `direct_first` 模式下调用 `extractWorkflowHints` 并将结果作为 `## 工作流参考路径` 注入 system prompt；提取失败时退化为 ai_only 并记录 warning
- [x] 4.3 为 `extractWorkflowHints` 编写单元测试（含网关分支、串行流程、提取失败退化）

## 5. 智能引擎恢复机制

- [x] 5.1 在 `internal/app/itsm/engine/tasks.go` 新增 `HandleSmartRecovery` 函数，扫描 in_progress+smart 票据，对无活跃活动且未熔断的票据提交 `itsm-smart-progress` 任务
- [x] 5.2 在 ITSM App 的 `Tasks()` 方法中注册 `itsm-smart-recovery` 任务，类型为启动时执行一次
- [x] 5.3 在 `smart_engine_recovery.feature` 中编写 BDD 场景（恢复无活跃活动票据、跳过有活跃活动票据）
- [x] 5.4 实现恢复场景的 step definitions

## 6. BDD 对话层场景补齐

- [x] 6.1 创建 `vpn_e2e_dialog_flow.feature`，编写网络支持和安全合规的完整对话到创单场景（@llm 标签）
- [x] 6.2 实现 E2E 对话场景的 step definitions（含服务台 Agent 对话 + 引擎触发验证）
- [x] 6.3 创建 `vpn_dialog_coverage.feature`，使用 Scenario Outline 编写 6 种对话模式（complete_direct / colloquial_complete / multi_turn_fill_details / full_info_hold / ambiguous_incomplete_hold / multi_turn_hold），@llm 标签
- [x] 6.4 实现对话模式场景的 step definitions（对话模板 + 工具调用序列断言）
- [x] 6.5 创建 `service_desk_session_isolation.feature`，编写连续请求状态隔离和 new_request 重置场景
- [x] 6.6 实现会话隔离场景的 step definitions
- [x] 6.7 创建 `service_knowledge_routing.feature`，编写知识命中路由、知识未命中默认路由、知识库不可用场景（@llm 标签）
- [x] 6.8 实现知识驱动路由场景的 step definitions（需 mock KnowledgeSearcher 或使用真实知识库 seed 数据）

## 7. 验证与集成

- [x] 7.1 运行 `go build -tags dev ./cmd/server/` 验证编译通过
- [x] 7.2 运行 `go test ./internal/llm/` 验证 ExtractJSON 单元测试通过
- [x] 7.3 运行 `make test-bdd` 验证所有现有 + 新增 BDD 场景通过（排除 @llm）
- [x] 7.4 配置 LLM 环境变量后运行 @llm 标签的 BDD 场景验证
