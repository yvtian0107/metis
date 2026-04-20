## MODIFIED Requirements

### Requirement: 工具错误返回格式
工具执行失败时 SHALL 返回结构化错误 JSON 而非抛出异常，让 Agent 能够理解错误并继续推理。工具参数解析失败时 SHALL 返回明确的参数错误信息。

#### Scenario: 工具执行失败
- **WHEN** 工具查询数据库出错
- **THEN** 工具 SHALL 返回 `{"error": true, "message": "具体错误描述"}`，ReAct 循环将此作为 tool result 追加到消息中

#### Scenario: 工具参数 JSON 解析失败
- **WHEN** Agent 传入的参数 JSON 格式错误（无法 unmarshal）
- **THEN** 工具 SHALL 返回 `{"error": true, "message": "参数格式错误: <具体解析错误>"}`
- **AND** 不得静默使用零值参数
