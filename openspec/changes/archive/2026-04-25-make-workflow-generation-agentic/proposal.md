## Why

当前参考路径生成把 workflow_json 的运行期完整性校验前置成硬失败：只要 LLM 草图仍有 blocking validation error，`POST /api/v1/itsm/workflows/generate` 就返回 400，用户看不到图，也无法基于图继续与 Agent 达成共识。这让“参考路径生成”退化成传统确定性校验器，违背智能工单的 Agentic 协作体验。

同时，现有死端检测只从第一个 end 节点反向遍历，会把合法的多终点分支误判为无法到达终点，进一步放大生成阶段的失败率。

## What Changes

- 参考路径生成 API 在成功拿到可解析 workflow_json 后 SHALL 返回 200，并附带 validation errors、保存状态和健康检查结果；blocking validation errors 不再让生成请求整体 400。
- 生成阶段允许保存带 blocking issues 的参考路径草图，但服务发布健康检查 SHALL 标记对应阻塞项，确保运行/发布风险仍然可见。
- LLM 上游失败、协作规范为空、引擎未配置、无法提取有效 JSON 仍然按错误返回；本次只改变“已有 workflow_json 草图但校验未完全通过”的体验。
- 工作流拓扑死端检测 SHALL 支持多个 end 节点，从所有 end 节点反向遍历，避免把独立结束分支误判为死端。
- 服务定义页面点击生成参考路径后 SHALL 展示生成结果；若存在 blocking/warning issues，前端 SHALL 用非阻断方式提示并切到工作流图/刷新服务数据。

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `itsm-workflow-generate`: 生成 API 的校验失败语义从“blocking 直接 400”调整为“返回草图 + issues + 健康状态”；拓扑校验支持多终点。
- `itsm-service-definition-ui`: 生成参考路径后，无论是否存在 validation issues，都应展示可视化草图并以非阻断方式提示用户。

## Impact

- 后端：`internal/app/itsm/definition/workflow_generate_service.go`、`internal/app/itsm/definition/workflow_generate_handler.go`、`internal/app/itsm/engine/validator.go`、相关 Go tests。
- 前端：`web/src/apps/itsm/pages/services/[id]/index.tsx`、`web/src/apps/itsm/api.ts`、相关 i18n 文案。
- API：`POST /api/v1/itsm/workflows/generate` 对“JSON 可解析但 blocking 校验失败”的响应状态从 400 改为 200，响应体保留 errors/saved/service/healthCheck 信息。
- 数据库：无 schema 变更。
