# itsm-workflow-editor

## Purpose

ITSM 工作流编辑器核心规范，定义编辑器的三栏布局、节点面板、工具栏和节点类型扩展。

## Requirements

### Requirement: 工作流编辑器三栏布局
系统 SHALL 将工作流编辑器组织为三栏布局：左侧节点面板（支持所有节点类型）、中间 ReactFlow 画布、右侧属性面板（根据选中元素类型渲染对应子面板）。

#### Scenario: 拖拽新节点到画布
- **WHEN** 用户从左侧面板拖拽一个节点类型到画布
- **THEN** 在拖拽释放位置创建对应类型的新节点，节点使用 BPMN 风格渲染

#### Scenario: 选中节点打开属性面板
- **WHEN** 用户单击画布中的节点
- **THEN** 右侧属性面板显示该节点类型对应的完整配置界面（标签、参与人、表单绑定、变量映射等）

#### Scenario: 选中边打开属性面板
- **WHEN** 用户单击画布中的连线
- **THEN** 右侧属性面板显示边配置界面（outcome、默认边开关、条件构建器）

#### Scenario: 点击画布空白关闭面板
- **WHEN** 用户单击画布空白处
- **THEN** 右侧属性面板关闭

### Requirement: 节点面板支持所有节点类型
系统 SHALL 在左侧节点面板中列出所有可用的节点类型，包括新增的 timer、signal、parallel、inclusive、subprocess、script。

#### Scenario: 节点面板分组显示
- **WHEN** 编辑器加载完成
- **THEN** 左侧面板按类别分组显示节点：事件（start, end, timer, signal）、任务（form, approve, process, action, script, notify）、网关（exclusive, parallel, inclusive）、其他（subprocess, wait）

### Requirement: 工具栏操作按钮
系统 SHALL 在画布顶部右侧显示工具栏，包含保存、自动布局、撤销、重做按钮。

#### Scenario: 工具栏显示
- **WHEN** 编辑器加载完成
- **THEN** 顶部右侧工具栏显示：Undo（灰色如果无历史）、Redo（灰色如果无 future）、自动布局、保存按钮

### Requirement: 节点类型扩展
系统 SHALL 在 types.ts 中扩展 NODE_TYPES 数组，新增 timer、signal、parallel、inclusive、subprocess、script 六种类型，保留原有 9 种不变。系统 SHALL 在 types.ts 中将 form/user_task 节点的 `formDefinitionId` 字段替换为 `formSchema` (FormSchema 对象)。WFNodeData 接口 SHALL 移除 `formDefinitionId?: string`，新增 `formSchema?: FormSchema`。WFNodeData 接口扩展支持 inputMapping、outputMapping、scriptAssignments、subprocessJson 等新字段。

#### Scenario: 类型定义完整性
- **WHEN** 编辑器使用 NodeType 联合类型
- **THEN** 包含全部 15 种类型：start, end, form, approve, process, action, exclusive, notify, wait, timer, signal, parallel, inclusive, subprocess, script

#### Scenario: 新字段向后兼容
- **WHEN** 加载旧格式的 workflowJson（不含新字段）
- **THEN** 新字段默认为 undefined，编辑器正常工作

#### Scenario: form 节点属性面板内嵌表单设计器
- **WHEN** 用户选中一个 form 或 user_task 节点
- **THEN** 右侧属性面板 SHALL 显示内嵌的 FormDesigner 组件，允许直接编辑 formSchema

#### Scenario: 保存时 formSchema 嵌入 workflowJson
- **WHEN** 用户保存工作流
- **THEN** 每个 form/user_task 节点的 data.formSchema SHALL 包含完整的表单 schema JSON
