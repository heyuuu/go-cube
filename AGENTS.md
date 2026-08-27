# AGENTS.md

面向未来 ZCode agent 的项目工作规则。先读此文件，再动手改 cube。

> 项目采用 SDD（Spec-Driven Development）管理演进，**当前现状见 [`docs/spec/现状.md`](./docs/spec/现状.md)**（定位/架构/命令/API/数据/配置）。改动功能或架构前，先读 现状.md 对应段落。本文件与 现状.md 冲突时，**现状.md 是事实基准**（它描述代码「是什么」），本文件侧重「怎么改」。未来需求提案在 [`docs/proposals/`](./docs/proposals/)。

## 项目简介

**cube** —— 面向个人开发者的本地多项目管理工具（CLI 优先 + 本地 Web）。Go 1.26 编写，module path `cube`（go.mod 第一行）。

- 历史有三代：v1 (php)、v2 (go)、**v3 (当前，按领域重构)**。
- 出口：CLI（人用 / alfred）、本地 Web HTTP server（`cube server`，huma + 标准 ServeMux）。MCP 出口为后续规划。
- 定位原则：不做云服务、不绑 AI（cube 可被 AI 编排，但自身不集成 AI）。
- Go 源码根在 `server/`（不是仓库根）；`make build` / `make install` 都 `cd server` 再执行。

## 分层架构（改代码必须遵守的依赖纪律）

```
基础设施  config / db / logger / version / runtime               所有层共享
能力      opener / util(git / fuzzy / easycache / pathkit / slicekit / tui)  通用动作, 不含业务实体
领域      project (含 gitcache / scan / clone) / history          业务 domain, 含实体和规则
出口      cmd / handlers / web                                     把领域包成 CLI/Web（handlers=业务 HTTP handler 层，web=服务端框架）
装配      app / main                                              接线
```

- 基础设施不依赖上层；能力层只依赖基础设施；领域层依赖能力+基础设施；`handlers` 依赖领域层与 `web` 框架；**cmd 与 web 不互调**；`app` 是唯一接线点。
- **App 装配是显式构造，不是懒初始化**：`app.New(cfg)` 一次性开 db + AutoMigrate + 构造各 service + 组装 web server（`server/app/app.go`）。`cmd.Execute()`（`server/cmd/root.go`）在 main 里调用，把 `*app.App` 显式传给所有命令工厂（`newXxxCmd(a *app.App)`）。**无全局单例、无 `app.Default()`、无包级 `init()` 反向依赖**。
- 加一个新 domain = ①领域包 ②`cmd/<x>` 子命令组 ③`handlers/` 加 `<domain>_handler.go`（实现 `Handler.Register`，注册走 `web.ApiGet`/`web.ApiPost`） ④config 加节 ⑤`app/app.go` 装配清单加构造。**五处都是加法，不碰现有 domain**。

## 关键机制（改动前先理解）

