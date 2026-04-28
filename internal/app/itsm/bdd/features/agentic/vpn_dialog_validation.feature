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
      | 网络管理员处理人   | network-operator   | it   | network_admin  |
      | 安全管理员处理人   | security-operator  | it   | security_admin |
    And 已发布 VPN 对话测试服务

  Scenario: 完整输入 — Agent 直接整理草稿而非追问已给信息
    Given 用户消息为 "我要申请VPN，线上支持用的，wenhaowu@dev.com"
    When 服务台 Agent 处理用户消息
    Then 工具调用序列包含 "itsm.service_match"
    And 工具调用序列包含 "itsm.service_load"
    And 工具调用序列包含 "itsm.draft_prepare"
    And 回复内容不匹配 "请补充.*VPN账号|请补充.*访问原因|是否还有其他具体原因|设备型号"

  Scenario: 跨路由冲突 — Agent 识别并向用户澄清
    Given 用户消息为 "我要申请VPN开通，原因是网络调试和安全审计都需要"
    When 服务台 Agent 处理用户消息
    Then 工具调用序列包含 "itsm.service_match"
    And 工具调用序列包含 "itsm.service_load"
    And Agent 未调用 draft_prepare 或未继续到 draft_confirm
    And 回复内容匹配 "不同.*路|处理.*路|选择|冲突|分属|哪一个|分别"

  Scenario: 同路由多选 — Agent 合并后正常推进
    Given 用户消息为 "我要申请VPN开通，访问原因选online_support，VPN类型L2TP，申请原因是需要远程访问内网进行线上支持和故障排查，访问时段2026-05-01 09:00:00~18:00:00"
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
