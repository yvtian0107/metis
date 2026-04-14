## 1. Database Schema & Model Foundation

- [x] 1.1 修改 `license_licenses` 表模型：新增 `lifecycle_status`、`original_license_id`、`suspended_at`、`suspended_by` 字段
- [x] 1.2 新建 `license_registrations` 表模型及迁移（字段：id, product_id, licensee_id, code, source, fingerprint, expires_at, bound_license_id）
- [x] 1.3 修改 `license_licensees` 模型：将 `contact_name`、`contact_phone`、`contact_email`、`business_info` 标记为 deprecated/移除（仅保留 name, code, notes, status）
- [x] 1.4 更新 LicenseApp.Models() 注册新模型

## 2. Backend - License Lifecycle & Registration

- [x] 2.1 实现 License 生命周期状态推导逻辑（签发时根据 validFrom 设置 pending/active；调度任务自动检测 expired）
- [x] 2.2 实现 Renewal 服务逻辑：更新 `validUntil`，恢复 active 状态
- [x] 2.3 实现 Upgrade 服务逻辑：创建新 License 记录，旧记录标记 revoked，新记录指向 `original_license_id`
- [x] 2.4 实现 Suspend / Reactivate 服务逻辑
- [x] 2.5 实现 LicenseRegistration 服务：预注册码创建、自动生成、查询可用列表、标记已绑定、清理过期任务
- [x] 2.6 修改 License 签发服务：Registration Code 必须来自 `license_registrations` 且未被占用
- [x] 2.7 新增生命周期 API Handler：`POST /:id/renew`、`:id/upgrade`、`:id/suspend`、`:id/reactivate`
- [x] 2.8 新增 Registration API Handler：预注册提交、查询可用注册码、自动生成
- [x] 2.9 修改 License 列表/详情 API：返回 `lifecycleStatus`，支持按 `lifecycleStatus` 筛选
- [x] 2.10 修改 License 导出 API：兼容 `expired`/`suspended` 状态导出，返回对应 `key_version` 的公钥

## 3. Backend - Key Compatibility & Impact Assessment

- [x] 3.1 实现 Key Rotation 影响评估服务：统计某 Product 下 `status != revoked` 且 `key_version < current` 的 License 数量
- [x] 3.2 实现批量重签服务：对指定 License ID 列表使用当前最新密钥重新签名（限制每次 <= 100）
- [x] 3.3 新增 Key Rotation 影响评估 API Handler：`GET /products/:id/rotate-key-impact`
- [x] 3.4 新增批量重签 API Handler：`POST /products/:id/bulk-reissue`
- [x] 3.5 修改 Product Key Rotation Handler：执行前先调用影响评估并展示数据（或在 UI 层处理）

## 4. Backend - Licensee Simplification & Response Cleanup

- [x] 4.1 修改 Licensee Service：创建/更新时忽略 CRM 字段
- [x] 4.2 修改 LicenseeResponse：移除 `contactName`、`contactPhone`、`contactEmail`、`businessInfo`
- [x] 4.3 修改 Licensee Handler 请求/响应绑定，确保旧字段被忽略
- [x] 4.4 更新 Seed 数据（如 Licensee 相关菜单权限有变化则同步调整）

## 5. Frontend - License Pages & Lifecycle UI

- [x] 5.1 修改 License 列表页：更新 Status Badge 颜色映射（active=绿、expired=黄、suspended=灰、revoked=红、pending=蓝）
- [x] 5.2 修改 License 列表页：增加 Registration Code 列，状态筛选器扩展为 lifecycleStatus
- [x] 5.3 修改 License 列表页：行操作根据 lifecycleStatus 显示续期/升级/暂停/恢复/吊销
- [x] 5.4 重构 IssueLicenseSheet：Registration Code 改为"选择预注册码 / 自动生成"交互
- [x] 5.5 重构 IssueLicenseSheet：选择预设 Plan 时以只读摘要卡片展示约束值，自定义时展开编辑器
- [x] 5.6 修复 IssueLicenseSheet 表单状态管理反模式（用 useEffect 替代 render 期 side effect）
- [x] 5.7 修改 License 详情页：展示 lifecycleStatus、升级链路（originalLicenseId）、暂停信息
- [x] 5.8 修改 License 详情页：根据状态条件渲染续期/升级/暂停/恢复/吊销按钮
- [x] 5.9 实现续期 Dialog 和升级 Dialog 组件

## 6. Frontend - Product Pages & Constraint Editor

- [x] 6.1 修改 Product 详情页「密钥管理」Tab：点击 Rotate Key 先展示影响评估弹窗，再确认执行
- [x] 6.2 在密钥管理弹窗中集成"批量重签"快捷入口
- [x] 6.3 优化 ConstraintEditor：默认折叠高级设置（min/max/default/key），减少视觉嵌套
- [x] 6.4 优化 ConstraintEditor：修复或移除装饰性拖拽手柄（GripVertical），避免误导

## 7. Frontend - Licensee Pages & Shared Components

- [x] 7.1 修改 Licensee 列表/详情/表单页：移除 CRM 字段（联系人、企业信息）的展示和输入
- [x] 7.2 修改 Licensee Sheet/Form：只保留 name、notes、status
- [x] 7.3 更新前端路由/菜单注册（如有新增页面则注册，无则跳过）

## 8. Scheduler & Cleanup Tasks

- [x] 8.1 注册 License 过期检测定时任务（每日执行）
- [x] 8.2 注册 LicenseRegistration 过期清理定时任务（每日执行）
- [x] 8.3 在 LicenseApp.Tasks() 中返回上述任务定义

## 9. Integration & Verification

- [x] 9.1 运行 `go build -tags dev ./cmd/server/` 验证后端编译
- [x] 9.2 运行 `cd web && bun run lint` 验证前端无 ESLint 错误
- [x] 9.3 运行 `go test ./internal/app/license/...` 验证新增逻辑
- [ ] 9.4 手动走查：签发 → 升级 → 续期 → 暂停 → 恢复 → 吊销 完整工作流