- **项目前提：所有项目都是 git 项目**。`.git` 存在是扫描判定项目的必要条件（详见 `project/scan.go`）。因此 `tags` 不打冗余的 `git` 标签，只标额外特征（`worktree` / `godot`）。改扫描/tag 逻辑时遵守此假设。
- **全量 project 列表永不落盘**：project 一切由 scan-rule 推导，持久化只有 cache（git.json）。因此**禁止在 settings.json / config 等全局配置里按项目路径记录具体 project 的内容**——项目目录会重命名/移动，按路径 keyed 的配置会失联留脏数据。需要「project 自身的配置」（如 monorepo workspace 声明，方向定为项目根 `.cube/cube.json`）时，放项目内跟仓库走，不放全局配置。
- **gitcache 异步采集**：`project list --status` 等读命令从 `~/.config/cube/cache/git.json` 读 git 状态快照（几乎零开销）；**单写者模型**——常驻 server 是 git.json 的唯一写方（后台异步采集回写），CLI 只读不写，落盘靠原子写（tmp + rename），无跨进程锁。**读路径不得阻塞采集——只能读快照**。详见 [`docs/spec/现状.md`](./docs/spec/现状.md)「三、关键机制」。
- **opener：接口 + 唯一 exec 实现 + settings.json**：`Opener` 是接口（`opener/opener.go`），唯一实现 `execOpener`（`opener/exec.go`，cmd 模板 `$0/$1` 占位）——「打开工作台页」不设独立形态，配 exec cmd `["cube","ui","workbench","$0"]` 组合 cube 自身 CLI；能力由 `roles []Role` 声明（见 `opener/role.go`），`slotCount` 由 role 推导；`Open(role, slotArgs...)` 的 role 校验收敛在实现内；经 `Executor` 执行（`opener/executor.go`，测试注入 fake）。**opener 数据存 settings.json 的 openers 节**（`settings` 包节级 API，Service 直读不缓存、写侧领域校验；详见 现状.md 3.3）——改 opener 时同步看 `opener/opener.go`、`opener/exec.go`、`opener/role.go`、`opener/executor.go`、`settings/settings.go`。
- **全局 flag 预解析**：`-c`（配置目录）/ `-d`（debug）用 Go 原生 `flag` 包在 cobra 初始化**之前**预解析（`cmd/root.go` 的 `extractGlobalFlags`），保证 logger 和 config 先就绪。cobra 上的 `--config`/`--debug` 仅用于 help 提示。新增需在 logger/config 之前生效的全局 flag，走 `extractGlobalFlags` 而非 cobra。
- **Web 出口分两层**：`web` 包是服务端框架（Server 装配 / envelope / 静态资源 / system 端点，`web.NewServer(handlers ...Handler)` 自动追加内置 system 与 static handler）；业务 handler 在 `handlers` 包（`<domain>_handler.go` 同包分文件，不按 domain 分子包），实现 `Handler.Register(api huma.API, mux *http.ServeMux)`——注册统一走 `web.ApiGet` / `web.ApiPost`，WebSocket 等原生路由直接挂 mux（不经 huma）。统一 `ApiOutput{ok,message,data}` envelope（泛型 `ApiOutput[T]`，见 `web/api.go`）；路径强制 `/api/` 前缀，由 `apiRegister` 解析 group tag + operationId。响应 JSON 经 `nilSliceJSONFormat`（`web/jsonfmt.go`）把 nil 切片序列化为 `[]`——新增 handler 自动复用，不要在 handler 里手写 `make([]T, 0)` 兜底。
- **配置与双环境**：默认目录按环境分流——dev（源码直跑 / air / run.sh）→ `~/.config/cube-dev/`，prod（`make build` / `make install`，ldflags 注入了正式 version）→ `~/.config/cube/`；身份判定见 `version.IsDev()`，详见 [`docs/proposals/archived/1026-环境分离/`](./docs/proposals/archived/1026-环境分离/)。`config.json` 按 domain 分节，`server.port` 是端口唯一事实源（无 `-p` flag、不支持多实例）。`-c` 覆盖配置文件路径，`-d` 开 debug（只影响 logger 初始化）。配置解析失败/缺失不阻断启动（降级优先，见 `opener.NewService` 跳过坏配置）。**无热 reload**（已移除，转向命令式改 config）。配置目录下的运行期状态（sqlite `data.db`、`cache/git.json`、`app.log`）由 `app.Paths`（`server/app/paths.go`）统一计算，不要在调用方硬拼路径。应用标识（`AppName`/`AppTitle`）收敛在 `version/name.go`：whoami 身份 / 进程探测 / shutdown token / 默认配置目录路径均由 `version.AppName` 派生；日志文件名是通用的 `app.log`，不含应用名。

## 常用命令

构建 / 安装（Makefile 在**仓库根**，已注入 version ldflags；go 源码在 `server/`）：

```bash
make build        # 先 pnpm -C web build 并拷 web/dist 到 server/web/ui (go:embed)，再 cd server && go build 到 tmp/cube
make install      # cd server && go install + 安装 zsh completion
make tag          # 当前位置打递增版本 tag（末位 +1）
```

Web 开发热重载（`server/.air.toml`，已内置 `goimports -w .` + `go vet ./...` 预检）：

```bash
cd server && air               # 需安装 air；args_bin = ["--debug", "server"]（端口读 dev config 的 6001）
./run.sh [args]                # 手动：goimports -> go vet -> go build -> 运行（在 server/ 下）
```

测试（标准 go test，在 `server/` 下执行；无额外 harness）：

```bash
cd server && go test ./...
cd server && go test ./opener/...     # 聚焦某个包
```

### 测试辅助包 `internal/testfixture`

