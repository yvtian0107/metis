@llm
Feature: VPN 开通申请 — 服务台 Agent 对话校验

  验证服务台 Agent 在对话层的智能识别能力：
  - 跨路由冲突识别
  - 同路由多选合并
  - 必填缺失追问

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份               | 用户名             | 部门 | 岗位           |
      | 申请人             | vpn-requester      | -    | -              |
      | 网络管理员审批人   | network-operator   | it   | network_admin  |
      | 安全管理员审批人   | security-operator  | it   | security_admin |
    And 已发布 VPN 对话测试服务

  Scenario: 跨路由冲突 — Agent 识别并向用户澄清
    Given 用户消息为 "我要申请VPN开通，原因是网络调试和安全审计都需要"
    When 服务台 Agent 处理用户消息
    Then 工具调用序列包含 "itsm.service_match"
    And 工具调用序列包含 "itsm.service_load"
    And Agent 未调用 draft_prepare 或未继续到 draft_confirm
    And 回复内容匹配 "不同.*路|审批.*路|选择|冲突|分属|哪一个|分别"

  Scenario: 同路由多选 — Agent 合并后正常推进
    Given 用户消息为 "我要申请VPN开通，原因是网络调试和远程维护，VPN类型L2TP，申请原因是需要远程访问内网，访问时段2026-05-01 09:00:00~18:00:00"
    When 服务台 Agent 处理用户消息
    Then 工具调用序列包含 "itsm.service_match"
    And 工具调用序列包含 "itsm.service_load"
    And 工具调用序列包含 "itsm.draft_prepare"
    And draft_prepare 的路由字段为单值
    And 回复内容不匹配 "请选择|二选一|冲突"

  Scenario: 必填缺失 — Agent 追问缺失信息而非直接提交
    Given 用户消息为 "帮我开个VPN"
    When 服务台 Agent 处理用户消息
    Then 工具调用序列包含 "itsm.service_match"
    And Agent 未调用 draft_prepare 或未继续到 draft_confirm
