# AGENTS.md

面向未来 ZCode agent 的项目工作规则。先读此文件，再动手改 cube。

## 项目简介

**cube** —— 面向个人开发者的本地多项目管理工具（CLI 优先 + 本地 Web）。Go 1.25 编写，module path `github.com/heyuuu/cube`。

- 历史有三代：v1 (php)、v2 (go)、**v3 (当前，按领域重构)**。
- 出口：CLI（人用 / alfred）、本地 Web HTTP server（`cube server`，huma + 标准 ServeMux）。MCP 出口为后续规划。
- 定位原则：不做云服务、不绑 AI（cube 可被 AI 编排，但自身不集成 AI）。

## 分层架构（改代码必须遵守的依赖纪律）

```
基础设施  config / db / logger / version                                  所有层共享
能力      opener / util(git / gogit / fuzzy / easycache / pathkit / slicekit)  通用动作, 不含业务实体
领域      project (含 gitcache / scan / clone)                            业务 domain, 含实体和规则
出口      cmd / web                                                       把领域包成 CLI/Web
装配      app / main                                                      接线
```

- 基础设施不依赖上层；能力层只依赖基础设施；领域层依赖能力+基础设施；**cmd 与 web 不互调**；`app` 是唯一接线点（`app.Default()` 用 `sync.Once` 懒初始化整个 App）。
- 加一个新 domain = ①领域包 ②`cmd/<x>` 子命令组 ③`web.NewXxxHandler` ④config 加节 ⑤`app/init.go` 装配清单加构造。**五处都是加法，不碰现有 domain**。

## 关键机制（改动前先理解）

- **项目前提：所有项目都是 git 项目**。`.git` 存在是扫描判定项目的必要条件（详见 `project/scan.go`）。因此 `tags` 不打冗余的 `git` 标签，只标额外特征（`worktree` / `godot`）。改扫描/tag 逻辑时遵守此假设。
- **gitcache 异步采集**：`project list --status` 等读命令从 `~/.config/cube/cache/git.json` 读 git 状态快照（几乎零开销）；后台 fork 子进程异步采集回写，TTL 1 分钟内不重复，跨进程 flock 串行化。读路径**不得阻塞**采集——只能读快照。详见 `docs/design/v3-design.md` 第四节。
- **opener 参数槽 (slots)**：`Opener` 的能力由 `slots []Slot` 声明（`dir` / `file` / `path` + 可选 shell glob），`Arity() = len(slots)`；`cmd` 中用 `$0/$1...` 占位符引用路径。改 `opener` 时务必同步看 `opener/slot.go` 和 `command_test.go`。
- **easycobra**：`cmd/util/easycobra` 是 cobra 的封装，分组命令（无 `Run` 的纯分组）+ 叶子命令（`Run` 或 `InitRun`）两种。分组命令不会触发 `PersistentPreRunE`，所以全局初始化放在 `cobra.OnInitialize`（见 `cmd/root.go`）。
- **App 懒初始化**：`app.Default()` 首次调用才构造各 service。`cmd/*` 通过 `app.Default().ProjectService()` 等访问。不要在包级 `init()` 里反向依赖未就绪的服务。
- **Web 装配**：`web.NewServer(handlers ...Handler)`，每个 domain 实现 `Handler.Register(api huma.API)`；统一 `ApiOutput{ok,message,data}` envelope；路径强制 `/api/` 前缀，由 `apiRegister` 解析 group tag + operationId。
- **配置**：默认目录 `~/.config/cube/`，`config.json` 按 domain 分节（`log` / `project{scan,clone}` / `openers`）。`-c` 覆盖目录，`-d` 开 debug。配置解析失败/缺失不阻断启动（降级优先，见 `opener.NewService` 跳过坏配置）。

## 常用命令

构建 / 安装（Makefile 已注入 version ldflags）：

```bash
make build        # 构建到 tmp/cube
make install      # go install 到 GOBIN
make tag          # 当前位置打递增版本 tag
```

Web 开发热重载（`.air.toml`，已内置 `goimports -w .` + `go vet ./...` 预检）：

```bash
air               # 需安装 air；args_bin = ["-d", "server"]
./run.sh [args]   # 手动：goimports -> go build -> 运行
```

测试（标准 go test，无额外 harness）：

```bash
go test ./...
go test ./opener/...     # 聚焦某个包
```

### 测试辅助包 `internal/testfixture`

cube 的 IO 测试（git 操作、文件扫描、缓存读写）通过 `internal/testfixture` 包提供统一 fixture builder：

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

**为什么 testfixture 不 import `project` 包**：底层包（`gogit`/`gitcache`）的测试要用 testfixture，而 `project → gitcache → gogit` 是依赖链。若 testfixture 反向依赖 project 会形成循环。所以「构造 project.Service」这种依赖 `project` 包的逻辑写在调用方测试里（见 `project/scan_test.go` 的 `newServiceAt`），不沉淀进 testfixture。

### 测试策略（什么测、什么不测）

- **纯函数**（解析、计算、字符串处理）：普通表驱动测试。`fuzzy`/`pathkit`/`git/url`/`gogit 纯函数`/`slicekit`/`easycache`/`opener 解析`。
- **依赖外部进程/库的 IO**（go-git 读仓库、git 二进制）：**用 testfixture 建真实临时仓库测**，不 mock。`gogit` 的 `Branches/Remotes/Tags/IsDirty`、`git.FindGitRoot`、`gitcache.Load/Save/Refresh/collectEntry`。
- **依赖 sqlite**：用 `:memory:` 内存库 + 直接 AutoMigrate。`history` 全部测试。
- **依赖真实目录扫描**：用 testfixture 建工程目录树，构造 `config.ProjectConfig` 喂给 `project.NewService`（绕开 config/app 单例）。`project/scan_test.go`。
- **不写单测的（靠手动/集成验证）**：
  - `git.Run`/`git.Clone`/`git.Push`（透传 stdio 到 `os.Stdout`，无法捕获输出；且本质是组装 git 参数）
  - `gitcache.TryAsyncRefresh`（fork 自身可执行文件跑子命令，进程编排非逻辑）
  - `opener.Open`（实际启动编辑器/IDE，副作用）
  - `config`/`db`（全局单例无 setter，测试无法隔离）
  - `cmd/*`（cobra 命令编排）、`web`（huma 路由 + envelope，集成测比单测值）