cube 的 IO 测试（git 操作、文件扫描、缓存读写）通过 `server/internal/testfixture` 包提供统一 fixture builder：

- **临时目录落 `runtime/test/`**（已 gitignore），**不用系统 `/tmp`**——失败时方便翻看现场排查。每个测试拿到独立子目录（`runtime/test/<时间戳>-<test名>/`），不自动清理。
- **`Workspace`**：测试工作区。`ws := testfixture.NewWorkspace(t)` → `ws.Dir` 是该测试专属目录；`ws.Mkdir/WriteFile/Join` 在其下操作。
- **`BuildGitRepo(t, dir, GitRepoSpec)` / `ws.MakeGitRepo(name)`**：建真实 git 仓库（用系统 git + 注入 user 配置，不依赖全局 git config）。`GitRepoSpec` 声明预期状态（分支/commit 数/tag/remote/ahead/dirty）。
- **`ws.MakeProjectDir(relPath, opts...)`**：建「会被 cube 扫描识别为 project」的目录（默认含 git 仓库）。opts：`WithGodot()`/`WithWorktree()`/`WithDirty()`/`WithoutGit()`。

```go
ws := testfixture.NewWorkspace(t)
repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{
    Branch: "develop", RemoteUrl: "/tmp/remote.git", MakeDirty: true,
})
// 或工程目录
ws.MakeProjectDir("scanroot/g1/proj", testfixture.WithGodot())
```

**为什么 testfixture 不 import `project` 包**：底层包（`git`/`gitcache`）的测试要用 testfixture，而 `project → gitcache → git` 是依赖链。若 testfixture 反向依赖 project 会形成循环。所以「构造 project.Service」这种依赖 `project` 包的逻辑写在调用方测试里（见 `project/scan_test.go` 的 `newServiceAt`），不沉淀进 testfixture。

### 测试策略（什么测、什么不测）

- **纯函数**（解析、计算、字符串处理）：普通表驱动测试。`fuzzy`/`pathkit`/`git/url`/`git 读输出解析`/`slicekit`/`easycache`/`opener 解析`。
- **依赖外部进程/库的 IO**（git 二进制读/写仓库）：**用 testfixture 建真实临时仓库测**，不 mock。`git` 的 `Refs/Remotes/IsDirty/LoadRepoStatus`、`git.FindGitRoot`、`gitcache.Load/Save/Refresh/collectEntry`。
- **依赖 sqlite**：用 `:memory:` 内存库 + 直接 AutoMigrate。`history` 全部测试。
- **依赖真实目录扫描**：用 testfixture 建工程目录树，构造 `config.ProjectConfig` 喂给 `project.NewService`（绕开 config/app 单例）。`project/scan_test.go`。
- **opener 执行类**：通过 `Executor` 接口注入 fake，不真的启动编辑器。见 `opener/opener_test.go`。
- **web 层**：httptest 拉起真实 `Server.Handler()` 打真实 HTTP 请求（见 `web/server_test.go` 的 newTestEnv 基建），断言路由 / DTO / envelope / nil 序列化 / 静态资源契约。新增 handler 时在此模式上补用例。
- **不写单测的（靠手动/集成验证）**：
  - `git.Run`/`git.Clone`/`git.Push`（透传 stdio 到 `os.Stdout`，无法捕获输出；且本质是组装 git 参数）
  - `opener.Open` 的真实进程启动（已用 Executor 隔离，但默认实现的真启动仍靠手动验证）
  - `config`/`db`（全局单例无 setter，测试无法隔离）
  - `cmd/*`（cobra 命令编排）

## 必须遵守的编码规则

1. **Go 代码修改后必须先格式化与校验再提交/收工**——这是强制规则：
   ```bash
   goimports -w .     # 格式化 + 整理 import（本地已安装：~/go/bin/goimports）
   go vet ./...       # 静态校验
   ```
   每次改完 `.go` 文件都要跑，不要跳过。`air` 与 `run.sh` 也已内置这两步，保持一致。**命令在 `server/` 目录下执行**（go.mod 在那里）。
