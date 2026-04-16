## Tasks

- [x] 1. NodeData 扩展：在 `workflow.go` 的 NodeData struct 中新增 `Assignments []Assignment` 字段，定义 Assignment struct（Variable string + Expression string）
- [x] 2. 引擎常量更新：从 `engine.go` 的 UnimplementedNodeTypes 中移除 NodeScript
- [x] 3. IsAutoNode 更新：在 `engine.go` 的 IsAutoNode 函数中加入 NodeScript
- [x] 4. 添加 expr-lang/expr 依赖：`go get github.com/expr-lang/expr`
- [x] 5. 新建 `engine/expr.go`：封装 evaluateExpression 函数，接受 expression string + env map[string]any，返回 (any, error)。使用 expr.Env() 限制环境，禁用内置函数
- [x] 6. expr.go 补充：实现 inferValueType(val any) string 函数，根据 Go 类型推断 value_type（number/boolean/string/json）
- [x] 7. expr.go 补充：实现 buildScriptEnv 函数，从 process variables 构建表达式环境 map（变量名为 key，反序列化值为 value），同时注入 ticket 只读字段
- [x] 8. 实现 handleScript：在 `classic.go` 中新增 handleScript 方法，遍历 assignments，调用 evaluateExpression 求值，写入 process variable（复用 processVariableModel upsert），记录 timeline，然后递归推进到下一节点
- [x] 9. processNode 路由：在 `classic.go` 的 processNode switch 中新增 `case NodeScript: return e.handleScript(...)` 分支
- [x] 10. Validator 增强：在 `validator.go` 中新增 script 节点校验规则——script 节点必须有且仅有一条出边
- [x] 11. 编译验证：运行 `go build -tags dev ./cmd/server/` 确认无编译错误
