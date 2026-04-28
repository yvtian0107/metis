Feature: SLA 保障岗 Agentic 处理
  系统需要确认 SLA 保障岗在 Agentic 模式下能读取风险工单、SLA 上下文和升级规则，并触发正确保障动作。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份       | 用户名        | 部门 | 岗位      |
      | 申请人     | sla-requester | it   | staff     |
      | 当前处理人 | ops-current   | it   | ops_admin |
      | 升级处理人 | ops-lead      | it   | ops_lead  |
    And 已发布带 SLA 的智能服务和 SLA 保障岗

  Scenario: SLA 保障岗触发通知升级
    Given 存在响应 SLA 已超时且命中 "notify" 升级规则的工单
    When 执行 SLA 保障扫描
    Then SLA 保障岗已调用工具 "sla.risk_queue"
    And SLA 保障岗已调用工具 "sla.ticket_context"
    And SLA 保障岗已调用工具 "sla.escalation_rules"
    And SLA 保障岗已调用工具 "sla.trigger_escalation"
    And 时间线包含 "sla_escalation" 类型事件

  Scenario: SLA 保障岗触发转派升级
    Given 存在响应 SLA 已超时且命中 "reassign" 升级规则的工单
    When 执行 SLA 保障扫描
    Then SLA 保障岗已调用工具 "sla.risk_queue"
    And SLA 保障岗已调用工具 "sla.ticket_context"
    And SLA 保障岗已调用工具 "sla.escalation_rules"
    And SLA 保障岗已调用工具 "sla.trigger_escalation"
    And 工单已转派给 "ops-lead"
    And 时间线包含 "sla_escalation" 类型事件

  Scenario: SLA 保障岗触发优先级升级
    Given 存在响应 SLA 已超时且命中 "escalate_priority" 升级规则的工单
    When 执行 SLA 保障扫描
    Then SLA 保障岗已调用工具 "sla.risk_queue"
    And SLA 保障岗已调用工具 "sla.ticket_context"
    And SLA 保障岗已调用工具 "sla.escalation_rules"
    And SLA 保障岗已调用工具 "sla.trigger_escalation"
    And 工单优先级为 "urgent"
    And 时间线包含 "sla_escalation" 类型事件
