## Context

ITSM 经典引擎已实现 10 种节点类型的图遍历执行。当前自动节点（exclusive/action/notify/parallel/inclusive）执行后立即递归推进到下一节点，不创建 pending Activity。Script Task 是新的自动节点类型，在流程中内联执行变量赋值计算，无需外部 HTTP 调用。

现有基础设施：
- `processVariableModel` + `writeFormBindings()` 支持流程变量的 upsert（按 ticket_id + scope_id + key 唯一约束）
- `buildEvalContext()` 从 process variables 构建评估上下文，支持 `var.*` / `form.*` / `ticket.*` 前缀
- `condition.go` 中的 `evaluateCondition()` 已实现基于字段的简单条件判断（equals/gt/lt 等）

Script Task 需要更强的**表达式能力**——三元运算、字符串拼接、算术运算——这超出了现有 evaluateCondition 的范围。

## Goals / Non-Goals

**Goals:**
- 实现 script 节点的 handleScript handler，作为自动节点内联执行
- 引入 `expr-lang/expr` 表达式引擎，安全评估变量赋值表达式
- 复用现有 processVariableModel 写入流程变量
- 确保表达式执行的安全性（无函数调用、无 IO、无副作用）

**Non-Goals:**
- 不实现完整脚本语言（无循环、无自定义函数定义）
- 不实现 script 节点的前端 UI（⑥ itsm-bpmn-designer 中实现）
- 不修改现有的 evaluateCondition 逻辑（script 使用独立的表达式引擎）

## Decisions

### D1: 表达式引擎选型 — expr-lang/expr
**选择**: 使用 `github.com/expr-lang/expr` 作为表达式引擎

**理由**:
- 纯 Go 实现，无 CGO 依赖，与项目 CGO_ENABLED=0 策略一致
- 内置安全沙箱：默认禁用函数调用，可精确控制可用操作符
- 支持所需的全部操作：算术、比较、字符串拼接、三元运算符、字段访问
- 编译+求值两阶段模型，可提前捕获语法错误
- 社区活跃，广泛用于工作流/规则引擎场景

**备选**:
- `antonmedv/expr`（旧名，已迁移为 expr-lang/expr）
- `Knetic/govaluate` — 更轻量但缺乏三元运算符支持
- 内置 evaluateCondition 扩展 — 不够灵活，会导致 condition.go 膨胀
- `dop251/goja`（JS 引擎）— 过于强大，难以安全限制

### D2: Script 节点执行模式 — 同步自动节点
**选择**: Script 是自动节点（IsAutoNode=true），handleScript 执行后立即调用 processNode 递归推进

**理由**: 表达式求值是纯 CPU 操作（无 IO），微秒级完成，无需异步。与 exclusive/notify 一致的模式。

### D3: 变量写入 — 复用 processVariableModel upsert
**选择**: handleScript 遍历 assignments，对每个赋值调用 expr 求值，然后通过 GORM upsert 写入 itsm_process_variables 表

**理由**: 完全复用已有的 processVariableModel 和 upsert 逻辑（`clause.OnConflict`），保持变量存储的一致性。expr 求值结果会自动推断 value_type。

### D4: 表达式环境 — 从 process variables 构建
**选择**: 表达式执行时从 process variables 构建 env map，key 直接为变量名（无前缀）。在表达式中通过 `priority` 而非 `var.priority` 访问。

**理由**: 表达式上下文是 script 专属的，不需要 `var.` 前缀（与 gateway condition 的 field path 不同）。更简洁直观，如 `sla_level = priority == "P0" ? 1 : 3`。同时注入 `ticket_priority_id`、`ticket_status` 等只读 ticket 字段。

### D5: 文件结构 — 新增 engine/expr.go
**选择**: 新建 `engine/expr.go` 封装 expr-lang/expr 的编译和执行逻辑

**理由**:
- 与 condition.go（现有条件评估器）职责分离
- 将 expr 依赖封装在一个文件中，方便测试和替换
- 提供 `evaluateAssignment(expression string, env map[string]any) (any, error)` 高层接口

## Risks / Trade-offs

**[表达式注入]** → 使用 `expr.Env()` 选项限制可用变量，`expr.DisableAllBuiltins()` 禁用内置函数，仅允许算术/比较/字符串操作符。用户无法调用 Go 函数或访问未注入的变量。

**[执行超时]** → expr-lang/expr 不提供超时机制，但表达式是纯计算（无 IO/循环），实际执行时间为微秒级。当前不添加超时控制，如未来需要可在 expr.go 中添加 context 超时。

**[value_type 推断]** → expr 返回值为 `any`，需要根据 Go 类型推断 value_type（number/string/boolean）。推断逻辑在 expr.go 中实现，边界情况（如 NaN、nil）回退为 string 类型。
