# itsm-form-definition

## Purpose

~~独立的表单定义实体管理。~~ **已废弃** — 表单 schema 已内嵌到 ServiceDefinition.IntakeFormSchema 和 WorkflowNode.FormSchema 中，不再需要独立的 FormDefinition 实体。

## Requirements

*All requirements removed by change `itsm-inline-form-schema`.*

*原有需求（FormDefinition model、Form CRUD API、Schema validation on save、Form definition audit trail、IOC registration）已全部移除。表单管理在服务定义 UI 和工作流编辑器中内联完成。*
