## Why

引擎配置页面当前管理两个 internal agent（`itsm.generator` 和 `itsm.runtime`），但实际执行决策的是 `tools/provider.go` seed 的完整 assistant agent（"流程决策智能体"、"IT 服务台智能体"）。"决策引擎"这个名称也不准确——它本质上是智能体而非引擎。同时服务台智能体的 LLM 配置缺少统一管理入口，只能去 AI 模块的智能体管理页面修改。

## What Changes

- **移除 `itsm.runtime` internal agent**：不再 seed 这个纯 LLM 的 internal agent，决策能力由完整的"流程决策智能体"承担
- **给 preset agent 加 code**：`tools/provider.go` 的两个 preset agent 增加 `code` 字段（`itsm.servicedesk`、`itsm.decision`），使引擎配置服务可以通过 code 查找和管理它们
- **引擎配置 API 结构调整**：`runtime` 区块改为 `decision`（对应 `itsm.decision`），新增 `servicedesk` 区块（对应 `itsm.servicedesk`），`decisionMode` 移入 `decision` 区块
- **前端引擎配置页面调整**："决策引擎"改名为"决策智能体"，新增"服务台智能体"配置卡片
- **SmartEngine 对接调整**：从读取 `itsm.runtime` internal agent 改为读取 `itsm.decision` 完整 agent 的 LLM 配置发起 ReAct 执行

## Capabilities

### New Capabilities

_(无新增能力，均为已有能力的修正)_

### Modified Capabilities

- `itsm-engine-config`: API 结构从 `generator + runtime + general` 改为 `generator + servicedesk + decision + general`；seed 移除 `itsm.runtime`，preset agent 增加 code 字段；前端改名 + 新增卡片
- `itsm-smart-engine`: SmartEngine 的 AgentProvider 从读 `itsm.runtime` internal agent 改为读 `itsm.decision` 完整 agent

## Impact

- **后端**：`engine_config_service.go`、`engine_config_handler.go`、`seed.go`、`tools/provider.go`、`engine/smart.go`
- **前端**：`pages/engine-config/index.tsx`、`api.ts`、`locales/zh-CN.json`、`locales/en.json`
- **API**：`GET/PUT /api/v1/itsm/engine/config` 响应/请求结构变更（**BREAKING**：`runtime` → `decision`，新增 `servicedesk`）
- **数据库**：`ai_agents` 表中 preset agent 记录新增 `code` 字段值；`itsm.runtime` internal agent 不再 seed（已有数据由用户手动清理）
