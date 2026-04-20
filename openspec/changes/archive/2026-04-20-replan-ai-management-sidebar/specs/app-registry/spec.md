## MODIFIED Requirements

### Requirement: 前端模块注册机制
前端 SHALL 在 `web/src/apps/registry.ts` 中提供能够注册路由与导航元数据的 `AppModule` 类型以及 `registerApp()`、`getAppRoutes()` 等函数。每个 App 模块 SHALL 在 `web/src/apps/<name>/module.ts` 中调用 `registerApp()` 注册自己的路由定义，并且可以声明该 App 的导航分组与分组下的资源项。

#### Scenario: App 模块注册路由
- **WHEN** 一个 App 模块的 `module.ts` 被导入
- **THEN** 该模块 SHALL 调用 `registerApp()` 注册自己的路由定义（path + lazy component）

#### Scenario: App 模块注册分组导航
- **WHEN** 一个 App 模块声明二级分组和三级资源菜单项
- **THEN** 注册表 SHALL 保存这些分组导航元数据，供 sidebar 渲染和激活态计算使用

#### Scenario: App 模块仅注册简单导航
- **WHEN** 一个 App 模块只声明现有的简单导航信息而不提供分组
- **THEN** 注册表 SHALL 继续接受该模块定义，不要求其升级到分组导航格式

#### Scenario: AI 模块声明工具域三级菜单
- **WHEN** AI 模块注册导航元数据
- **THEN** 其 `工具` 分组 SHALL 能够声明 `内建工具`、`MCP 服务`、`技能包` 三个独立三级菜单项

### Requirement: 获取所有 App 路由
前端路由装配层 SHALL 继续从 app registry 中获取所有已注册模块的路由定义，无论该模块是否声明分组导航。

#### Scenario: 获取所有 App 路由
- **WHEN** 应用入口调用 `getAppRoutes()`
- **THEN** SHALL 返回所有已注册模块的路由定义扁平数组

#### Scenario: 分组导航不影响路由装配
- **WHEN** 一个 App 模块新增分组导航元数据
- **THEN** 路由装配结果 SHALL 仍只由其注册的 route objects 决定，而不会因为导航分组而丢失页面路由
