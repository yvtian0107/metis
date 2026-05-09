@llm
Feature: 高风险变更协同申请（Boss）— 服务台对话校验

  验证服务台 Agent 在对话层对高风险变更协同申请的智能识别能力：
  - 必填字段缺失时追问
  - 时间窗口非法时拒绝建单
  - 枚举值非法时提示修正
  - 变更明细表（change_items）缺失或字段不完整时拒绝建单
  - 多条明细混合权限级别时完整保留

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份       | 用户名           | 部门         | 岗位            |
      | 申请人     | boss-requester-1 | -            | -               |
      | 首级处理人 | boss-head-reviewer | headquarters | serial_reviewer |
      | 二级处理人 | ops-handler      | it           | ops_admin       |
    And 已发布高风险变更协同申请对话测试服务
    And "boss-requester-1" 发起高风险变更协同申请对话

  # BS-101 ---------------------------------------------------------------
  Scenario: 缺申请主题时服务台追问
    Given 用户消息为 "我要提一个高风险变更协同申请"
    When 服务台 Boss Agent 处理对话
    Then 服务台未调用 draft_prepare
    And 回复内容匹配 "申请主题|主题|变更名称|叫什么"

  # BS-102 ---------------------------------------------------------------
  Scenario: 缺申请类别时服务台追问
    Given 用户消息为 "我要申请高风险变更，主题是支付网关升级，风险等级高，期望2026-05-20 12:00前完成，变更窗口2026-05-20 00:00到2026-05-20 02:00，影响范围支付链路，回滚要求：需要，影响模块：gateway，变更明细：system=payment-gw, resource=pgw-admin, permission_level=read_write"
    When 服务台 Boss Agent 处理对话
    Then 服务台未调用 draft_prepare
    And 回复内容匹配 "申请类别|变更类型|类别|生产变更|访问授权|应急"

  # BS-103 ---------------------------------------------------------------
  Scenario: 缺风险等级时服务台追问
    Given 用户消息为 "帮我提高风险变更申请，主题：订单系统配置变更，类别：prod_change，期望2026-05-20 18:00前完成，变更窗口2026-05-20 02:00到2026-05-20 04:00，影响范围订单核心链路，回滚要求：required，影响模块：order，变更明细：system=order-core, resource=order-admin, permission_level=read_write"
    When 服务台 Boss Agent 处理对话
    Then 服务台未调用 draft_prepare
    And 回复内容匹配 "风险等级|风险|low|medium|high"

  # BS-104 ---------------------------------------------------------------
  Scenario: 缺变更时间窗口时服务台追问
    Given 用户消息为 "我要提高风险变更协同申请，主题：监控系统规则调整，类别：prod_change，风险：medium，期望2026-05-20 20:00前完成，影响范围：监控告警链路，回滚要求：not_required，影响模块：monitoring，变更明细：system=monitor-core, resource=monitor-admin, permission_level=read"
    When 服务台 Boss Agent 处理对话
    Then 服务台未调用 draft_prepare
    And 回复内容匹配 "变更窗口|开始时间|结束时间|时间段|几点"

  # BS-105 ---------------------------------------------------------------
  Scenario: 时间窗口结束早于开始时拒绝建单
    Given 用户消息为 "提高风险变更申请，主题：网关切换，类别：prod_change，风险：high，期望完成：2026-05-20 10:00，变更窗口：2026-05-20 08:00 到 2026-05-20 06:00，影响范围：网关接入层，回滚要求：required，影响模块：gateway，变更明细：system=api-gateway, resource=gw-admin, permission_level=read_write"
    When 服务台 Boss Agent 处理对话
    Then 服务台未完成草稿确认
    And 回复内容匹配 "结束时间|时间非法|时间错误|早于|修正时间"

  # BS-106 ---------------------------------------------------------------
  Scenario: 缺影响范围时服务台追问
    Given 用户消息为 "提高风险变更，主题：支付核心升级，类别：prod_change，风险：high，期望完成：2026-05-21 06:00，变更窗口：2026-05-21 00:00 到 2026-05-21 03:00，回滚要求：required，影响模块：payment，变更明细：system=payment-core, resource=pay-admin, permission_level=read_write"
    When 服务台 Boss Agent 处理对话
    Then 服务台未调用 draft_prepare
    And 回复内容匹配 "影响范围|影响了什么|受影响|业务影响"

  # BS-107 ---------------------------------------------------------------
  Scenario: 缺回滚要求时服务台追问
    Given 用户消息为 "提高风险变更，主题：订单状态修复，类别：prod_change，风险：medium，期望完成：2026-05-21 12:00，变更窗口：2026-05-21 02:00 到 2026-05-21 04:00，影响范围：订单状态流转，影响模块：order，变更明细：system=order-svc, resource=order-readonly, permission_level=read"
    When 服务台 Boss Agent 处理对话
    Then 服务台未调用 draft_prepare
    And 回复内容匹配 "回滚|rollback|是否需要回滚|回退"

  # BS-108 ---------------------------------------------------------------
  Scenario: 缺影响模块时服务台追问
    Given 用户消息为 "提高风险变更，主题：告警阈值调整，类别：prod_change，风险：low，期望完成：2026-05-22 18:00，变更窗口：2026-05-22 10:00 到 2026-05-22 12:00，影响范围：告警通知链路，回滚要求：not_required，变更明细：system=alert-center, resource=alert-admin, permission_level=read_write"
    When 服务台 Boss Agent 处理对话
    Then 服务台未调用 draft_prepare
    And 回复内容匹配 "影响模块|模块|gateway|payment|monitoring|order"

  # BS-109 ---------------------------------------------------------------
  Scenario: 缺变更明细表时服务台追问
    Given 用户消息为 "提高风险变更，主题：支付网关升级，类别：prod_change，风险：high，期望完成：2026-05-23 06:00，变更窗口：2026-05-23 00:00 到 2026-05-23 03:00，影响范围：支付核心链路，回滚要求：required，影响模块：gateway,payment"
    When 服务台 Boss Agent 处理对话
    Then 服务台未调用 draft_prepare
    And 回复内容匹配 "变更明细|明细|系统|resource|资源账号|system"

  # BS-110 ---------------------------------------------------------------
  Scenario: 明细表字段缺失时提示补全
    Given 用户消息为 "提高风险变更，主题：监控规则变更，类别：prod_change，风险：medium，期望完成：2026-05-24 12:00，变更窗口：2026-05-24 02:00 到 2026-05-24 04:00，影响范围：监控告警，回滚要求：not_required，影响模块：monitoring，变更明细：permission_level=read"
    When 服务台 Boss Agent 处理对话
    Then 服务台未调用 draft_prepare
    And 回复内容匹配 "系统名|system|资源账号|resource|缺少|补充"

  # BS-111 ---------------------------------------------------------------
  Scenario: 明细表为空时拒绝建单
    Given 用户消息为 "提高风险变更，主题：紧急补丁，类别：emergency_support，风险：high，期望完成：2026-05-25 06:00，变更窗口：2026-05-25 00:00 到 2026-05-25 03:00，影响范围：全链路，回滚要求：required，影响模块：gateway,payment，变更明细：无"
    When 服务台 Boss Agent 处理对话
    Then 服务台未调用 draft_prepare
    And 回复内容匹配 "变更明细|明细|至少一条|请补充"

  # BS-112 ---------------------------------------------------------------
  Scenario: 申请类别枚举值非法时拒绝建单
    Given 用户消息为 "提高风险变更，主题：系统优化，类别：other，风险：high，期望完成：2026-05-26 06:00，变更窗口：2026-05-26 00:00 到 2026-05-26 03:00，影响范围：全系统，回滚要求：required，影响模块：gateway，变更明细：system=api-gw, resource=gw-admin, permission_level=read_write"
    When 服务台 Boss Agent 处理对话
    Then 服务台未完成草稿确认
    And 回复内容匹配 "类别|prod_change|access_grant|emergency_support|不支持|非法|请选择"

  # BS-114 ---------------------------------------------------------------
  Scenario: 多条明细混合权限级别应完整保留
    Given 用户消息为 "提高风险变更，主题：核心链路联合变更，类别：prod_change，风险：high，期望完成：2026-05-27 06:00，变更窗口：2026-05-27 00:00 到 2026-05-27 04:00，影响范围：网关和支付链路，回滚要求：required，影响模块：gateway,payment，变更明细：1) system=api-gateway, resource=gw-release, permission_level=read_write; 2) system=payment-core, resource=pay-readonly, permission_level=read"
    When 服务台 Boss Agent 处理对话
    Then 服务台调用了 draft_prepare
    And draft_prepare 的 change_items 包含完整的多条明细
