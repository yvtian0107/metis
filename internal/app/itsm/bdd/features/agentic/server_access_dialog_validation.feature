@llm
Feature: Server Access Dialog Validation

  Background:
    Given server access dialog participants exist:
      | 身份 | 用户名 | 部门 | 岗位 |
      | 申请人 | ops-access-requester | - | - |
      | 运维处理人 | ops-operator | it | ops_admin |
      | 网络处理人 | network-operator | it | network_admin |
      | 安全处理人 | security-operator | it | security_admin |
    And server access dialog service is published
    And server access dialog is open for requester "ops-access-requester"

  Scenario: Missing target host must trigger follow-up
    When requester says "我要临时访问生产环境排查问题，请帮我提单"
    And the server access agent processes the dialog
    Then tool call sequence contains "itsm.service_match"
    And tool call sequence contains "itsm.service_load"
    And agent does not call draft_prepare or draft_confirm
    And response matches "目标服务器|哪台服务器|服务器名"

  Scenario: Missing access window must trigger follow-up
    When requester says "需要登录 prod-app-02，来源 IP 10.20.30.41，账号 ops.reader，排查应用进程异常"
    And the server access agent processes the dialog
    Then tool call sequence contains "itsm.service_match"
    And tool call sequence contains "itsm.service_load"
    And agent does not call draft_prepare or draft_confirm
    And response matches "访问时段|时间窗口|几点到几点"

  Scenario: Past access window must be corrected before drafting
    When requester says "请帮我提交生产服务器临时访问申请，目标服务器 prod-app-02，来源 IP 10.20.30.41，账号 ops.reader，访问时间 2026-04-28 20:00-21:00，原因是排查应用进程异常"
    And the server access agent processes the dialog
    Then tool call sequence contains "general.current_time"
    And agent does not call draft_prepare
    And response matches "时间.*已过|访问时间.*过去|请修改时间"

  Scenario: Multiple target hosts are preserved in draft
    When requester says "请帮我提交生产服务器临时访问申请，需要访问 prod-app-02, prod-app-03，来源 IP 10.20.30.41，账号 ops.reader，今晚 20:00-21:00 排查应用进程异常"
    And the server access agent processes the dialog
    Then tool call sequence contains "itsm.draft_prepare"
    And draft_prepare field "target_host" contains all of "prod-app-02,prod-app-03"

  Scenario: Mixed cross-route requests must be clarified
    When requester says "需要登录 prod-app-02，今晚 20:00-21:00，来源 IP 10.20.30.41，账号 ops.reader，一边排查进程异常一边调整防火墙策略"
    And the server access agent processes the dialog
    Then tool call sequence contains "itsm.service_load"
    And agent does not call draft_prepare or draft_confirm
    And response matches "澄清|分别属于|哪个诉求|先办理哪一个"

  Scenario: Latest clarification overrides earlier ops intent
    When requester says "需要登录 prod-app-02，今晚 20:00-21:00，来源 IP 10.20.30.41，账号 ops.reader，排查应用进程异常"
    And requester says "补充一下，不是普通运维排障，是为了保全异常访问证据做安全取证"
    And the server access agent processes the dialog
    Then tool call sequence contains "itsm.draft_prepare"
    And draft_prepare field "request_kind" equals "security_investigation"
    And response matches "安全|取证|证据"

  Scenario: Abnormal access evidence semantics should prefer security
    When requester says "为了保全异常访问证据，需要进入生产机核查日志，目标服务器 prod-app-03，来源 IP 10.20.30.43，账号 sec.audit，今晚 23:00-23:45"
    And the server access agent processes the dialog
    Then tool call sequence contains "itsm.draft_prepare"
    And draft_prepare field "request_kind" equals "security_investigation"
    And response matches "异常访问|取证|安全"