2. 遵循 v3 分层依赖纪律（见上），不要让 `cmd` 直接调 `web`、不要让基础设施包 import 领域包。
3. **加新 domain 走"五处加法"流程**，不修改既有 domain 的接线。
4. 日志统一用 `log/slog`（`slog.Debug` / `slog.Info` / ...），不要用 `fmt.Println` 做日志（`fmt` 仅用于面向用户的 CLI 输出）。debug 日志受 `-d` 控制。
5. 错误处理遵循现有风格：可恢复的降级用 `slog` 记录后继续；致命错误用 `fmt.Errorf("...: %w", err)` 包装并返回。`cmd.Execute()` 的 `checkError` 会在退出前把错误打到 `slog` + stdout。
6. **所有 Error 消息一律用中文**（`errors.New` / `fmt.Errorf` 的字符串）。变量名、标识符、以及约定俗成的英文专业名词/技术术语保留英文原词——例如 `repoUrl`、`OpenAPI`、`worktree`、`opener`、`config`、`slot`、`tty` 等。参考既有代码，如 `fmt.Errorf("repoUrl 不是合法地址: url=%s", rawRepoUrl)`、`errors.New("未找到指定app: " + appName)`。
7. **构造函数命名约定**（按返回值形态选前缀）：
   - `NewXxx()` → 返回 `*Xxx`（指针，单返回值）。例：`NewService` / `NewOpenerHandler` / `NewItem`。
   - `MakeXxx()` → 返回 `Xxx`（值类型，单返回值）。
   - `InitXxx()` → 用于构造时需要返回 `error` 等额外值的情况（即 `(*Xxx, error)` 或 `(Xxx, error)`）。
   - 解析/加载类函数不在本约定范围内，保留 `ParseXxx` / `LoadXxx` 等既有命名（如 `ParseRepoUrl`、`gitcache.Load`）。
8. **getter / setter 尽量写成一行**，避免函数体展开过多行影响阅读密度。例：
   ```go
   func (o *Opener) Name() string { return o.name }
   func (s *Service) ScanRules() []ScanRule { return s.scanRules }
   ```
   仅当逻辑较复杂、单行会牺牲可读性时才折行展开。`server/app/app.go` 的 `App` 一组 getter 是参考样板。

   **准确定义**：本规则所说的 getter/setter 仅指——
   - 是 **struct 的方法**（带 receiver），不是包级函数；
   - 方法名形如 `GetXxx()` / `SetXxx(v)`（Go 惯例 getter 不带 Get 前缀，直接用属性名，如 `Name()`）；
   - 方法体是**直接读 / 直接写 struct 的某个属性**（`return o.xxx` 或 `o.xxx = v`）。

   以下**不属于** getter/setter，仍按多行显示：
   - 包级函数（如 `config.Path()`、`config.Default()`、`db.Default()`、`config.SetDebug()`）；
   - 虽名为 `GetXxx`/`SetXxx` 但方法体不是属性的直接读/写——例如 `return s.cache.Get()`、`return strings.Join(o.cmd, " ")`、`Projects()`（委托、计算、聚合等）。

   getter/setter 的书写约定：
   - **建议不加注释**（建议性，非强制）——struct 属性的行尾注释通常已足够说明，方法上再写只会重复。参考 `project.Project`：属性 `path string // 项目路径，唯一标识`，getter `Path()` 不写注释。**但如果注释包含超出属性说明本身的内容**（如跨文件调用指引、设计意图、注意事项等），则应当保留。参考 `gitcache` 系列的 getter：除说明返回值外，还注明其调度用途（如「供 TTL 调度逻辑使用」）。
   - **多个 getter（或多个 setter）连写在一起，不加空行**；顺序与对应属性在 struct 内的声明顺序一致。参考 `project.Project` 的 `Group/Name/Path/Tags/GitInfo`。
   - getter 组与其它方法之间保留一个空行分隔。
