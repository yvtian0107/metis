# Capability: itsm-decision-quality-observability

## Purpose

ITSM 智能引擎决策质量观测能力，提供核心指标聚合与运营看板。

## Requirements

### Requirement: 决策质量核心指标
系统 SHALL 提供决策质量核心指标：`approval_rate`、`rejection_rate`、`retry_rate`、`avg_decision_latency`、`recovery_success_rate`，并支持按服务与部门维度聚合。

#### Scenario: 查看服务维度质量指标
- **WHEN** 运营人员按服务查看决策质量
- **THEN** 系统返回该服务的核心指标聚合结果

#### Scenario: 查看部门维度质量指标
- **WHEN** 运营人员按部门筛选
- **THEN** 系统返回所选部门的核心指标聚合结果

### Requirement: 指标口径一致性
指标定义与计算窗口 SHALL 在 capability 中固定，变更必须通过新变更发布，不得在运行时隐式调整。

#### Scenario: 指标版本变更受控
- **WHEN** 团队需要调整某指标口径
- **THEN** 必须通过 OpenSpec 变更流程更新规范并发布
