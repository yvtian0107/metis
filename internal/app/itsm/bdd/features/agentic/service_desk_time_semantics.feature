@llm
Feature: IT 服务台智能体 — 时间语义解析

  服务台智能体在展示提单草稿前，应把明确时间表达转换为可落表的绝对时间。
  对缺少具体时刻的宽泛时段必须继续追问；对结束早于开始的时间区间默认按跨天解释。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份               | 用户名             | 部门 | 岗位           |
      | 申请人             | vpn-requester      | -    | -              |
      | 网络管理员处理人   | network-operator   | it   | network_admin  |
      | 安全管理员处理人   | security-operator  | it   | security_admin |
    And 已发布 VPN 对话测试服务

  Scenario: 明天下午 5 点 — Agent 调用时间工具并写入可落表时间
    Given 用户消息为 "我要申请VPN，线上支持用的，访问原因online_support，申请原因是远程处理线上问题，访问时段明天下午5点"
    When 服务台 Agent 处理用户消息
    Then 工具调用序列包含 "itsm.service_match"
    And 工具调用序列包含 "itsm.service_load"
    And 工具调用序列包含 "general.current_time"
    And 工具调用序列包含 "itsm.draft_prepare"
    And draft_prepare 的访问时段等于基于当前时间的明天 17 点

  Scenario: 下午 5 点 — Agent 不得把当前日期下已过去的时间写入草稿
    Given 用户消息为 "我要申请VPN，线上支持用的，访问原因online_support，申请原因是远程处理线上问题，访问时段下午5点"
    When 服务台 Agent 处理用户消息
    Then 工具调用序列包含 "general.current_time"
    And 下午 5 点没有被解析为过去时间

  Scenario: 明天晚上 — Agent 追问具体时刻而不是自行补全
    Given 用户消息为 "我要申请VPN，线上支持用的，访问原因online_support，申请原因是远程处理线上问题，访问时段明天晚上"
    When 服务台 Agent 处理用户消息
    Then 工具调用序列包含 "general.current_time"
    And Agent 未进入可确认草稿
    And 回复内容匹配 "具体.*(时间|时刻)|几点|几分|明确.*时间"

  Scenario: 22:00-01:00 — Agent 默认按跨天区间写入草稿
    Given 用户消息为 "我要申请VPN，线上支持用的，访问原因online_support，申请原因是远程处理线上问题，访问时段22:00-01:00"
    When 服务台 Agent 处理用户消息
    Then 工具调用序列包含 "general.current_time"
    And 工具调用序列包含 "itsm.draft_prepare"
    And draft_prepare 的访问时段按当前时间解析为跨天区间 "22:00" 到 "01:00"

  Scenario: 12:00-10:00 — Agent 默认按跨天区间写入草稿
    Given 用户消息为 "我要申请VPN，线上支持用的，访问原因online_support，申请原因是远程处理线上问题，访问时段12:00-10:00"
    When 服务台 Agent 处理用户消息
    Then 工具调用序列包含 "general.current_time"
    And 工具调用序列包含 "itsm.draft_prepare"
    And draft_prepare 的访问时段按当前时间解析为跨天区间 "12:00" 到 "10:00"
