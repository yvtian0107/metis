## Context

当前 License 模块已完成底层基建：Product（商品定义）、Plan（套餐规格）、License（权利凭证）三层模型，以及 Ed25519 签名、.lic 文件导出、Casbin 权限控制等核心能力。但前端工作流和状态管理仍停留在 MVP 阶段，无法满足真实 B2B 授权运营需求。

## Goals / Non-Goals

**Goals:**
- 建立完整的 License 生命周期状态机（pending → active → expired/suspended → revoked）。
- 消除 Registration Code 自由文本带来的脏数据风险，引入客户端预注册或自动生成模式。
- 让 Key Rotation 从"盲操作"变为"有影响的确认操作"。
- 精简 Licensee 模型，使其回归轻量主体引用。
- 修复前端表单反模式，优化信息架构和视觉反馈。

**Non-Goals:**
- 不改动 Ed25519 签名算法和 .lic 文件格式（保持向后兼容）。
- 不引入外部 License Server（Flexera、Keygen 等），系统仍是自研实现。
- 不重构 ConstraintSchema 的抽象能力，只优化编辑器交互。

## Decisions

### 1. License 状态机：在现有 `issued/revoked` 上扩展，而非替换
**决策**：保留 `issued` 和 `revoked` 的存储值，新增 `pending`、`active`、`expired`、`suspended`。
**理由**：
- 已有代码和 UI 大量依赖 `issued/revoked`；完全替换会产生大面积 BREAKING 变更。
- `issued` 可继续作为"非 revoked"的聚合状态在旧 API 中使用，新 API 返回细粒度状态。
- 迁移成本最低：只需在 `License` 表增加一个 `lifecycle_status` 或把 `status` 字段扩展即可。

**替代方案**：把 `status` 彻底重定义为 5 态机。被否决，因为会中断所有现有查询和 UI 判断逻辑。

### 2. Renewal 和 Upgrade：新建记录还是修改原记录？
**决策**：Renewal 修改原记录（更新 `validUntil` 并追加历史日志）；Upgrade 创建新记录，并在新记录中保存 `original_license_id` 指向旧记录。
**理由**：
- Renewal 只是时间延长，权利主体和配置不变，修改原记录最自然，且保持客户侧的 License ID 不变。
- Upgrade 涉及 Plan 或 ConstraintValues 的变更，属于新的权利契约，创建新记录能保留完整历史链条，便于审计和追踪。

### 3. Registration Code：预注册 vs 自动生成
**决策**：引入 `LicenseRegistration` 中间表，支持两种来源：
- `pre_registered`：客户端调用公开 API 上传 machine fingerprint，操作员在签发时从下拉列表选择。
- `auto_generated`：系统按 `{licensee_code}-{random}` 或 UUID 生成，签发时自动填充。
**理由**：
- 预注册模式适合需要绑定硬件的大型部署（服务器许可）。
- 自动生成模式适合 SaaS 订阅或不需要硬件绑定的场景。
- 中间表解耦了"注册码池"与"License 记录"，后续可扩展 quota 管理（如一个 Licensee 最多预注册 N 台机器）。

### 4. Licensee CRM 字段：剥离而非立即删除
**决策**：在 API Response 中不再返回 `contactPhone`、`contactEmail`、`businessInfo` 等字段；数据库表保留这些列但标记为 deprecated，由后续数据清理任务处理。
**理由**：
- 避免立即执行 destructive migration（删除列可能导致回滚困难）。
- 当前数据量小，逻辑忽略比物理删除更安全。

### 5. Key Rotation 影响评估：运行时统计还是预计算？
**决策**：运行时通过 `license_licenses` 表按 `product_id + key_version < current_version AND status != revoked` 实时统计。
**理由**：
- 数据量可控（一个 product 的 license 通常在千级以内），实时查询无需额外索引或缓存。
- 不引入新表或后台任务，实现简单。

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| License 状态扩展后，旧 UI 可能无法识别新状态 | 在 API 层为新状态提供兼容映射（如 `active` 对外暴露时仍可兼容旧 UI 的 `issued` 判断） |
| Key Rotation + 批量重签可能产生大量签名计算 | 限制批量重签每次最多 100 条，超出时引导用户分批次操作 |
| 移除 Licensee CRM 字段可能影响已有业务依赖 | 在变更前确认没有其他模块读取这些字段；Response 中保留空字段以避免前端解构报错 |
| Registration 预注册表可能积累僵尸记录 | 增加 `expires_at` 字段，定期清理超过 30 天未绑定的预注册记录 |

## Migration Plan

1. **Schema Migration**：
   - `license_licenses` 表增加 `lifecycle_status`（默认值为 `'active'` 或根据 `validUntil` 推导）、`original_license_id`（nullable FK）、`suspended_at`、`suspended_by`。
   - 新建 `license_registrations` 表（id, product_id, licensee_id, code, source, fingerprint, expires_at, created_at）。
2. **API Migration**：
   - 现有 `/api/v1/license/licenses` 列表接口在 `status` 参数上保持 `issued/revoked` 兼容，新增 `lifecycleStatus` 参数用于细粒度筛选。
   - 新增 `/api/v1/license/licenses/:id/renew`、`/upgrade`、`/suspend`、`/reactivate` 端点。
3. **Frontend Migration**：
   - 逐步替换 IssueLicenseSheet 中的 Registration Code 输入框为 Select + 生成按钮组合。
   - License 列表 Status Badge 颜色映射更新。
4. **Rollback**：
   - 若新状态机引发严重问题，可在 API 层把 `lifecycle_status` 映射回 `issued/revoked` 的二元判断，前端切回旧分支即可。

## Open Questions

- 是否需要为 License Upgrade 引入"差价计算"或"试用期"逻辑？（当前仅做记录关联，不涉及计费）
- `pending` 状态是否需要自动超时（如 7 天后自动 revoke）？
- 客户端预注册的 API 是否需要独立的鉴权机制（如只允许持有特定 token 的客户端调用）？
