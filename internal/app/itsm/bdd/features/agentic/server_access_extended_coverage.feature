Feature: Server Access Extended Coverage

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

  Scenario: Wrong role cannot handle an ops server access ticket
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    Then the current work is assigned to position "ops_admin"
    And user "network-operator" cannot claim the current work
    And user "network-operator" cannot process the current work

  Scenario: Requester without required position cannot handle the current work
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    Then user "ops-access-requester" cannot claim the current work
    And user "ops-access-requester" cannot process the current work

  Scenario: Requester can handle the current work when also matching the collaboration role
    Given user "ops-access-requester" also belongs to department "it" and position "ops_admin"
    And requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    Then user "ops-access-requester" can claim the current work
    And user "ops-access-requester" can process the current work
    When user "ops-access-requester" claims and completes the current work
    And a confirmed smart decision completes the ticket
    Then ticket status is "completed"

  Scenario: Admin can reassign the current server access work
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    And admin "it-admin" reassigns the current work to "backup-ops" because "temporary workload rebalancing"
    Then timeline contains event type "override_reassign"
    And the current ticket assignee is "backup-ops"

  Scenario: Inactive routed members should not silently complete the ticket
    Given all members of position "ops_admin" are inactive
    And requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    Then ticket status is not "completed"
    And current activity status is "pending"
    And timeline contains event type "participant_resolution_pending"

  Scenario: Empty routed position should not silently complete the ticket
    Given position "ops_admin" has no members
    And requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    Then ticket status is not "completed"
    And current activity status is "pending"
    And timeline contains event type "participant_resolution_pending"

  Scenario: Admin can cancel an in-progress server access ticket
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    And admin "it-admin" cancels the in-progress ticket because "manual stop for emergency review"
    Then ticket status is "cancelled"
    And timeline contains event type "ticket_cancelled"
    And all activities and assignments are cancelled

  Scenario: Server access ticket persists the required form data
    Given requester "ops-access-requester" has a server access ticket for "ops"
    Then the server access form data persists all key fields

  Scenario: Server access ticket exposes AI routing reasoning
    Given requester "ops-access-requester" has a server access ticket for "security"
    When a confirmed smart decision creates work for position "security_admin"
    Then the latest activity exposes AI reasoning and confidence

  Scenario: Server access timeline records the core lifecycle
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    And the current assignee claims and completes the work
    And a confirmed smart decision completes the ticket
    Then ticket status is "completed"
    And the server access timeline contains the core lifecycle

  Scenario: Requester can see in-progress server access ticket in my tickets
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    Then requester "ops-access-requester" can see the ticket in my tickets

  Scenario: Requester can still see completed server access ticket in my tickets
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    And the current assignee claims and completes the work
    And a confirmed smart decision completes the ticket
    Then ticket status is "completed"
    And requester "ops-access-requester" can still see the ticket in my tickets after completion

  Scenario: Routed operator can see server access ticket in pending approvals
    Given requester "ops-access-requester" has a server access ticket for "network"
    When a confirmed smart decision creates work for position "network_admin"
    Then operator "network-operator" can see the ticket in pending approvals

  Scenario: The actual completed operator can see processed server access ticket in approval history
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    And the current assignee claims and completes the work
    And a confirmed smart decision completes the ticket
    Then ticket status is "completed"
    And the user who completed the current work can see the ticket in approval history

  Scenario: Repeated completion does not advance the ticket twice
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    And the current assignee claims and completes the work
    And a confirmed smart decision completes the ticket
    Then ticket status is "completed"
    When user "ops-operator" attempts to complete the latest finished work again
    Then the last operation failed

  Scenario: Server access ticket is visible in monitor with core fields
    Given requester "ops-access-requester" has a server access ticket for "ops"
    When a confirmed smart decision creates work for position "ops_admin"
    Then the ticket is visible in monitor with service, step, owner, waiting time, and SLA
