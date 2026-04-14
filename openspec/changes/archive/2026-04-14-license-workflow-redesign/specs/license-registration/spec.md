## ADDED Requirements

### Requirement: 客户端预注册（LicenseRegistration）数据模型
系统 SHALL 提供 `LicenseRegistration` 实体，存储客户端预提交的机器标识或硬件指纹，供后续签发 License 时选择。表名 `license_registrations`，字段包括：`id` (PK)、`product_id` (uint, FK)、`licensee_id` (uint, FK)、`code` (string, unique)、`source` (enum: `pre_registered`, `auto_generated`)、`fingerprint` (string, optional)、`expires_at` (*time.Time)、`created_at`、`updated_at`。

#### Scenario: 客户端提交预注册
- **WHEN** 客户端调用预注册 API 提交 machine fingerprint
- **THEN** 系统 MUST 创建 `LicenseRegistration` 记录，`source` 为 `pre_registered`，`code` 自动生成，设置 30 天后过期

#### Scenario: 自动生成注册码
- **WHEN** 操作员在签发界面选择"自动生成注册码"
- **THEN** 系统 MUST 即时创建 `source=auto_generated` 的 `LicenseRegistration` 记录，并将其 `code` 填入签发表单

### Requirement: 注册码管理 API
系统 SHALL 提供注册码的查询和清理 API。

#### Scenario: 查询可用注册码
- **WHEN** 用户请求某商品 + 授权主体下的可用注册码列表
- **THEN** 系统 MUST 返回 `source` 为 `pre_registered` 或 `auto_generated`、未绑定 License 且未过期的注册码列表

#### Scenario: 清理过期注册码
- **WHEN** 调度任务每日执行
- **THEN** 系统 MUST 软删除所有 `expires_at < now` 且未绑定 License 的 `LicenseRegistration` 记录

### Requirement: 签发时注册码来源变更
系统 SHALL 在 License 签发流程中，将 `registrationCode` 的来源从"任意文本输入"改为"必须从 `LicenseRegistration` 中选取或自动生成"。

#### Scenario: 使用预注册码签发
- **WHEN** 操作员选择了一个已有的 `LicenseRegistration.code`
- **THEN** 系统 MUST 校验该 code 未被其他 License 占用，然后完成签发

#### Scenario: 使用自动生成码签发
- **WHEN** 操作员触发自动生成
- **THEN** 系统 MUST 先创建 `LicenseRegistration` 记录，再使用该 `code` 完成签发

#### Scenario: 非法注册码
- **WHEN** 签发请求中提供的 `registrationCode` 不存在于 `license_registrations` 表中
- **THEN** 系统 MUST 返回错误 "无效的注册码"
