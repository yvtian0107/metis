## 1. bddContext 扩展

- [x] 1.1 在 `steps_common_test.go` 的 `bddContext` 中新增 `priority *Priority` 字段
- [x] 1.2 在 `reset()` 中将 `bc.priority` 设为 nil

## 2. ParticipantResolver 支持 position_department

- [x] 2.1 在 `engine/workflow.go` 的 `Participant` 结构体新增 `PositionCode` 和 `DepartmentCode` 字段
- [x] 2.2 在 `engine/resolver.go` 的 `OrgService` 接口新增 `FindUsersByPositionCodeAndDepartmentCode(positionCode, departmentCode string) ([]uint, error)` 方法
- [x] 2.3 在 `engine/resolver.go` 的 `Resolve` 中新增 `position_department` case
- [x] 2.4 在 `steps_common_test.go` 的 `testOrgService` 中实现 `FindUsersByPositionCodeAndDepartmentCode`

## 3. LLM 工作流生成 fixture

- [x] 3.1 重写 `vpn_support_test.go`：用 LLM 生成 VPN 工作流（使用 VPN collaboration spec + itsmGeneratorSystemPrompt），包含重试和验证逻辑
- [x] 3.2 定义 `vpnSampleFormData` 变量
- [x] 3.3 实现 `publishVPNService(bc *bddContext, cfg llmConfig) error`：LLM 生成 workflow → 创建 ServiceCatalog + Priority + ServiceDefinition

## 4. BDD suite 环境门控

- [x] 4.1 在 `bdd_test.go` 的 `TestBDD` 中添加 LLM 环境变量检测，缺失时 skip 整个 suite
- [x] 4.2 在 vpn_support_test.go 中定义 `llmConfig` 和 `hasLLMConfig`/`requireLLMConfig` 辅助函数

## 5. 验证

- [x] 5.1 运行 `go test ./internal/app/itsm/ -run TestBDD -v` 确认编译通过（无 LLM env 时 skip）
- [x] 5.2 删除 `vpn_support_validate_test.go`（硬编码 fixture 测试不再需要）
