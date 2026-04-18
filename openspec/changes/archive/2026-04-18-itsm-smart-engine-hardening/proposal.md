## Why

智能引擎代码深度审查发现 6 类系统性问题：LLM 结构化输出脆弱（同仓库两套 extractJSON，ITSM 未复用 jsonrepair 增强版）、驱动层缺 ResponseFormat 导致不同模型表现不一致、知识装载基础设施就绪但 ServiceDefinition 缺字段导致空转、direct_first 决策模式仅存于 prompt 文字无实质 hints 注入、BDD 对话层场景严重缺失、以及 in_progress 票据无重启恢复机制。这些问题在切换非 OpenAI 模型（Claude/DeepSeek）或服务目录扩大时会集中暴露。

## What Changes

- **统一 extractJSON 并复用 jsonrepair**：将 `knowledge_compile_longdoc.go` 的增强版 extractJSON 提取到 `internal/llm/json.go` 作为公共函数，ITSM 智能引擎和知识编译统一使用
- **llm.ChatRequest 增加 ResponseFormat**：在统一接口层支持结构化输出请求，OpenAI 驱动用 `json_schema`，Anthropic 驱动用 prefill 技巧 + prompt 强化，其他协议用 prompt 注入 + extractJSON fallback
- **ServiceDefinition 增加 knowledge_base_ids 字段**：打通服务定义到知识库的关联，使 `decision.knowledge_search` 工具真正生效
- **direct_first 模式实现 workflow_hints 注入**：从 WorkflowJSON 提取结构化步骤提示，作为 Agent seed message 的一部分注入，而非仅依赖 prompt 文字
- **补齐 BDD 对话层场景**：覆盖对话全链路 E2E、多种对话模式（直接完成/口语化/多轮补充/保持等待）、会话隔离、知识驱动路由
- **智能引擎决策循环恢复机制**：Server 启动时扫描 in_progress + 智能引擎的票据，对无活跃活动的票据重新触发决策循环

## Capabilities

### New Capabilities
- `itsm-smart-recovery`: 智能引擎决策循环的重启恢复机制，确保 in_progress 票据不会因 server 重启而卡死

### Modified Capabilities
- `ai-llm-client`: ChatRequest 增加 ResponseFormat 字段，各驱动层实现结构化输出策略
- `itsm-smart-engine`: extractJSON 统一为 jsonrepair 增强版；direct_first 模式实现 workflow_hints 注入
- `itsm-service-definition`: 模型增加 knowledge_base_ids 字段，关联知识库
- `itsm-decision-tools`: knowledge_search 工具接入 ServiceDefinition 的 knowledge_base_ids
- `itsm-bdd-infrastructure`: 补齐对话全链路 E2E、多对话模式、会话隔离、知识驱动路由的 BDD 场景

## Impact

- **internal/llm/**: 新增 `json.go`（公共 extractJSON）、`ChatRequest` 增加字段、`openai_client.go` 和 `anthropic_client.go` 适配 ResponseFormat
- **internal/app/itsm/engine/**: `smart.go` 删除本地 extractJSON 改用公共版、`smart_react.go` 实现 hints 注入 + ResponseFormat 使用、新增恢复任务
- **internal/app/itsm/model_service.go**: ServiceDefinition 增加字段 + AutoMigrate
- **internal/app/itsm/tools/**: knowledge_search handler 读取 knowledge_base_ids
- **internal/app/ai/knowledge_compile_*.go**: 删除本地 extractJSON 改用公共版
- **internal/app/itsm/features/**: 新增 5+ 个 .feature 文件
- **go.mod**: 无新增依赖（jsonrepair 已在）
