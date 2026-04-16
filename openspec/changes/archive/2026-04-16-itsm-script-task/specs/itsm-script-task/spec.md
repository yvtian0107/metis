## ADDED Requirements

### Requirement: Script 节点执行
script 节点 SHALL 作为自动节点（IsAutoNode=true）执行变量赋值操作。节点 `data.assignments` 定义赋值列表，每个赋值包含 `variable`（目标变量名）和 `expression`（expr-lang/expr 表达式）。handleScript SHALL 遍历赋值列表，对每个表达式求值后将结果写入 itsm_process_variables 表（scope_id 来自当前 token）。执行完成后 SHALL 立即递归推进到下一节点。

#### Scenario: 简单变量赋值
- **WHEN** 流程到达 script 节点，assignments 包含 `{variable: "sla_level", expression: "3"}`
- **THEN** 系统将流程变量 `sla_level` 设为 `3`（value_type="number"），然后自动推进到下一节点

#### Scenario: 基于现有变量的条件赋值
- **WHEN** script 节点 expression 为 `priority == "P0" ? 1 : impact == "high" ? 2 : 3`
- **AND** 流程变量 priority="P0"
- **THEN** 求值结果为 1，写入目标变量

#### Scenario: 字符串拼接表达式
- **WHEN** script 节点 expression 为 `"SLA 升级: " + ticket_code`
- **AND** 流程变量 ticket_code="INC-2024-001"
- **THEN** 求值结果为 "SLA 升级: INC-2024-001"，写入目标变量（value_type="string"）

#### Scenario: 多个赋值按顺序执行
- **WHEN** script 节点 assignments 包含 `[{variable: "a", expression: "1"}, {variable: "b", expression: "a + 1"}]`
- **THEN** 系统先求值并写入 a=1，再求值 b=a+1=2（后续表达式可引用前面已赋值的变量）

#### Scenario: 表达式求值失败
- **WHEN** script 节点的某个 expression 语法错误或引用不存在的变量
- **THEN** 系统记录错误到 Timeline（warning 级别），跳过该赋值，继续处理后续赋值，流程不中断

#### Scenario: assignments 为空
- **WHEN** script 节点的 assignments 列表为空或未配置
- **THEN** 系统不执行任何赋值，直接推进到下一节点（相当于 pass-through）

---

### Requirement: expr-lang/expr 表达式引擎安全约束
系统 SHALL 使用 `expr-lang/expr` 库作为 script 节点的表达式求值引擎。表达式环境 SHALL 仅注入当前 scope 的流程变量值和只读 ticket 字段。系统 SHALL 禁用所有内置函数，仅允许算术运算（+、-、*、/、%）、比较运算（==、!=、>、<、>=、<=）、逻辑运算（&&、||、!）、字符串拼接（+）和三元运算符（? :）。

#### Scenario: 允许的操作符
- **WHEN** 表达式为 `amount > 10000 ? 3 : 1`
- **THEN** 表达式正常求值

#### Scenario: 禁止函数调用
- **WHEN** 表达式中包含函数调用如 `len(name)` 或 `print("hello")`
- **THEN** 表达式编译失败，记录错误

#### Scenario: 未注入变量访问
- **WHEN** 表达式引用了未在环境中注入的变量名
- **THEN** 表达式编译阶段报错（expr-lang/expr 的类型检查）

---

### Requirement: 变量赋值 value_type 推断
handleScript 写入流程变量时 SHALL 根据 expr 求值结果的 Go 类型自动推断 value_type：`int`/`float64` → "number"，`bool` → "boolean"，`string` → "string"，其他类型 → JSON 序列化后 value_type="json"。

#### Scenario: 数值结果
- **WHEN** 表达式求值结果为 Go float64 类型
- **THEN** 变量写入 value_type="number"

#### Scenario: 布尔结果
- **WHEN** 表达式求值结果为 Go bool 类型
- **THEN** 变量写入 value_type="boolean"

#### Scenario: 字符串结果
- **WHEN** 表达式求值结果为 Go string 类型
- **THEN** 变量写入 value_type="string"