9. 写表用 `tui.PrintTable`，交互选择用 `tui.SelectItem`，保持 CLI 输出风格一致。
10. **优先复用 `util/` 下的辅助函数**，能力收敛在各 util 子包内，不在调用方就地重造：
    - 动手前先 grep 对应 util 包；缺什么就**在该 util 包里加新函数**，而不是在 `cmd/` / domain 里实现。例：git 读写统一走 `util/git`（读按主题分文件：`refs.go` 分支/tag/ahead-behind、`remote.go` 远端配置、`status.go` 工作区状态、共享执行核 `run.go`；写：push / clone / commit / `FindGitRoot`，见 `git.go`）；路径处理走 `util/pathkit`；切片运算走 `util/slicekit`。
    - util 包内的函数必须**足够内聚且无副作用**：只依赖入参做纯运算，不读进程状态、不读环境。与环境强相关的副作用（`os.Getwd()` / `os.Getenv()` / 读 `~` / 当前时间等）只允许出现在**职责就是处理环境的 util 包**（如 `pathkit` 展开 `~`、`config` 读配置目录；`git` 的 `runOut` 为子进程显式构造环境属于其执行职责，不算读环境）；其它 util 包（`git` 的纯解析函数 / `slicekit` 等）一律不得调用这类函数。参数处理、cwd 解析、交互编排属于 `cmd` 层职责，不沉淀进 util 包。
    - 警惕功能重叠：例如「向上探测 `.git` 根」已有 `git.FindGitRoot(dir)`，调用方就不该再写一遍 `os.Stat(filepath.Join(..., ".git"))` 的循环。
