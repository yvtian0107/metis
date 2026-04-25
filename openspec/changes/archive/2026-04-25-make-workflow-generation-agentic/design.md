## Context

智能服务定义里的 workflow_json 是参考路径：它既承载可视化共识，也为 SmartEngine 提供 workflow_context、approved/rejected 出边提示和发布健康检查依据。当前 `WorkflowGenerateService.Generate()` 在 LLM 返回可解析 JSON 后运行 `ValidateWorkflow()`；若最后一次尝试仍有 blocking errors，handler 返回 400，前端只显示失败 toast。

这个设计把“生成草图”和“运行前完整性保障”混在一起。对用户来说，点击生成流程后等待十几秒，最后看不到图，只看到“阻断性错误”，无法与 Agent 继续协作修正。对系统来说，运行风险仍需要被识别，但它更适合出现在服务健康检查和发布门禁中，而不是拦截一次参考路径生成。

另一个实现问题是 `detectDeadEnds()` 只选择第一个 end 节点作为反向 BFS 起点；当 workflow_json 有正常结束、驳回结束等多个 end 节点时，其他合法终点分支会被误判为 dead-end。

## Goals / Non-Goals

**Goals:**

- 生成阶段只要获得可解析 workflow_json，就返回图、issues、保存状态和健康检查结果。
- blocking validation errors 不再让 `/api/v1/itsm/workflows/generate` 返回 400；它们 SHALL 进入响应体和发布健康检查。
- 保留 LLM retry：校验错误仍反馈给下一次 LLM 调用，尽量让 Agent 自修复。
- 修复多 end 节点 dead-end 误判。
- 前端生成后刷新服务定义并展示可视化结果，用 toast/健康检查说明风险，而不是把用户挡在 400 错误外。

**Non-Goals:**

- 不降低运行期、发布期、工单创建期的 workflow_json 校验要求。
- 不移除 `ValidateWorkflow()` 的 blocking/warning 分级。
- 不把 SmartEngine 改造成确定性流程执行器。
- 不引入新的数据库字段或兼容旧响应字段。
- 不调整 path builder prompt 的业务约束，除非测试发现必须补充。

## Decisions

### D1：生成 API 将“可解析草图”视为成功响应

选择：`WorkflowGenerateHandler.Generate()` 在 service 返回 `WorkflowJSON` 时始终返回 200，即使 `Errors` 中包含 blocking issues。响应体中的 `saved` 表示是否写入 ServiceDefinition；`errors` 表示草图质量问题；`healthCheck` 表示服务发布健康状态。

替代方案：继续返回 400，但把 workflow_json 放在错误响应 data 中。这个方案仍会被前端 API 层当成失败路径处理，用户体验仍像传统表单校验，不符合“生成草图并协作修正”的目标。

### D2：允许保存带 issues 的参考路径草图

选择：当 `serviceId > 0` 且 workflow_json 可解析时，生成服务仍写入 `workflow_json` 和 `collaboration_spec`，随后调用 `RefreshPublishHealthCheck()`。若有 blocking errors，健康检查标记 fail，用户能看到图和阻塞原因。

替代方案：只返回草图不保存。这样前端必须维护一份未持久化图状态，刷新页面会丢失 Agent 产物，也无法让健康检查、后续上下文和用户协作围绕同一份草图继续。

### D3：错误边界只保留在“没有草图”的情况

以下情况仍返回错误状态：
- 协作规范为空
- 路径生成模型未配置
- LLM 上游调用失败或超时
- 多次尝试后仍无法提取可解析 JSON

这些情况没有可展示的 workflow_json，无法进入协作式修正流程。

### D4：多终点 dead-end 检测从所有 end 反向遍历

选择：`detectDeadEnds()` 收集全部 `type=end` 节点，全部作为 reverse BFS 起点。任何能到达任一 end 的节点都不是 dead-end。end 节点本身永远不会因为“到不了另一个 end”而报错。

替代方案：要求只有一个 end。现有 classic engine 规格和真实业务都允许多个 end，例如正常结束、驳回结束、异常结束；收窄为单终点会破坏流程表达能力。

### D5：前端以“生成完成但需确认”呈现 issues

选择：前端 mutation 的 success path 处理 `errors`：
- 无 errors：显示成功。
- 有 warning/blocking：显示生成完成但存在问题，刷新服务数据，让工作流图和发布健康检查承担详细解释。

API 类型补齐 `level` 和 `saved` 字段。前端不在生成按钮旁堆叠大段校验文案，避免重复健康检查区域。

## Risks / Trade-offs

- [Risk] 保存带 blocking issues 的 workflow_json 可能被误以为可运行 → Mitigation：`RefreshPublishHealthCheck()` 必须返回 fail，运行/发布入口继续使用 `ValidateWorkflow()` 阻断。
- [Risk] 现有测试期望 400 → Mitigation：调整 handler/service tests，新增“blocking issues 返回 200 且 saved=true/health fail”的覆盖。
- [Risk] 当前 active change 中已有相反规格 → Mitigation：本 change 明确修改 `itsm-workflow-generate` 语义，后续实现以本 change 为准。
- [Risk] 前端只 toast 不够显眼 → Mitigation：生成后刷新详情页数据，健康检查区域显示 fail/warn，并让工作流 Tab 展示实际图。