## 必须遵守的编码规则

1. **Go 代码修改后必须先格式化与校验再提交/收工**——这是强制规则：
   ```bash
   goimports -w .     # 格式化 + 整理 import（本地已安装：~/go/bin/goimports）
   go vet ./...       # 静态校验
   ```
   每次改完 `.go` 文件都要跑，不要跳过。`air` 与 `run.sh` 也已内置这两步，保持一致。
2. 遵循 v3 分层依赖纪律（见上），不要让 `cmd` 直接调 `web`、不要让基础设施包 import 领域包。
3. **加新 domain 走"五处加法"流程**，不修改既有 domain 的接线。
4. 日志统一用 `log/slog`（`slog.Debug` / `slog.Info` / ...），不要用 `fmt.Println` 做日志（`fmt` 仅用于面向用户的 CLI 输出）。debug 日志受 `-d` 控制。
5. 错误处理遵循现有风格：可恢复的降级用 `log.Printf`/`slog` 记录后继续；致命错误用 `fmt.Errorf("...: %w", err)` 包装并返回。
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
   仅当逻辑较复杂、单行会牺牲可读性时才折行展开。

   **准确定义**：本规则所说的 getter/setter 仅指——
   - 是 **struct 的方法**（带 receiver），不是包级函数；
   - 方法名形如 `GetXxx()` / `SetXxx(v)`（Go 惯例 getter 不带 Get 前缀，直接用属性名，如 `Name()`）；
   - 方法体是**直接读 / 直接写 struct 的某个属性**（`return o.xxx` 或 `o.xxx = v`）。

   以下**不属于** getter/setter，仍按多行显示：
   - 包级函数（如 `config.Path()`、`config.Default()`、`db.Default()`、`config.SetDebug()`）；
   - 虽名为 `GetXxx`/`SetXxx` 但方法体不是属性的直接读/写——例如 `return s.cache.Get()`、`return strings.Join(o.cmd, " ")`、`Projects()`（委托、计算、聚合等）。

   getter/setter 的书写约定：
   - **建议不加注释**（建议性，非强制）——struct 属性的行尾注释通常已足够说明，方法上再写只会重复。参考 `project.Project`：属性 `path string // 项目路径，唯一标识`，getter `Path()` 不写注释。**但如果注释包含超出属性说明本身的内容**（如跨文件调用指引、设计意图、注意事项等），则应当保留。参考 `gitcache.Cache.Dir()`：除说明返回值外，还注明「供 refresh.go 的 TTL/flock 等调度逻辑使用」。
   - **多个 getter（或多个 setter）连写在一起，不加空行**；顺序与对应属性在 struct 内的声明顺序一致。参考 `project.Project` 的 `Group/Name/Path/Tags/GitInfo`。
   - getter 组与其它方法之间保留一个空行分隔。
9. 写表用 `tui.PrintTable`，交互选择用 `tui.SelectItem`，保持 CLI 输出风格一致。
10. **优先复用 `util/` 下的辅助函数**，能力收敛在各 util 子包内，不在调用方就地重造：
    - 动手前先 grep 对应 util 包；缺什么就**在该 util 包里加新函数**，而不是在 `cmd/` / domain 里实现。例：git 读走 `util/gogit`（分支 / remote / tag / ahead-behind / dirty），git 写走 `util/git`（push / clone / commit / `FindGitRoot`）；路径处理走 `util/pathkit`；切片运算走 `util/slicekit`。
    - util 包内的函数必须**足够内聚且无副作用**：只依赖入参做纯运算，不读进程状态、不读环境。与环境强相关的副作用（`os.Getwd()` / `os.Getenv()` / 读 `~` / 当前时间等）只允许出现在**职责就是处理环境的 util 包**（如 `pathkit` 展开 `~`、`config` 读配置目录）；其它 util 包（`git` / `gogit` / `slicekit` 等）一律不得调用这类函数。参数处理、cwd 解析、交互编排属于 `cmd` 层职责，不沉淀进 util 包。
    - 警惕功能重叠：例如「向上探测 `.git` 根」已有 `git.FindGitRoot(dir)`，调用方就不该再写一遍 `os.Stat(filepath.Join(..., ".git"))` 的循环。

## 文档

改动敏感区域前先读：

- `docs/design/v3-design.md` —— v3 架构、分层、gitcache、Web API、里程碑、扩展模型。改架构边界或加 domain 前必读。
- `docs/design/v3-frontend.md` —— Web 前端规划（当前 server 只暴露 API + `/docs`，前端 embed 为 M4）。
- `README.md` —— v3 主要变更总览。

## 已知 gotcha

- `logger` 包用 `runtime.Callers` 在 `init()` 里推算项目绝对路径（`relativeProjPath = "../../"`），移动/重命名 logger 源文件位置会让日志里的 `file` 相对路径错位。
- `tmp/`、`cube`（构建产物）、`openapi.json` 在 `.gitignore` 内，不要提交。`.zcode/plans` 也已忽略。
- 默认配置目录是 `~/.config/cube/`（非项目目录），运行期状态（sqlite `data.db`、`cache/git.json`、日志）都落在那里。
