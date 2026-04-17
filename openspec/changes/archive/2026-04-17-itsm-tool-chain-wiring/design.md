## Architecture

```
Agent Session → gateway.selectExecutor() → ReactExecutor/PlanAndExecuteExecutor
                                              │
                                              ▼
                                    CompositeToolExecutor
                                    ┌──────────┬──────────┐
                                    ▼          ▼          ▼
                              GeneralTool   ITSM Tool   (future
                               Registry     Registry    app regs)
                                    │          │
                                    ▼          ▼
                              handleCurrentTime  serviceMatchHandler(op, store)
                              handleUserProfile  serviceLoadHandler(op, store)
                              handleOrgContext    draftPrepareHandler(op, store)
                                                 ...
                                                    │          │
                                                    ▼          ▼
                                              ServiceDesk    StateStore
                                               Operator      (Session.State)
                                                    │
                                    ┌───────┬───────┼───────┬────────┐
                                    ▼       ▼       ▼       ▼        ▼
                              ServiceDef  FormDef  Ticket  Participant  UserService
                              Service     Service  Service  Resolver
```

## Key Design Decisions

### 1. CompositeToolExecutor 放在 AI App

CompositeToolExecutor 实现 `ToolExecutor` 接口，属于 AI App 的执行层。它不依赖具体 App 的类型，而是持有一组 `ToolHandlerRegistry` 接口。各 App 通过 IOC 注册自己的 registry 实现。

```go
// internal/app/ai/tool_executor.go

// ToolHandlerRegistry 是各 App 注册工具处理器的接口
type ToolHandlerRegistry interface {
    HasTool(name string) bool
    Execute(ctx context.Context, toolName string, userID uint, args json.RawMessage) (json.RawMessage, error)
}

type CompositeToolExecutor struct {
    registries []ToolHandlerRegistry
    sessionID  uint
    userID     uint
}

func (e *CompositeToolExecutor) ExecuteTool(ctx context.Context, call ToolCall) (ToolResult, error) {
    ctx = context.WithValue(ctx, tools.SessionIDKey, e.sessionID)
    for _, reg := range e.registries {
        if reg.HasTool(call.Name) {
            result, err := reg.Execute(ctx, call.Name, e.userID, call.Args)
            // marshal result to ToolResult
            return ...
        }
    }
    return ToolResult{IsError: true, Output: "unknown tool: " + call.Name}, nil
}
```

### 2. ServiceDeskOperator 放在 ITSM tools 包

具体实现 `ServiceDeskOperator` 接口，注入 ITSM 业务服务。

```go
// internal/app/itsm/tools/operator.go

type Operator struct {
    serviceDefSvc *itsm.ServiceDefService
    formDefSvc    *itsm.FormDefService
    ticketSvc     *itsm.TicketService
    actionSvc     *itsm.ServiceActionService
    resolver      *engine.ParticipantResolver
    userSvc       *service.UserService
    db            *gorm.DB
}
```

方法实现：

| 方法 | 逻辑 |
|------|------|
| `MatchServices(query)` | 查询所有 active ServiceDefinition，按关键词匹配名称+description+keyword，计算 score，返回 top 3 |
| `LoadService(serviceID)` | 查 ServiceDefinition + 关联 FormDefinition 字段 + ServiceAction 列表 + 解析 workflow_json 提取 routing_field_hint |
| `CreateTicket(userID, serviceID, summary, formData, sessionID)` | 调用 TicketService.Create()，source="agent"，agentSessionID=sessionID |
| `ListMyTickets(userID, status)` | 查 Ticket where requester_id=userID，可选 status 过滤，判断 can_withdraw |
| `WithdrawTicket(userID, ticketCode, reason)` | 查 Ticket by code，校验 requester_id，调用 TicketService.Cancel() |
| `ValidateParticipants(serviceID, formData)` | 解析 workflow_json 获取活跃分支的参与者配置，调用 ParticipantResolver 检查是否可解析到有效用户 |

### 3. StateStore 基于 AgentSession.State

```go
// internal/app/itsm/tools/state_store.go

type SessionStateStore struct {
    db *gorm.DB
}

func (s *SessionStateStore) GetState(sessionID uint) (*ServiceDeskState, error) {
    var session struct{ State string }
    if err := s.db.Table("ai_agent_sessions").Where("id = ?", sessionID).Select("state").First(&session).Error; err != nil {
        return nil, err
    }
    if session.State == "" {
        return nil, nil
    }
    var state ServiceDeskState
    json.Unmarshal([]byte(session.State), &state)
    return &state, nil
}

func (s *SessionStateStore) SaveState(sessionID uint, state *ServiceDeskState) error {
    data, _ := json.Marshal(state)
    return s.db.Table("ai_agent_sessions").Where("id = ?", sessionID).Update("state", string(data)).Error
}
```

### 4. ITSM App 暴露 Registry 给 AI App

通过 `internal/app/app.go` 定义跨 App 接口：

```go
// internal/app/app.go 新增接口
type ToolRegistryProvider interface {
    GetToolRegistry() any // 返回实现了 ai.ToolHandlerRegistry 的对象
}
```

ITSM App 实现此接口。AI App 在构建 CompositeToolExecutor 时，遍历所有已注册 App，收集 ToolRegistryProvider。

### 5. Gateway 注入

```go
// gateway.go selectExecutor 修改
func (gw *AgentGateway) selectExecutor(agent *Agent, sessionID uint, userID uint) (Executor, error) {
    switch agent.Type {
    case AgentTypeAssistant:
        client, _ := gw.buildLLMClient(agent)
        toolExec := gw.buildToolExecutor(sessionID, userID)
        switch agent.Strategy {
        case AgentStrategyPlanAndExecute:
            return NewPlanAndExecuteExecutor(client, toolExec), nil
        default:
            return NewReactExecutor(client, toolExec), nil
        }
    ...
    }
}

func (gw *AgentGateway) buildToolExecutor(sessionID, userID uint) ToolExecutor {
    return NewCompositeToolExecutor(gw.registries, sessionID, userID)
}
```

### 6. 服务匹配算法

简单关键词权重匹配：

1. 对 query 分词（空格/标点分割）
2. 对每个 ServiceDefinition 计算得分：
   - 名称完全包含 query → +0.5
   - 名称包含 query 中的关键词 → +0.3 per keyword
   - description 包含关键词 → +0.1 per keyword
   - keywords 字段精确匹配 → +0.4 per match
3. 归一化到 0-1，取 top 3 score > 0

### 7. 路由字段提示提取

从 ServiceDefinition.WorkflowJSON 解析 BPMN 节点：
- 找 exclusive_gateway 节点
- 解析其 conditions 中引用的 form field key
- 构建 option → route label 映射

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/app/itsm/tools/operator.go` | Create | ServiceDeskOperator 具体实现 |
| `internal/app/itsm/tools/state_store.go` | Create | StateStore 基于 AgentSession.State |
| `internal/app/ai/tool_executor.go` | Create | CompositeToolExecutor + ToolHandlerRegistry 接口 |
| `internal/app/ai/gateway.go` | Modify | 注入 CompositeToolExecutor，替换 TODO |
| `internal/app/ai/app.go` | Modify | IOC 注册 CompositeToolExecutor 工厂 |
| `internal/app/app.go` | Modify | 新增 ToolRegistryProvider 接口 |
| `internal/app/itsm/app.go` | Modify | IOC 注册 ITSM Registry，实现 ToolRegistryProvider |
| `internal/app/itsm/tools/provider.go` | Modify | 验证/修正 seed 绑定 |
