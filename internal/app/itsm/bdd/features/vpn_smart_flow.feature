Feature: VPN 开通申请 — 智能引擎流程

  智能引擎使用真实 LLM 为 VPN 申请生成合法的流程决策。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份               | 用户名             | 部门 | 岗位           |
      | 申请人             | vpn-requester      | -    | -              |
      | 网络管理员处理人   | network-operator   | it   | network_admin  |
      | 安全管理员处理人   | security-operator  | it   | security_admin |
    And 已定义 VPN 开通申请协作规范
    And 已基于协作规范发布 VPN 开通服务（智能引擎）

  Scenario: 智能引擎为网络支持请求生成合法决策
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 存在至少一个活动
    And 活动类型在允许列表内
    And 决策置信度在合法范围内
    And 若指定了参与人则参与人在候选列表内
    And 时间线应包含 AI 决策相关事件

  Scenario: 智能引擎为安全合规请求生成合法决策
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "external_collaboration"
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 存在至少一个活动
    And 活动类型在允许列表内
    And 若指定了参与人则参与人在候选列表内

  Scenario: 低置信度决策进入人工处置状态后人工人工处置
    Given 智能引擎置信度阈值设为 0.99
    And "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 智能引擎执行决策循环
    Then 工单状态为 "in_progress"
    And 当前活动状态为 "pending"
    And 活动记录中包含 AI 推理说明
    When 管理员确认该人工处置决策
    Then 当前活动状态不为 "pending"

  Scenario: 处理节点缺失参与者时智能引擎安全兜底
    Given "vpn-requester" 已创建 VPN 工单（使用缺失参与者的工作流）
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 时间线应包含 AI 决策相关事件

  Scenario: 智能引擎完整链路 — 决策 → 处理 → 完成
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 智能引擎执行决策循环
    Then 工单状态为 "in_progress"
    And 存在至少一个活动
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"
