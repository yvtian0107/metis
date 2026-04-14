## ADDED Requirements

### Requirement: 密钥轮转影响评估
系统 SHALL 在执行 Product Key Rotation 前，提供受影响 License 的数量统计。统计范围：该 Product 下所有 `status != revoked` 且 `key_version < 当前即将被轮换的版本` 的 License 记录。

#### Scenario: 获取影响评估
- **WHEN** 用户请求查看某商品的密钥轮转影响
- **THEN** 系统 MUST 返回受影响 License 的总数，以及按 `key_version` 分组的统计

### Requirement: 批量重签（Re-issue）
系统 SHALL 提供批量重签 API，允许对因密钥轮转而失效的历史 License 使用新密钥重新签名，生成新的 `activationCode` 并更新 `key_version`。

#### Scenario: 批量重签成功
- **WHEN** 用户提交批量重签请求，指定一组 License ID（最多 100 条）
- **THEN** 系统 MUST 对每条 License 使用当前最新密钥重新构建 payload、签名、生成 activationCode，并更新记录的 `signature`、`activationCode`、`key_version`

#### Scenario: 超出批量上限
- **WHEN** 用户提交的 License ID 数量超过 100
- **THEN** 系统 MUST 返回错误 "单次批量重签不得超过 100 条"

#### Scenario: 重签已吊销许可
- **WHEN** 批量重签列表中包含 `revoked` 状态的 License
- **THEN** 系统 MUST 跳过这些记录，并在响应中返回跳过数量和原因

### Requirement: 多版本公钥验证支持
系统 SHALL 在导出 .lic 文件和提供验证接口时，支持根据 License 的 `key_version` 获取对应版本的公钥。

#### Scenario: 导出历史版本许可
- **WHEN** 用户导出一条 `key_version=2` 的 License
- **THEN** 系统 MUST 返回 `key_version=2` 对应的公钥，而不是仅返回当前最新公钥

#### Scenario: 验证历史签名
- **WHEN** 外部系统使用 `/api/v1/license/verify` 验证一个旧版 License 的 activationCode
- **THEN** 系统 MUST 从 activationCode 中解析 `kv`（key version），并获取对应版本的公钥执行验证
