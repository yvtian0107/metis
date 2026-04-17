## MODIFIED Requirements

### Requirement: itsm.draft_prepare 工具
系统 SHALL 注册 `itsm.draft_prepare` 工具，用于在向用户展示草稿前登记当前版本并校验表单字段。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {
    "summary": { "type": "string", "description": "工单摘要" },
    "form_data": { "type": "object", "description": "表单字段键值对（必须是完整表单，不能只传增量）" }
  },
  "required": ["summary", "form_data"]
}
```

**返回结构**:
```json
{
  "ok": true,
  "draft_version": 2,
  "summary": "申请VPN临时接入",
  "form_data": { "vpn_type": "临时远程", "duration": "2026-04-16 20:00:00~22:00:00" },
  "warnings": []
}
```

#### Scenario: 成功登记草稿
- **WHEN** Agent 调用 itsm.draft_prepare，传入完整的 summary 和 form_data，服务已加载
- **THEN** 系统 SHALL 校验 form_data 中的必填字段、选项值合法性，自增 draft_version，更新 Session.State，stage 更新为 `awaiting_confirmation`

#### Scenario: 必填字段缺失
- **WHEN** form_data 中缺少服务表单定义的必填字段
- **THEN** 系统 SHALL 在 warnings 中返回缺失字段列表 `[{"type": "missing_required", "field_key": "vpn_type", "field_label": "VPN 类型"}]`

#### Scenario: 无效选项值
- **WHEN** form_data 中某个 select 字段的值不在选项列表中
- **THEN** 系统 SHALL 在 warnings 中返回 `[{"type": "invalid_option", "field_key": "vpn_type", "value": "xxx", "valid_options": ["临时远程", "长期远程"]}]`

#### Scenario: 单选字段传入多值且为路由字段
- **WHEN** form_data 中某个单选字段传入了逗号分隔的多个值
- **AND** 该字段是 RoutingFieldHint.FieldKey（路由决策字段）
- **THEN** 系统 SHALL 在 warnings 中返回 `multivalue_on_single_field` 类型警告，附带 `resolved_values` 数组，每个元素包含 `value`（原始值）和 `route`（该值在 OptionRouteMap 中对应的路由分支标签）
- **AND** 若某个值不在 OptionRouteMap 中，`route` SHALL 为空字符串

#### Scenario: 单选字段传入多值但非路由字段
- **WHEN** form_data 中某个单选字段传入了逗号分隔的多个值
- **AND** 该字段不是路由决策字段
- **THEN** 系统 SHALL 在 warnings 中返回 `multivalue_on_single_field` 类型警告，不包含 `resolved_values`

#### Scenario: 服务未加载时调用
- **WHEN** Agent 调用 itsm.draft_prepare 但 Session.State 中无 loaded_service_id
- **THEN** 系统 SHALL 返回错误 `{"ok": false, "error": "请先调用 service_load 加载服务详情"}`

#### Scenario: 草稿内容变更自增版本
- **WHEN** 本次传入的 summary 或 form_data 与上一次 draft_prepare 不同
- **THEN** 系统 SHALL 自增 draft_version，将 confirmed_draft_version 清空（需要重新确认）
