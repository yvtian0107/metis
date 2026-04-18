@bdd @itsm @recovery
Feature: 智能引擎恢复机制
  当服务器重启后，智能引擎应恢复被中断的决策循环

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份     | 用户名        | 部门 | 岗位       |
      | 申请人   | vpn-requester | it   | staff      |
      | 网络管理 | net-admin     | it   | net_admin  |

  Scenario: 恢复无活跃活动的智能引擎票据
    Given 存在一个状态为 "in_progress" 的智能引擎票据且无活跃活动
    When 执行智能引擎恢复扫描
    Then 恢复任务已提交

  Scenario: 跳过有活跃活动的智能引擎票据
    Given 存在一个状态为 "in_progress" 的智能引擎票据且有活跃活动
    When 执行智能引擎恢复扫描
    Then 恢复任务未提交