11. **注释只写「为什么」和「目的」，不要复述「执行过程」**。函数体内的步骤标号（`// 1. 先读 pid 文件 // 2. 再发信号`）、逐行翻译式注释（`// 遍历列表`、`// 返回结果`）属于过程复述——代码本身已经表达了执行过程，注释再写一遍只会制造**两个需要同步维护的事实源**，代码改了忘改注释就会两边对不上。应保留的是代码读不出来的信息：设计意图（如「端口冲突要报错，否则造孤儿」）、非显然的取舍（如「用 SIGKILL 兜底而不是无限等」）、外部约束（如「子进程 stdio 接 /dev/null，日志走 slog」）。判断标准：如果删掉这条注释，读者看代码能否理解「在做什么」——能，就删；读者看代码无法理解「为什么这么做」，就留。
12. **前端 shadcn 基于 Base UI，不可使用 Radix UI 写法**。前端源码在 `web/`（仓库根），shadcn style 为 `base-mira`，UI 原语统一从 `@base-ui/react/<模块>` 子路径导入并按命名空间使用部件（如 `@base-ui/react/checkbox` 的 `CheckboxPrimitive.Root` / `.Indicator`）。AI 训练语料中的 shadcn 示例绝大多数是 Radix 版本，写/改 `web/src` 时**勿照搬 Radix 写法**：不引入 `@radix-ui/*` 依赖；组件多态渲染用 `render` prop 而非 `asChild`；状态样式用 `data-open` / `data-checked` 等具体布尔属性而非 `data-state="..."`。不确定 API 时以 `web/src/components/ui/` 现有组件为准，参考 [Base UI 文档](https://base-ui.com)。

13. **Go struct 与其方法必须放在同一文件中**。同一 struct 的定义（`type Xxx struct { ... }`）和该 struct 的所有方法（`func (x *Xxx) Method()`）必须写在同一个 `.go` 文件里，不要分散到多个文件——方便审阅时一次看完一个类型的全部行为。
   - 允许拆分的是：与 struct 无关的纯函数/纯辅助工具（如解析函数、常量、独立类型定义），它们可以按职责分文件存放。
   - 示例：`workbench/service.go` 包含 `Service` struct 定义和全部 17 个方法；`workbench/diff.go` 仅保留 `DiffEntry` 类型、`DiffTreesResult` 类型和纯辅助函数。

14. **git 子进程调用只允许出现在 `util/git` 包内**。包外（domain / cmd / web / app）不得 `exec.Command("git", ...)`、不得拼 git 子命令参数，一律调用 `util/git` 的**类型化函数**；缺什么就在包内新增类型化封装（含输出解析），**不对外暴露「调用方传 args 的通用执行口」**（历史上曾有 `RunRead`，已移除）。
   - 原因：git 的参数拼装与输出解析天生平台/环境强相关（`core.quotePath` 对非 ASCII 路径的八进制转义、locale 随配置翻译 stderr、pager 挂起、`--no-optional-locks` 防抢 `index.lock`、`diff --no-index` 有差异时退出码为 1、`ls-files --directory` 对混合目录的折叠语义等），必须收敛在单点做兼容——换环境（如部署到 Linux 服务器）出问题时统一排查、统一修，而不是去改散落在各处的参数。这些兼容注入见 `runOut`（`util/git/run.go`）。
   - 返回值解析也尽量放包内（返回类型化的 struct / 切片 / 集合，如 `[]DiffFile` / `*Ignored`），调用方只做业务语义映射；纯展示层 DTO 转换（如状态字母 → 前端语义词）留在出口层。
   - 例外：测试代码（`internal/testfixture` 与各 `*_test.go` 建仓/对账）可直接 exec git——fixture 若反向调用被测包会循环依赖。

15. **Web API 只用 GET / POST 两种 method**。GET = 查询（参数走 query），POST = 动作（参数走 body，平铺挂在 huma input 的 `Body` 子结构上，惯例同 `opener/open`）。不引入 PUT / DELETE / PATCH——保存、删除等语义放进 **API 名**（path / operationId，如 `workbench/file/save`、`xxx/delete`），不用 method 区分。前后端都不提供其他 method 的 helper（后端 `api.go` 仅 `ApiGet` / `ApiPost`，前端 `client.ts` 同；曾有的 `apiPut` / `apiDelete` / `apiRegisterOp` 已移除）。
16. **Service 包的代码布局**（workbench / project / opener / history 等 service 形态的包）：
    - **入口统一在 service.go**：所有依赖 Service 的对外能力都是 `Service` 的公开方法，全部集中在 service.go（与规则 13 呼应）。方法体保持薄——参数校验与分支编排之外的具体逻辑一律外调；10 行以内、不值得单开主题的简单逻辑可直接写在方法里。
    - **复杂逻辑内聚到主题文件**：需要独立输出类型、含复杂算法、或多个方法共享辅助函数的主题（如 diff.go / pty.go）单独成文件，以**包级函数**暴露功能（可含该主题私有的类型与常量），service.go 只留一行委托。判断标准：这组逻辑是否值得脱离 Service 单独阅读与测试。
    - **types.go 放跨主题复用的类型**：struct / 常量，及其构建、解析函数（newXxx / parseXxx 等）；仅单主题使用的类型跟随主题文件。
    - **helpers.go 放跨主题复用的内部函数**：与具体主题无关的通用小工具。


## 文档

改动敏感区域前先读：

- [`docs/spec/现状.md`](./docs/spec/现状.md) —— 项目现状（定位/架构/命令/API/数据/配置）。改架构边界或加 domain 前必读。**与代码冲突时以代码为准**。
- [`docs/proposals/`](./docs/proposals/) —— 待办需求提案（按 `1xxx-主题/` 目录组织，每个提案含 README.md，部分含 alternatives.md / design/）。**新建提案必须先 `make last-proposal` 取最大 ID，新 ID = 最大 ID + 1，不得重复或凭猜测取号**（详见 docs/README.md 的「ID 分配规则」）。
- [`docs/references/`](./docs/references/) —— 竞品分析（mani / gitbatch，做批量操作前必读 gitbatch 避坑点）。
- `README.md` —— 项目主要变更总览。

## 已知 gotcha

- **go 源码根在 `server/`**，不是仓库根。`go build` / `go test` / `goimports` / `go vet` 都要在 `server/` 下跑（Makefile 和 run.sh 已处理 `cd`，手动执行时别忘）。
- **module path 是 `cube`**（不是 `github.com/heyuuu/cube`——README 里写的旧值，以 go.mod 为准）。import 路径写 `cube/...`。
- `logger` 包用 `runtime.Callers` 在 `init()` 里推算项目绝对路径（`relativeProjPath = "../../"`），移动/重命名 logger 源文件位置会让日志里的 `file` 相对路径错位。
- `.gitignore` 忽略：`tmp/`、`runtime/`（测试产物）、`server/web/ui`（`make build-ui` 从 `web/dist` 复制而来，go:embed 嵌入）、`openapi.json`、`.zcode/plans` / `.claude/plans` / `.cursor/plans`。不要提交这些。
- 默认配置目录按环境分流：dev `~/.config/cube-dev/`、prod `~/.config/cube/`（非项目目录），运行期状态（sqlite `data.db`、`cache/git.json`、日志）落在对应配置目录。两环境数据不互通（history 各自积累、config 人工 diff 合并）。
- **前端源码在 `web/`（仓库根）**，`make build-ui` 时 `pnpm -C web build` 后把 `web/dist` 拷到 `server/web/ui` 供 go:embed 嵌入。改前端改 `web/`，不要直接改 `server/web/ui/`（会被覆盖）；旧版 vanilla 前端 `ui/` 已删除。
