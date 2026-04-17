## Context

ITSM BDD 测试基础设施已完成（godog + bddContext + ClassicEngine wiring + LLM 工作流生成）。现在需要在此基础上创建两组 feature 文件和步骤实现，覆盖经典引擎和智能引擎的 VPN 开通申请主链路。

当前 bddContext 只有 ClassicEngine，没有 SmartEngine。SmartEngine 需要 AgentProvider、UserProvider 等依赖注入，且执行决策时调用真实 LLM。

参考 bklite-cloud 的 pytest-bdd 模式：步骤直接调用引擎层函数（非 HTTP），scenario_ctx dict 共享状态。Metis 用 Go struct（bddContext）实现同等效果。

## Goals / Non-Goals

**Goals:**
- 经典引擎 2 scenarios green：网络支持路由 + 安全合规路由，断言精确到分配人
- 智能引擎 5 scenarios green：正常决策、低置信度 pending_approval、参与者缺失兜底、完整审批链路
- 全部使用真实 LLM，无 mock
- 共享基础设施（参与人、协作规范、工单状态断言）不重复

**Non-Goals:**
- 不测 Service Desk Agent 对话层（多轮对话 BDD 是独立 change）
- 不测工单撤回、草稿版本等边缘场景（已有独立 change proposals）
- 不实现前端 BDD
- 不改变引擎或工具链的生产代码

## Decisions

### 1. bddContext 扩展 SmartEngine + test doubles

bddContext 新增字段：

```go
type bddContext struct {
    // ... 已有字段 ...
    smartEngine *engine.SmartEngine
    llmCfg      llmConfig
}
```

reset() 中额外构建 SmartEngine：

```go
agentProvider := &testAgentProvider{db: db, llmCfg: bc.llmCfg}
userProvider  := &testUserProvider{db: db}
bc.smartEngine = engine.NewSmartEngine(agentProvider, nil, userProvider, &noopSubmitter{})
```

**Why**: SmartEngine 的 4 个依赖中，KnowledgeSearcher 传 nil（测试不需要知识库），TaskSubmitter 复用已有的 noopSubmitter，AgentProvider 和 UserProvider 需要新建 test doubles。

**Alternative considered**: 用 TicketService 包装引擎调用。放弃原因：TicketService 有太多 repo 依赖需要注入，BDD 测试应直接面向引擎层。

### 2. testAgentProvider 设计

```go
type testAgentProvider struct {
    db     *gorm.DB
    llmCfg llmConfig
}
```

从 DB 读 Agent 记录获取 SystemPrompt/Temperature/MaxTokens，从环境变量获取 LLM 连接信息（BaseURL/APIKey/Model）。Protocol 固定为 "openai"。

**Why**: 解耦 Agent 定义（DB 层）和 LLM 连接（env 层），与生产环境的 AgentProvider 行为一致。

### 3. testUserProvider 设计

```go
type testUserProvider struct{ db *gorm.DB }
```

查询 bddContext 内存 DB 中所有 is_active=true 的用户，关联 UserPosition + Position + Department 构建 ParticipantCandidate 列表。

**Why**: SmartEngine.compilePolicy() 需要 UserProvider.ListActiveUsers() 来构建策略快照中的候选参与人列表。

### 4. Feature 文件分离

经典引擎和智能引擎使用独立 feature 文件：
- `features/vpn_classic_flow.feature`
- `features/vpn_smart_flow.feature`

**Why**: 两条赛道的 Background 不同（经典引擎用 `publishVPNService`，智能引擎用 `publishVPNSmartService`），断言策略不同（精确 vs 合法性），分离更清晰。

### 5. 智能引擎断言策略

Smart Engine 断言分两层：

**硬断言（fail on violation）**：
- 工单状态正确（不为 failed）
- 活动存在
- 活动类型在 AllowedSmartStepTypes 内
- 置信度 ∈ [0, 1]
- 若指定参与人，则 participant_id 在 policy.ParticipantCandidates 内
- 时间线包含 AI 决策事件

