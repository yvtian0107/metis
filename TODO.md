**结论**

按“Agentic ITSM 产品经理”视角看：当前 ITSM 不是传统完整 ITSM 套件，而是一个很有方向感的 **Agentic Service Desk + 工单编排引擎**。它在“自然语言提单、服务匹配、流程决策、审批流转、SLA、自动动作、知识辅助决策”上已经有骨架，而且信仰很明确。

我给两个分：

- **Agentic ITSM MVP：7.2 / 10**
- **企业级完整 ITSM：5.6 / 10**

强项很强，短板也很清楚：现在更像“智能服务请求 / 审批编排平台”，还不是覆盖 Incident / Problem / Change / CMDB / Knowledge / Reporting 的完整 ITSM。

**已具备能力**

- 服务目录、服务定义、接入表单、经典流程、智能协作规范、知识库绑定、服务动作都在服务定义模型里：[model_catalog.go](/Users/umr/Documents/codespace/metis/internal/app/itsm/model_catalog.go:91)
- 工单主生命周期完整，有状态、来源、请求人、处理人、SLA、表单数据、流程快照：[model_ticket.go](/Users/umr/Documents/codespace/metis/internal/app/itsm/model_ticket.go:42)
- 有活动、审批/处理分配、认领、转派、委派、时间线、变量、执行 token：[model_ticket.go](/Users/umr/Documents/codespace/metis/internal/app/itsm/model_ticket.go:141)
- 后端 API 覆盖目录、服务、动作、知识文档、SLA、升级规则、工单、审批、变量、token、人工覆盖、SLA 暂停恢复、转派委派认领：[app.go](/Users/umr/Documents/codespace/metis/internal/app/itsm/app.go:251)
- Agentic 服务台提示词已经有明确状态机：服务匹配、加载、草稿、确认、参与者校验、创建工单：[provider.go](/Users/umr/Documents/codespace/metis/internal/app/itsm/tools/provider.go:238)
- 智能决策 Agent 强调证据、知识、动作、参与者、SLA，方向是对的：[provider.go](/Users/umr/Documents/codespace/metis/internal/app/itsm/tools/provider.go:281)

**主要缺口**

1. **直连提单体验不完整**
   前端创建工单只提交 title / description / serviceId / priorityId，没有渲染服务自己的 intake form，也没有提交 formData：[create/index.tsx](/Users/umr/Documents/codespace/metis/web/src/apps/itsm/pages/tickets/create/index.tsx:85)。这会导致“通过 Agent 提单很强，用户自己点表单提单反而弱”。

2. **ITIL 主对象没有成体系**
   当前菜单是服务、工单、配置，缺 Incident / Problem / Change / CMDB 等一级产品能力：[module.ts](/Users/umr/Documents/codespace/metis/web/src/apps/itsm/module.ts:10)。虽然代码里有 TicketLink 和 PostMortem 模型：[model_ticket.go](/Users/umr/Documents/codespace/metis/internal/app/itsm/model_ticket.go:248)，但没有看到对应路由和前端入口，说明事故关联和复盘还没产品化。

3. **SLA 有骨架，但不够生产级**
   已有 SLA 模板和升级规则：[model_sla.go](/Users/umr/Documents/codespace/metis/internal/app/itsm/model_sla.go:54)，也有定时检查任务：[app.go](/Users/umr/Documents/codespace/metis/internal/app/itsm/app.go:396)。但缺少工作时间日历、暂停原因、首次响应确认、升级去重记录、OLA/UC、SLA 报表等关键能力。

4. **工作流很强，但高级节点存在“声明大于落地”风险**
   引擎注册了很多节点，但 timer / signal 标记为未实现：[engine.go](/Users/umr/Documents/codespace/metis/internal/app/itsm/engine/engine.go:105)。这会影响复杂 ITSM 流程里的等待、外部事件、定时器编排可信度。

5. **运营视图不足**
   目前有全部工单、我的工单、待办、历史、审批，但缺团队队列、值班视图、SLA 风险队列、重大事件看板、MTTA/MTTR、积压分析、Agent 命中率/误判率等管理视角。

**评分拆解**

| 能力域                      | 分数 | 判断                                                               |
| --------------------------- | ---: | ------------------------------------------------------------------ |
| 服务目录 / 服务请求         |  8.0 | 服务定义、表单、流程、动作、知识都具备，但用户自助提单表单断层明显 |
| 工单流转 / 审批             |  7.5 | 生命周期、审批、转派委派认领、时间线较完整，前端暴露不足           |
| Agentic 能力                |  8.5 | 服务台状态机和决策 Agent 设计很有信仰，是当前最大亮点              |
| SLA / 升级                  |  6.5 | 有模板、截止时间、检查、升级动作，但缺生产运营细节                 |
| Incident / Problem / Change |  4.0 | 有示例和局部模型，但没有形成一级能力                               |
| CMDB / 资产影响             |  2.5 | 暂未看到资产、CI、依赖、影响面能力                                 |
| 知识管理                    |  5.5 | 有服务知识文档，缺知识文章生命周期和工单闭环                       |
| 报表 / 运营治理             |  3.5 | 缺管理指标和质量闭环                                               |

**优先补齐建议**

1. **先补“提单主路径”**：让 `/itsm/tickets/create` 根据服务动态渲染 intake form，提交 formData，并复用后端校验。
2. **再补“ITSM 四大对象”**：Service Request / Incident / Change / Problem 明确分型，不要都塞进普通 Ticket。
3. **把事故和复盘产品化**：基于现有 TicketLink / PostMortem 做重大事件、关联工单、RCA、行动项闭环。
4. **补运营驾驶舱**：SLA 风险、待办积压、MTTA/MTTR、超时趋势、Agent 决策质量。
5. **引入 CMDB/资产上下文**：工单能关联 CI、服务、系统、负责人和影响范围，这是 ITSM 从“流程工具”走向“运营系统”的关键。

一句话评价：**方向很好，Agentic 部分有灵魂；但 ITSM 基础盘还缺几根承重柱。现在适合叫 Agentic Service Desk，不宜自称完整 ITSM。**