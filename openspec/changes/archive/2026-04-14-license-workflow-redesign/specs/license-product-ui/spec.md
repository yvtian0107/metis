## ADDED Requirements

### Requirement: 密钥轮转影响评估弹窗
系统 SHALL 在商品详情页的「密钥管理」Tab 中，点击"密钥轮转"按钮后，先展示影响评估弹窗，再要求二次确认。

#### Scenario: 展示受影响许可数量
- **WHEN** 用户在密钥管理 Tab 点击"密钥轮转"
- **THEN** 系统 MUST 先调用影响评估 API，弹窗中显示"当前有 N 条历史许可使用该商品的旧密钥签名"

#### Scenario: 批量重签入口
- **WHEN** 影响评估弹窗中显示 N > 0
- **THEN** 弹窗 MUST 提供"先批量重签再轮转"的快捷按钮，点击后跳转/打开批量重签 Dialog

#### Scenario: 无影响时直接确认
- **WHEN** 影响评估弹窗中显示 N = 0
- **THEN** 弹窗 MUST 简化为普通二次确认，用户确认后立即执行轮转

## MODIFIED Requirements

### Requirement: 密钥管理 Tab
系统 SHALL 在商品详情页提供密钥管理 Tab，展示当前密钥版本和公钥，提供密钥轮转按钮（需二次确认）。

#### Scenario: 查看当前密钥
- **WHEN** 用户查看密钥管理 Tab
- **THEN** 展示当前密钥版本和公钥，以及"密钥轮转"按钮

#### Scenario: 密钥轮转流程
- **WHEN** 用户点击"密钥轮转"按钮
- **THEN** 系统 MUST 先调用影响评估 API 展示弹窗，用户再次确认后才执行实际的轮转操作