**软断言（log, don't fail）**：
- 记录 LLM 实际选择的参与人和活动类型，便于观察 prompt 质量

**Why**: LLM 输出非确定性，但决策合法性是确定性的。测合法性保证引擎约束工作，不测具体路由选择避免 flaky。

### 6. pending_approval 场景触发

通过设置 ServiceDefinition.AgentConfig 中的 confidence_threshold 为 0.99：

```go
svc.AgentConfig = `{"confidence_threshold": 0.99}`
```

LLM 返回的 confidence 几乎不可能达到 0.99，强制走 pendApprovalDecisionPlan 路径。

**Alternative considered**: Mock AgentProvider 返回低 confidence 的 DecisionPlan。放弃原因：用户要求全部真 LLM。

### 7. 缺失参与者场景

构造静态 workflow_json fixture（approval 节点无 participant_type），collaboration_spec 故意模糊参与者信息。SmartEngine 基于 collaboration_spec 做决策，应安全兜底（生成 escalate 或 complete）而非 panic。

### 8. publishVPNSmartService

```go
func publishVPNSmartService(bc *bddContext) error {
    workflowJSON, _ := generateVPNWorkflow(bc.llmCfg)  // LLM 生成工作流上下文
    agent := &ai.Agent{...流程决策智能体配置...}          // seed Agent 记录
    bc.db.Create(agent)
    svc := &ServiceDefinition{
        EngineType:        "smart",
        WorkflowJSON:      JSONField(workflowJSON),
        CollaborationSpec: vpnCollaborationSpec,
        AgentID:           &agent.ID,
    }
    bc.db.Create(svc)
}
```

**Why**: Smart Engine 的工具链（LoadService、ValidateParticipants）依赖 workflow_json 作为结构化上下文。collaboration_spec 是 AI 决策的自然语言指导。两者都需要。

### 9. 步骤注册方式

所有步骤在 `initializeScenario` 中通过 bddContext 方法注册：

```go
// bdd_test.go
func initializeScenario(sc *godog.ScenarioContext) {
    bc := newBDDContext()
    sc.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
        bc.reset()
        return ctx, nil
    })
    // Common
    registerCommonSteps(sc, bc)
    // Classic
    registerClassicSteps(sc, bc)
    // Smart
    registerSmartSteps(sc, bc)
}
```

每个 steps_*_test.go 导出一个 `registerXxxSteps(sc, bc)` 函数。

**Why**: godog 的 ScenarioContext 要求所有步骤在 initializeScenario 中一次性注册，不能分散到多个 TestSuite。

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `features/vpn_classic_flow.feature` | Create | 经典引擎 2 scenarios |
| `features/vpn_smart_flow.feature` | Create | 智能引擎 5 scenarios |
| `steps_vpn_classic_test.go` | Create | 经典引擎步骤 + registerClassicSteps |
| `steps_vpn_smart_test.go` | Create | 智能引擎步骤 + registerSmartSteps |
| `steps_common_test.go` | Modify | bddContext 扩展 + test doubles + 共享步骤 + registerCommonSteps |
| `bdd_test.go` | Modify | initializeScenario 注册所有步骤组 |
| `vpn_support_test.go` | Modify | 新增 publishVPNSmartService + defineVPNCollaborationSpec |

## Risks / Trade-offs

- **[LLM 非确定性]** → 智能引擎断言只验证合法性不验证具体值，降低 flaky 风险
- **[LLM 调用耗时]** → 全量 BDD 约 10-16 次 LLM 调用，预计 30-60 秒。可接受（用户已确认 cost 不是问题）
- **[pending_approval 触发不确定]** → confidence_threshold=0.99 几乎必定触发，但理论上 LLM 可能返回 1.0。如果 flaky，可改为 threshold=1.01（永远触发）
- **[generateVPNWorkflow 校验失败]** → 已有重试机制（最多 3 轮），bklite-cloud 实践证明 2-3 轮内收敛
