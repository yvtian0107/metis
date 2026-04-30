@llm
Feature: 生产服务器临时访问申请 — Agentic 边界场景

  用真实 LLM + 真实 SmartEngine 工具链压测生产服务器临时访问申请的边界语义。
  协作规范是事实源，workflow_json 是辅助背景；form.access_reason、form.operation_purpose 和工具事实优先于模型猜测。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份             | 用户名               | 部门 | 岗位           |
      | 申请人           | ops-access-requester | -    | -              |
      | 运维管理员处理人 | ops-operator         | it   | ops_admin      |
      | 网络管理员处理人 | network-operator     | it   | network_admin  |
      | 安全管理员处理人 | security-operator    | it   | security_admin |
    And 已定义生产服务器临时访问申请协作规范
    And 已基于协作规范发布生产服务器临时访问服务（智能引擎）

  Scenario: 应用排障带安全窗口字样仍路由运维管理员
    Given "ops-access-requester" 已创建生产服务器访问工单，表单数据为:
      """
      {"target_servers":"prod-app-11","access_window":"今晚 20:00 到 21:00","operation_purpose":"查看应用日志和运行状态。","access_reason":"应用进程排障，在高敏发布安全窗口内查看应用日志和运行状态。"}
      """
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "ops_admin"
    And 当前处理任务未分配到岗位 "network_admin"
    And 当前处理任务未分配到岗位 "security_admin"

  Scenario: 网络抓包带生产安全窗口字样仍路由网络管理员
    Given "ops-access-requester" 已创建生产服务器访问工单，表单数据为:
      """
      {"target_servers":"prod-gateway-11","access_window":"今晚 21:00 到 22:00","operation_purpose":"抓包核对链路连通性。","access_reason":"抓包核对链路连通性和防火墙策略，备注说明只能在生产安全窗口内操作。"}
      """
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "network_admin"
    And 当前处理任务未分配到岗位 "ops_admin"
    And 当前处理任务未分配到岗位 "security_admin"

  Scenario: 安全取证带登录排查字样仍路由安全管理员
    Given "ops-access-requester" 已创建生产服务器访问工单，表单数据为:
      """
      {"target_servers":"prod-app-12","access_window":"今晚 23:00 到 23:45","operation_purpose":"临时登录生产机核查异常访问痕迹。","access_reason":"安全审计取证和证据保全，需要临时登录生产机核查异常访问痕迹。"}
      """
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "security_admin"
    And 当前处理任务未分配到岗位 "ops_admin"
    And 当前处理任务未分配到岗位 "network_admin"

  Scenario: 运维与安全目的冲突时不得高置信选择单一路由
    Given "ops-access-requester" 已创建生产服务器访问工单，表单数据为:
      """
      {"target_servers":"prod-app-13","access_window":"今晚 19:00 到 20:00","operation_purpose":"排查生产应用进程异常。","access_reason":"既要排查生产应用进程异常，又要对异常访问证据做取证保全。"}
      """
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 不得高置信选择单一路由
    And 进入澄清或低置信人工处置
    And 决策诊断事件已记录

  Scenario: 网络与安全目的冲突时不得高置信选择单一路由
    Given "ops-access-requester" 已创建生产服务器访问工单，表单数据为:
      """
      {"target_servers":"prod-gateway-12","access_window":"今晚 22:00 到 23:00","operation_purpose":"抓包核对 ACL 和防火墙策略。","access_reason":"既要抓包核对 ACL 和防火墙策略，又要做入侵排查和安全取证。"}
      """
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 不得高置信选择单一路由
    And 进入澄清或低置信人工处置
    And 决策诊断事件已记录

  Scenario: 缺失访问目的时不得高置信选择单一路由
    Given "ops-access-requester" 已创建生产服务器访问工单，表单数据为:
      """
      {"target_servers":"prod-app-14","access_window":"今晚 20:00 到 21:00"}
      """
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 不得高置信选择单一路由
    And 进入澄清或低置信人工处置
    And 决策诊断事件已记录

  Scenario: 未知访问目的时不得高置信选择单一路由
    Given "ops-access-requester" 已创建生产服务器访问工单，表单数据为:
      """
      {"target_servers":"prod-app-15","access_window":"今晚 20:00 到 21:00","access_reason":"临时查看一个没有说明用途的生产对象。"}
      """
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 不得高置信选择单一路由
    And 进入澄清或低置信人工处置
    And 决策诊断事件已记录

  Scenario: workflow_json 错误把网络节点标成安全管理员时必须以协作规范为准
    Given 生产服务器访问工作流参考图错误地把岗位 "network_admin" 标成 "security_admin"
    And "ops-access-requester" 已创建生产服务器访问工单，场景为 "network"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "network_admin"
    And 当前处理任务未分配到岗位 "security_admin"
    And AI 决策依据包含 "协作规范"

  Scenario: workflow_json 错误把运维节点标成网络管理员时必须以协作规范为准
    Given 生产服务器访问工作流参考图错误地把岗位 "ops_admin" 标成 "network_admin"
    And "ops-access-requester" 已创建生产服务器访问工单，场景为 "ops"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "ops_admin"
    And 当前处理任务未分配到岗位 "network_admin"
    And AI 决策依据包含 "协作规范"

  Scenario: 安全处理人不可解析时不得 fallback 到运维或网络管理员
    Given 生产服务器访问岗位 "security_admin" 处理人已停用
    And "ops-access-requester" 已创建生产服务器访问工单，场景为 "security"
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 当前岗位 "ops_admin" 的活跃处理任务数为 0
    And 当前岗位 "network_admin" 的活跃处理任务数为 0
    And 没有不可执行的高置信人工任务
    And 决策诊断事件已记录

  Scenario: 已有待处理网络任务时再次决策不得重复创建
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "network"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前岗位 "network_admin" 的活跃处理任务数为 1
    When 智能引擎再次执行决策循环
    Then 当前活跃人工任务数为 1
    And 当前岗位 "network_admin" 的活跃处理任务数为 1

  Scenario: completed 终态再次决策不得新增活动或改写结果
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "ops"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"
    And 工单结果为 "fulfilled"
    And 工单活动数保持为 2
    When 智能引擎再次执行决策循环
    Then 工单状态为 "completed"
    And 工单结果为 "fulfilled"
    And 工单活动数保持为 2

  Scenario: 错误 rejected workflow_json 不得诱导申请人补充
    Given 生产服务器访问工作流参考图错误地把驳回指向申请人补充表单
    And "ops-access-requester" 已创建生产服务器访问工单，场景为 "network"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "network_admin"
    When 当前活动的被分配人驳回，意见为 "访问理由不符合生产服务器访问规范"
    And 智能引擎再次执行决策循环
    Then 不得创建申请人补充表单
    And 工单处于驳回终态或已有决策诊断
