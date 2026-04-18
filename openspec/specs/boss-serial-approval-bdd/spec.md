# boss-serial-approval-bdd

## Purpose

ITSM 智能引擎两级串签（Serial Approval）审批流程的 BDD 验证规范，覆盖完整流程、审批隔离、复杂表单保留及并行工单隔离场景。

## Requirements

### Requirement: 两级串签完整流程验证
智能引擎 SHALL 能够按照协作规范依次安排首级审批（指定用户 serial-reviewer）和二级审批（信息部运维管理员岗位 it/ops_admin），二级审批通过后 SHALL 完结工单。

#### Scenario: 完整串签 — 首级指定用户审批 → 二级部门岗位审批 → 完成
- **WHEN** 申请人创建高风险变更工单，智能引擎执行首次决策循环
- **THEN** 工单状态为 in_progress，且当前活动为 approve 类型，分配给指定用户 serial-reviewer
- **WHEN** serial-reviewer 认领并审批通过，智能引擎执行后续决策循环
- **THEN** 当前活动为 approve 类型，分配到岗位 ops_admin（部门 it）
- **WHEN** 二级审批人认领并审批通过，智能引擎执行决策循环直到完成
- **THEN** 工单状态为 completed

### Requirement: 串签审批隔离验证
首级审批 SHALL 仅对指定用户可见，二级审批人不能操作首级审批；反之，首级审批人也不能认领二级审批的部门岗位任务。

#### Scenario: 审批隔离 — 二级审批人无法操作首级审批，首级审批人无法认领二级审批
- **WHEN** 智能引擎安排首级审批给 serial-reviewer
- **THEN** 当前审批仅对 serial-reviewer 可见
- **THEN** ops-approver（二级审批人）认领当前工单 SHALL 失败
- **WHEN** 首级审批通过后，智能引擎安排二级审批到 it/ops_admin
- **THEN** 当前审批分配到岗位 ops_admin
- **THEN** serial-reviewer（首级审批人）认领当前工单 SHALL 失败

### Requirement: 复杂表单明细保留验证
工单创建时提交的复杂表单数据（包含 resource_items 结构化明细表格）SHALL 在工单记录中完整保留，明细表格的数组结构和所有字段值不得丢失。

#### Scenario: 复杂表单 — resource_items 明细表格跨工单完整保留
- **WHEN** 申请人创建包含 resource_items 明细表格的高风险变更工单
- **THEN** 工单的 form_data 中 SHALL 包含完整的 resource_items 数组
- **THEN** 每条明细记录的 system_name、resource_account、permission_level、target_operation 字段值 SHALL 与提交时一致

### Requirement: 并行串签工单隔离验证
两张独立的高风险变更工单 SHALL 各自拥有独立的审批链和指派记录，不得交叉污染。

#### Scenario: 并行工单 — 两张串签工单的审批指派完全隔离
- **WHEN** 申请人甲和申请人乙分别创建高风险变更工单 A 和 B
- **THEN** 工单 A 和工单 B 各自独立完成首级审批 → 二级审批 → 完成
- **THEN** 工单 A 的审批 assignment 记录中不包含工单 B 的 ticket_id，反之亦然
