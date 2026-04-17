## 1. bddContext 扩展

- [x] 1.1 在 `steps_common_test.go` 中扩展 `bddContext` 结构体，增加 engine、参与人 map、工单生命周期字段
- [x] 1.2 扩展 `reset()` 方法：初始化所有 map 字段，清空 ticket/service

## 2. AutoMigrate 全量模型

- [x] 2.1 在 `reset()` 中 AutoMigrate 全部模型（Kernel User/Role + Org 3 + AI 2 + ITSM 17），import `metis/internal/app/org` 和 `metis/internal/app/ai`

## 3. 引擎组件

- [x] 3.1 实现 `testOrgService`（FindUsersByPositionID/FindUsersByDepartmentID/FindManagerByUserID 直接查 DB）
- [x] 3.2 实现 `noopSubmitter`（SubmitTask 返回 nil）
- [x] 3.3 在 `reset()` 中构建 ClassicEngine：ParticipantResolver(testOrgService) + noopSubmitter

## 4. 公共 Given 步骤

- [x] 4.1 实现 `givenSystemInitialized` 步骤（no-op）
- [x] 4.2 实现 `givenParticipants` 步骤（解析 DataTable → 创建 User/Department/Position/UserPosition → 存入 bddContext map）

## 5. 步骤注册

- [x] 5.1 在 `bdd_test.go` 的 `initializeScenario` 中注册公共 Given 步骤，移除 `_ = bc` 占位

## 6. 验证

- [x] 6.1 运行 `make test-bdd` 确认编译通过且无 panic（现有 @wip example.feature 被跳过）
