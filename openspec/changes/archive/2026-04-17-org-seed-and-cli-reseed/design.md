## Context

Metis 的 `app.App` 接口定义了 `Seed(db *gorm.DB, enforcer *casbin.Enforcer) error`，在 Install wizard 的 hotSwitch 阶段和每次正常启动时都会调用。当前没有办法区分首次安装和日常同步——所有 App 的 Seed 每次都执行相同逻辑。

Org App 的 seed 只创建菜单和 Casbin 策略（适合每次同步），但缺少内置的部门和岗位数据。bklite-cloud 项目已经验证了一套合理的初始数据（7 部门 + 7 岗位），适合移植。

开发阶段频繁需要重置/补充 seed 数据，但只能删库重装，没有轻量级的 CLI 手段。

## Goals / Non-Goals

**Goals:**
- App.Seed 接口支持区分首次安装 vs 日常同步
- Org App 在首次安装时创建内置部门和岗位
- 提供 `./server seed` CLI 命令手动触发完整 seed
- 所有现有 App 的 Seed 签名同步更新

**Non-Goals:**
- 不做 admin 用户与部门/岗位的自动绑定
- 不创建 assistant 等额外用户
- 不引入 Cobra 或其他 CLI 框架
- 不改变现有 seed.Install() / seed.Sync() 的 kernel 层逻辑

## Decisions

### D1: App.Seed 增加 `install bool` 参数

**选择**: `Seed(db *gorm.DB, enforcer *casbin.Enforcer, install bool) error`

**备选方案**:
- 拆成 `Install()` + `Sync()` 两个方法：语义更清晰，但多数 App 两个方法内容一样，增加维护负担
- App 内部自行调用 `seed.IsInstalled(db)` 判断：不可行，Install wizard 在 `App.Seed()` 执行前已调用 `SetInstalled()`

**理由**: 一个参数变更影响最小，语义直接，调用方完全掌控。所有 7 个 App 只需改签名，不关心 install 的 App 直接忽略该参数。

### D2: CLI 用 os.Args 子命令，不引入 Cobra

**选择**: `main()` 检查 `os.Args[1] == "seed"`，走独立分支

**理由**: 当前只需一个子命令，flag 包足够。Cobra 对单命令场景过重。将来需要更多子命令时再引入。

**CLI 行为**:
```
./server              → 正常启动（现有逻辑不变）
./server seed         → 加载 config → 连 DB → kernel Sync + App.Seed(install=true) → 退出
```

seed 子命令复用 `-config` flag 以支持自定义配置路径。

### D3: 部门/岗位 seed 用 FirstOrCreate + code 匹配

**选择**: `db.Where("code = ?", x).FirstOrCreate(&record)` 模式

**理由**: 与现有菜单 seed 的幂等模式一致（按 permission 查找）。code 字段有唯一索引，天然适合做幂等键。重复运行安全，已存在的记录不会被覆盖。

### D4: 部门层级用两阶段创建

先创建根部门（总部），拿到 ID 后再创建子部门并设置 ParentID。这与 bklite-cloud 的 initializer 模式一致。

## Risks / Trade-offs

- **接口变更波及所有 App** → 影响可控，7 个 App 都是内部代码，机械修改签名即可。无外部消费者。
- **CLI seed 重跑可能与用户修改冲突** → FirstOrCreate 只在记录不存在时创建，不覆盖已有数据。用户改了 code 的话不会冲突，删了的话会重建。这是期望行为。
- **子命令解析简陋** → 只支持一个子命令，不支持 `--help`。可接受，后续需要时引入 Cobra。
