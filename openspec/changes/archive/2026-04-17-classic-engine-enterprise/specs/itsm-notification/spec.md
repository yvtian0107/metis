## ADDED Requirements

### Requirement: ClassicEngine accepts optional NotificationSender interface

ClassicEngine SHALL accept an optional NotificationSender interface at construction time. When the interface is nil, all notification operations SHALL be silently skipped without error.

#### Scenario: No NotificationSender configured — skip silently
- **WHEN** a workflow reaches a notify node and no NotificationSender is configured
- **THEN** the engine skips the notification step without error
- **THEN** workflow progression continues normally
- **THEN** a timeline entry is recorded indicating notification was skipped

### Requirement: NotificationSender interface definition

The NotificationSender interface SHALL define the method: Send(ctx context.Context, channelID uint, subject string, body string, recipients []uint) error. The channelID identifies the Kernel MessageChannel, subject is the notification subject line, body is the rendered notification content, and recipients is a list of user IDs.

#### Scenario: Successful notification via NotificationSender
- **WHEN** a workflow reaches a notify node with channel_id=1 and two resolved participant user IDs
- **WHEN** NotificationSender is configured
- **THEN** handleNotify calls NotificationSender.Send with channelID=1, the rendered subject, the rendered body, and the two user IDs
- **THEN** the notification is delivered successfully
- **THEN** workflow progression continues to the next node

### Requirement: handleNotify resolves channel and participants from node config

handleNotify SHALL call NotificationSender.Send with the node's configured channel_id and the resolved participant user IDs from the node's participant configuration. The subject and body SHALL be sourced from the node's configuration properties.

#### Scenario: Notification with template variables rendered
- **WHEN** a notify node has body template "工单 {{ticket.code}} 当前状态: {{ticket.status}}, 活动: {{activity.name}}, 备注: {{var.remark}}"
- **WHEN** the ticket code is "INC-001", status is "in_progress", activity name is "主管审批", and process variable remark is "请尽快处理"
- **THEN** the rendered body is "工单 INC-001 当前状态: in_progress, 活动: 主管审批, 备注: 请尽快处理"

### Requirement: Notification body supports template variables

Notification body SHALL support the following template variables: {{ticket.code}}, {{ticket.status}}, {{activity.name}}, and any process variable via {{var.xxx}} where xxx is the variable key. Unresolved template variables SHALL be replaced with empty strings.

#### Scenario: Template variable for undefined process variable resolves to empty
- **WHEN** a notify node body contains "{{var.undefined_key}}"
- **WHEN** no process variable with key "undefined_key" exists
- **THEN** the template variable is replaced with an empty string

### Requirement: Notification failure does not block workflow

Notification failure SHALL NOT block workflow progression. When NotificationSender.Send returns an error, the engine SHALL record a warning-level entry in the ticket timeline and continue executing the next workflow node.

#### Scenario: Notification failure is non-blocking
- **WHEN** a workflow reaches a notify node
- **WHEN** NotificationSender.Send returns an error
- **THEN** a timeline entry is recorded with level "warning" describing the notification failure
- **THEN** workflow progression continues to the next node without interruption
