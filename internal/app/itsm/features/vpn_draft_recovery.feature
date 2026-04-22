@llm
Feature: VPN 开通申请 — 草稿字段变更后 Agent 自愈重试

  验证服务台 Agent 在 draft_confirm 返回"字段已变更"错误后，
  能自动重新调用 service_load 和 draft_prepare 完成恢复。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份               | 用户名             | 部门 | 岗位           |
      | 申请人             | vpn-requester      | -    | -              |
      | 网络管理员处理人   | network-operator   | it   | network_admin  |
      | 安全管理员处理人   | security-operator  | it   | security_admin |
    And 已发布 VPN 对话测试服务

  Scenario: 草稿版本校验 — 字段变更后 Agent 自动重试
    Given 服务字段将在草稿准备后变更
    And 用户消息为 "我要申请VPN开通，类型L2TP，原因网络调试，访问时段2026-05-01 09:00:00~18:00:00"
    When 服务台 Agent 处理用户消息（含字段变更）
    Then 工具调用序列包含 "itsm.draft_confirm"
    And "itsm.service_load" 被调用至少 2 次
    And "itsm.draft_prepare" 被调用至少 2 次
