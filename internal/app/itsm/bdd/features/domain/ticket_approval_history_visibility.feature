Feature: Ticket Approval History Visibility

  Background:
    Given the following participants exist:
      | 身份 | 用户名 | 部门 | 岗位 |
      | requester | ops-access-requester | - | - |
      | ops operator | ops-operator | it | ops_admin |
      | backup ops operator | backup-ops | it | ops_admin |
      | network operator | network-operator | it | network_admin |
      | security operator | security-operator | it | security_admin |
      | admin | it-admin | it | ops_admin |
    And server access smart service is published

  Scenario: Actual completed operator can see handled ticket in approval history
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    And user "backup-ops" claims and completes the current work
    And a confirmed smart decision completes the ticket
    Then ticket status is "completed"
    And the user who completed the current work can see the ticket in approval history

  Scenario: Another eligible operator cannot be used as a substitute for approval history visibility
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    And user "backup-ops" claims and completes the current work
    And a confirmed smart decision completes the ticket
    Then ticket status is "completed"
    And the user who completed the current work can see the ticket in approval history
    And user "ops-operator" cannot see the ticket in approval history
