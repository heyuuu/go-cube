# Go 后端 + Web 前端 · 项目设计规范（通用模板）

> 本文档是「Golang 后端 + 本地 Web 前端」这类**个人开发工具项目**的通用设计规范。它从 [cube](../) 项目提炼，把 cube 验证过有效的架构范式抽象成可复用模板。
>
> **定位**：新项目立项时，照此模板抄目录结构、技术选型、分层纪律、前后端契约；项目自身的行为规约（需求/设计/任务）另写一份 `spec.md`（见第十节）。本文是**项目骨架**的规范，不是产品功能规范。
>
> **维护节奏**：这份模板随 cube 演进迭代。cube 的多窗口工具（workspace）落地过程中会持续打磨本模板，稳定后再抽出为通用 skill。

---

## 一、适用场景

**适合**：

- 个人/小团队的**本地工具**（CLI + 本地 Web server，非云服务）
- 后端用 Go，前端用现代构建链（Vite + React + TS）
- 前后端通过 **OpenAPI 契约**对接（后端生成 → 前端消费）
- 单一可执行二进制（前端 `go:embed` 进二进制）

**不适合**：

- 纯云服务 / 多租户 / 需要认证授权的 Web 产品
- 移动端、SSR、高并发公网服务
- 后端非 Go 的项目（模板的分层、依赖纪律是 Go 语境的）

**典型例子**：本地多项目管理、本地 db 查看、端口/进程查看、本地文件批处理、开发环境编排——即「反复手动操作 → 收进工具」这一类个人辅助工具。

---

## 二、推荐技术栈

### 2.1 后端

| 能力 | 选型 | 理由 |
|---|---|---|
| HTTP 框架 | **[huma/v2](https://github.com/danielgtaylor/huma)** + `humago` adapter | 自动生成 OpenAPI 3.1，自动挂 `/openapi.json` `/docs`，强类型输入输出，零手写路由文档。是「后端生成 OpenAPI 给前端」工作流的源头。 |
| CLI 框架 | **[cobra](https://github.com/spf13/cobra)** | Go CLI 事实标准。子命令分组、flag、补全都顺手。 |
| 日志 | **标准库 `log/slog`** | 不引第三方。自定义 handler 实现文件 + stdio 双通道。 |
| 配置 | JSON 文件（按 domain 分节） + **[fsnotify](https://github.com/fsnotify/fsnotify)** 热 reload | 个人工具不需要 yaml/toml 的复杂度，JSON 手改方便、读写简单。 |
| 数据库 | **[gorm](https://gorm.io)** + **[gorm/sqlite](https://gorm.io/driver/sqlite)** | sqlite 足够个人工具；gorm 的 AutoMigrate 省心。重读场景配内存缓存。 |
| git 操作（如需） | **[go-git/v5](https://github.com/go-git/go-git)**（纯 Go 读） + 系统 `git`（写，透传 stdio） | 读走纯 Go 零依赖；写透传 stdio 让用户看到 git 真实输出。 |
| TUI（如需） | **[huh/v2](https://github.com/charmbracelet/huh)** + **[lipgloss/v2](https://github.com/charmbracelet/lipgloss)** | 交互式表单/选择、表格样式。 |

**版本基线**（参考 cube，新项目取最新稳定版）：
```
go 1.25+
github.com/danielgtaylor/huma/v2  v2.39+
github.com/spf13/cobra            v1.10+
gorm.io/gorm                      v1.31+
gorm.io/driver/sqlite             v1.6+
github.com/fsnotify/fsnotify      v1.10+
```

### 2.2 前端

| 能力 | 选型 | 理由 |
|---|---|---|
| 构建链 | **[Vite](https://vitejs.dev)** | 快、标准、生态顺。`dev` 热重载 + `build` 产物给 go:embed。 |
| 框架 | **[React](https://react.dev)** + **TypeScript** | 组件生态最大；Monaco/xterm/复杂 UI 都有现成绑定。 |
| UI 组件 | **[shadcn/ui](https://ui.shadcn.com)** + **Tailwind CSS** | 源码进项目、可改、无运行时依赖负担。 |
| 数据请求 | **[TanStack Query](https://github.com/TanStack/query)** | 与 RESTful + OpenAPI 生成的类型客户端契合，缓存/loading/error 省心。 |
| 路由 | **hash 路由**（简单项目）或 React Router | localhost 工具，hash 路由 F5 刷新不丢失当前页，且无需 server side 路由配合。 |
| API 类型 | **后端 OpenAPI 生成 → 前端类型/客户端代码生成** | 见第五节，核心工作流。 |

**可选前端库**（按需）：
- 文件树：[react-arborist](https://github.com/brimdata/react-arborist)
- 代码编辑器：[Monaco Editor](https://github.com/microsoft/monaco-editor) + `@monaco-editor/react`
- 终端：[xterm.js](https://xtermjs.org)
- 图表/复杂表格等：按需引入

### 2.3 工具链

| 能力 | 选型 |
|---|---|
| Go 格式化 + import | `goimports -w .` |
| Go 静态检查 | `go vet ./...` |
| 热重载（开发） | [air](https://github.com/cosmtrek/air)，内置 goimports + go vet 预检 |
| 前端热重载 | Vite dev server，开发期走 proxy 到后端 |
| 版本号注入 | `go build -ldflags`（Makefile 注入 git tag/commit） |
| API 文档 | huma 内置 Scalar 渲染（`/docs`） |

---

## 三、分层架构与依赖纪律

这是模板的**核心纪律**。Go 后端必须保持单向依赖，禁止循环。

```
基础设施  config / db / logger / version                  所有层共享，不依赖上层
能力      util/*（git / pathkit / slicekit / fuzzy ...）  通用动作，不含业务实体
领域      <domain>/（含实体、规则、service）               业务核心，依赖能力+基础设施
出口      cmd / web                                       把领域包成 CLI / Web
装配      app / main                                      唯一接线点
```

### 依赖规则

1. **基础设施层**（config/db/logger/version）：不 import 任何上层。
2. **能力层**（util/*）：只依赖基础设施。util 子包必须**内聚且无副作用**——只对入参做纯运算，不读进程状态/环境。与环境强相关的副作用（`os.Getwd`、`os.Getenv`、展开 `~`、当前时间）只允许出现在职责就是处理环境的 util 包里。
3. **领域层**（各 domain）：依赖能力 + 基础设施。domain 之间尽量不互相依赖；确需共享实体，下沉到能力层或基础设施层。
4. **出口层**（cmd / web）：**cmd 与 web 不互调**。都只通过 `app` 访问领域 service。
5. **装配层**（app / main）：唯一接线点。`app.Default()` 用 `sync.Once` 懒初始化整个 App。**不要在包级 `init()` 里反向依赖未就绪的服务**。

### 加新 domain 的「五处加法」流程

加一个新领域（如 `workspace`、`db`），是**纯加法**，不动既有 domain：

1. 领域包：`<domain>/`（实体 + service + 规则）
2. CLI 子命令组：`cmd/<domain>/`
3. Web handler：`web.NewXxxHandler`，实现 `Handler.Register(api huma.API)`
4. config 加节：在 `config.Config` 加字段 + 默认值
5. 装配清单：`app/init.go` 加构造（`NewService(...)` 注入依赖）

> 五处都是加法是纪律的核心——**新增不污染既有 domain**。如果加新 domain 必须改既有 domain，说明领域边界划错了，先重构边界。

### domain 内部结构（建议）

```
<domain>/
  <domain>.go      // 实体定义（struct + getter/setter）
  service.go       // Service 聚合根，对外接口
  <子能力>.go       // 按职责拆分（如 scan.go / clone.go / tree.go）
  <domain>_test.go // 表驱动测试
  <子包>/          // 重的子能力下沉为子包（如 gitcache/）
```

---

## 四、目录结构规范

### 4.1 后端（Go）

```
<project>/
├── main.go                 # 入口，调用 cmd.Execute()
├── embed.go                # //go:embed 前端产物（必须在 main 包，见 §8.2）
├── go.mod / go.sum
├── Makefile                # build / install / tag（见 §8.1）
├── .air.toml               # air 热重载配置
├── app/                    # 装配层：init.go（构造清单）、watch.go（热 reload）
├── cmd/                    # CLI 出口
│   ├── root.go
│   ├── server/             # cube server / cube server openapi
│   └── <domain>/           # 各 domain 的子命令组
├── web/                    # Web 出口
│   ├── server.go           # huma v2 Server + NewServer(handlers...)
│   ├── static.go           # embed 静态资源 serve
│   ├── api.go              # ApiOutput envelope + apiGet/apiPost 注册辅助
│   └── api_<domain>.go     # 各 domain 的 Handler
├── config/                 # 基础设施：配置加载/保存/热 reload
├── db/                     # 基础设施：gorm + sqlite
├── logger/                 # 基础设施：slog handler（文件 + stdio 双通道）
├── version/                # 基础设施：版本号（ldflags 注入）
├── <domain>/               # 领域层：每个 domain 一个包
├── util/                   # 能力层：各 util 子包
│   ├── git/                #   系统 git 写操作（透传 stdio）
│   ├── gogit/              #   go-git 读封装
│   ├── pathkit/            #   路径处理（含 ~ 展开）
│   ├── slicekit/           #   切片运算
│   └── ...
├── internal/               # 不对外的内部包
│   └── testfixture/        #   测试辅助（建临时仓库/目录）
├── ui/                     # 前端源码 + 构建产物（见 §4.2）
└── docs/                   # 文档
    ├── spec.md             #   项目自身的契约快照（见 §10）
    └── design/             #   设计推理、历史
```

### 4.2 前端（Vite + React）

```
ui/
├── package.json
├── vite.config.ts          # build.outDir = 'dist'，产物给 go:embed
├── tsconfig.json
├── index.html              # Vite 入口
├── src/
│   ├── main.tsx            # React 挂载
│   ├── App.tsx             # 根组件 + 路由
│   ├── components/         # shadcn/ui 组件（源码进项目）
│   ├── views/              # 页面级组件
│   ├── api/                # OpenAPI 生成的客户端（见 §5.3）
│   ├── lib/                # 工具函数
│   └── styles/             # Tailwind 入口
└── dist/                   # 构建产物（gitignore），go:embed 目标
```

**关键约定**：
- 前端源码与构建产物都在 `ui/` 下，`ui/dist/` 是 embed 目标。
- `ui/dist/` 进 `.gitignore`，不入库。
- `embed.go` 写 `//go:embed ui/dist`（见 §8.2）。

---

## 五、前后端契约：OpenAPI 工作流 ⭐

**这是模板的枢纽。** 前后端通过 OpenAPI 文件对接，不手写接口类型，不靠口头约定。

### 5.1 工作流总览

```
┌─────────────┐   huma 自动生成    ┌──────────────┐   代码生成      ┌──────────────┐
│ Go 后端      │ ───────────────▶ │ openapi.json │ ──────────────▶ │ 前端 TS 类型  │
│ (huma v2)   │                   │ (契约源)      │                 │ + API 客户端 │
└─────────────┘                   └──────────────┘                 └──────────────┘
       ▲                                                                  │
       │                                                                  ▼
       └──────────────── 实际 HTTP 请求（运行时） ─────────────────────────┘
```

### 5.2 后端：声明即契约

huma v2 的核心价值：**Go struct 的 tag 就是 OpenAPI schema 的来源**。写 handler 时声明输入输出 struct，OpenAPI 自动生成。

**统一响应信封**（强制，所有 API 走同一形态）：
```go
// web/api.go
type ApiOutput[T any] struct {
    Body struct {
        Ok      bool   `json:"ok"`
        Message string `json:"message,omitempty"`
        Data    T      `json:"data,omitempty"`
    }
}
```

**注册辅助**（强制 `/api/` 前缀，自动从 path 推导 tag + operationId）：
```go
// web/api.go
func apiGet(api huma.API, path string, h func(ctx.Context, i *Input) (*Output, error)) { ... }
func apiPost(api huma.API, path string, ...) { ... }
```

**handler 写法示例**：
```go
// web/api_project.go
func (h *ProjectHandler) Register(api huma.API) {
    apiGet(api, "/project/list", h.listProjects)
}

type listProjectsOutput struct{ Body ApiOutput[[]ProjectDTO] }

func (h *ProjectHandler) listProjects(ctx context.Context, i *struct{}) (*listProjectsOutput, error) {
    projects := h.svc.Projects()
    return &listProjectsOutput{Body: ApiOutput[[]ProjectDTO]{ /* ... */ }}, nil
}
```

**huma 自动暴露**（无需手写路由）：
- `GET /openapi.json` / `/openapi.yaml` — OpenAPI 3.1 规范
- `GET /docs` — Scalar 渲染的交互式文档

**CLI 导出**（给前端代码生成用）：
```bash
<app> server openapi --out openapi.json   # 产物 gitignore，不入库
```

### 5.3 前端：类型与客户端生成

从 `openapi.json` 生成前端 TS 类型 + API 客户端。推荐工具（按需选一）：

| 工具 | 产出 | 适合 |
|---|---|---|
| **[openapi-typescript](https://github.com/drwpow/openapi-typescript)** | 纯类型（`.d.ts`） | 只想要类型，fetch 自己写 |
| **[openapi-fetch](https://github.com/drwpow/openapi-fetch)** | 类型安全的 fetch 封装 | 配合 openapi-typescript，轻量 |
| **[orval](https://github.com/anymaniax/orval)** | 类型 + TanStack Query hooks | 想直接生成 `useXxxQuery`，与 TanStack Query 深度集成 |
| **[hey-api/openapi-ts](https://github.com/hey-api/openapi-ts)** | 类型 + 完整客户端 SDK | 想要开箱即用的客户端类 |

**推荐组合**：`openapi-typescript` + `openapi-fetch` + TanStack Query——类型安全、无重运行时、与 Query 契合。

**生成脚本**（`ui/package.json`）：
```json
{
  "scripts": {
    "gen:api": "openapi-typescript ../openapi.json -o src/api/schema.d.ts",
    "dev": "vite",
    "build": "npm run gen:api && vite build"
  }
}
```

**生成产物 gitignore**（`src/api/schema.d.ts` 不入库，由 openapi.json 生成）。

### 5.4 契约纪律

1. **openapi.json 是契约唯一源**。后端改了 handler → 重新导出 openapi.json → 前端重新 gen:api。
2. **openapi.json 不入库**（gitignore），它是构建产物。前端构建时从后端拉取或 CI 生成。
3. **前端不手写接口类型**，一律由生成来。手写 = 漂移风险。
4. **响应统一走 `ApiOutput` 信封**，前端封装一层 unwrap。
5. **错误码**：HTTP 状态码 + ApiOutput.message；业务错误用标准 HTTP 码（400/404/409），不要全 200 靠 body.ok 区分。

### 5.5 开发期联调

- **后端热重载**：`air`（改 Go 代码自动 rebuild + restart server）。
- **前端热重载**：`vite dev`（Vite dev server 起在 :5173，proxy `/api` 到后端 :port）。
- **契约同步**：改后端 handler 后，重跑 `<app> server openapi -o openapi.json` + 前端 `npm run gen:api`。可做成 git pre-commit hook 或 air 的 post-build 钩子。

---

## 六、配置管理

### 6.1 配置目录

默认 `~/.config/<app>/`（支持 `-c` 覆盖目录）。结构：
```
~/.config/<app>/
├── config.json          # 主配置（按 domain 分节，见下）
├── data.db              # sqlite（运行期状态）
├── <app>.log            # 日志
└── cache/               # 运行期缓存（各 domain 自管）
```

**规则**：config.json 是**配置**（用户可改、可热 reload）；data.db / cache / log 是**运行期状态**（程序管、不入库）。

### 6.2 config.json 分节

按 domain 分节，每个 domain 一个 struct：
```go
type Config struct {
    Log     LogConfig      `json:"log"`
    Project ProjectConfig  `json:"project"`
    // 加新 domain：在此加字段
}
```

### 6.3 配置纪律

- **降级优先**：配置缺失/解析失败**不阻断启动**。坏配置跳过该 domain，记录 slog warning。
- **原子写**：保存用 tmp + rename，避免半写状态。
- **热 reload**：web server 运行时，fsnotify 监听配置目录（300ms 去抖）→ 重新加载 → 调各 service 的 `Reload()`。**web.Server 端口/路由不重建**，只 reload service 状态。
- **路径展开**：配置里的 `~/xxx` 在加载时展开为绝对路径（`pathkit.ExpandHome`），使用方拿到的都是绝对路径。

---

## 七、日志

**统一用标准库 `log/slog`**，禁止用 `fmt.Println` 做日志（`fmt` 仅用于面向用户的 CLI 输出）。

### 7.1 双通道 handler

自定义 `slog.Handler`，同时写文件 + stdio：
- **文件通道**：始终启用，落 `<config_dir>/<app>.log`，完整结构化日志。
- **stdio 通道**：仅在 debug 模式（`-d` flag）或文件 handler 初始化失败时启用，输出到 stderr 带 ANSI 颜色。

### 7.2 日志格式

- debug 日志受 `-d` 控制；非 debug 只输出 Info 及以上。
- `{file}` 字段渲染为相对项目根的短路径（便于定位）。⚠️ 若复刻此能力，用 `runtime.Callers` 推算项目根是**位置敏感**的——移动 logger 源文件位置必须同步调整相对路径常量。

---

## 八、构建与发布

### 8.1 Makefile

```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS  = -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o tmp/<app> .

install:
	go install -ldflags "$(LDFLAGS)" .

tag:
	# 打递增版本 tag（具体逻辑按项目约定）
```

**版本注入**：`version` 包暴露变量，`main` 包 ldflags 注入。`<app> version` 命令 + huma 的 OpenAPI `info.version` 都用它。

### 8.2 go:embed 前端产物

```go
// embed.go (必须在 main 包，因为 //go:embed 不能引用上级目录)
package main

import "embed"

//go:embed ui/dist
var UIFS embed.FS
```

装配层注入（web 包不直接持有 FS，由 main 注入）：
```go
// main.go
func main() {
    web.SetUIAssets(UIFS)
    cmd.Execute()
}

// web/server.go
var uiAssets embed.FS
func SetUIAssets(fs embed.FS) { uiAssets = fs }
```

**空 FS 探测**：`isUIAssetsEmpty()` 用 `<uiRoot>/index.html` 是否存在判断，空时跳过静态 serve（支持「不带前端构建产物也能跑 server」的开发场景）。

### 8.3 热重载（开发）

**`.air.toml`**：监听 `.go` 文件变化，rebuild + restart。**内置预检**：
```toml
[build]
  pre_cmd = ["goimports -w .", "go vet ./..."]
  cmd = "go build -o ./tmp/<app> ."
  post_cmd = ["./tmp/<app> server"]
```

**强制规则**：Go 代码改完必须先 `goimports -w .` + `go vet ./...` 再提交/收工。air 已内置，`run.sh` 也保持一致。

### 8.4 发布产物（gitignore）

```
tmp/                # air 产物、构建中间
<app>               # 二进制
openapi.json        # OpenAPI 生成产物
ui/dist/            # 前端构建产物
ui/node_modules/
runtime/            # 运行期数据（测试 fixture 等）
```

---

## 九、测试策略

### 9.1 测什么

| 类型 | 方法 |
|---|---|
| **纯函数**（解析、计算、字符串、切片运算） | 表驱动测试 |
| **依赖外部进程/库的 IO**（go-git 读仓库、系统 git、文件扫描） | **建真实临时环境**测，不 mock |
| **依赖 sqlite** | `:memory:` 内存库 + 直接 AutoMigrate |
| **依赖真实目录扫描** | 用 testfixture 建工程目录树 |

### 9.2 不测什么（靠手动/集成验证）

- 透传 stdio 的命令（`git run` 类，无法捕获输出）
- fork 子进程的编排逻辑（非纯逻辑）
- 实际有副作用的启动（开编辑器、开 IDE）
- 全局单例无 setter 的包（config/db，测试无法隔离）
- CLI 命令编排（cobra wiring）、Web 路由 + envelope（集成测比单测值）

### 9.3 testfixture 模式

提供 `internal/testfixture` 包，统一 IO 测试的 fixture builder：
- 临时目录落 `runtime/test/`（**不用系统 `/tmp`**），失败时方便翻看现场。
- 每个测试独立子目录，不自动清理。
- 提供建真实 git 仓库、建工程目录树的 helper。

**为什么 testfixture 不 import 领域包**：底层包测试要用 testfixture，而 `领域 → 底层` 是依赖链。testfixture 反向依赖领域会循环。所以「构造领域 Service」这种依赖领域包的逻辑写在调用方测试里，不沉淀进 testfixture。

---

## 十、项目自身 Spec（SDD 三段式）

本模板规范的是**项目骨架**。每个项目还要写一份**自身的行为契约** `docs/spec.md`，采用 SDD 三段式：

```
## 一、Requirements（需求）
EARS 语法：WHEN <触发> THE SYSTEM SHALL <行为> / IF <条件> THEN ... / THE SYSTEM SHALL <无条件>
标 ✅ 已实现 / 🟡 有缺陷 / ⏳ 计划中。反向提取已落地功能为需求。

## 二、Design（设计）
技术契约浓缩：分层、数据模型、关键机制、配置契约、出口契约、测试策略。
详细推理见 docs/design/，spec.md 是事实源。

## 三、Tasks（未来工作）
checklist 按里程碑分组，每项可独立完成、可验证。
```

**维护节奏**：合并一个有意义变更后同步更新 spec.md（实现的需求从 Tasks 移到 Requirements；过时的 Design 段修正）。`docs/design/` 记「为什么这么设计」，spec.md 记「当前是什么」。

---

## 十一、可选扩展点

模板的基线是「CLI + HTTP API + embed 前端」。以下能力按需挂载，都是**加法**：

| 扩展 | 形态 | 架构落点 |
|---|---|---|
| **长连接会话**（PTY 终端、文件 watch 推送） | WebSocket | 基础设施层新增 `pty`/`ws` 子包；web 出口层接 ServeMux Upgrade。**引入长连接是架构复杂度的临界点，单独权衡。** |
| **TUI 交互**（多选、表单、表格） | huh + lipgloss | 能力层或 cmd/util/tui。CLI 出口的交互组件统一收口。 |
| **Alfred / Raycast / Script Filter 集成** | JSON 输出 | cmd 下独立子命令组；配 history 记录 frecency。 |
| **MCP 出口**（被 AI 平台编排） | MCP server | 第三个出口（与 cmd/web 平级），按 domain 分 toolset。 |
| **多 remote 批量操作** | 批量 API | 领域层加批量 service；注意并发/失败策略（全失败回滚 vs best-effort）。 |
| **worktree 管理** | 领域子能力 | 物理位置约定 + 是否纳入扫描的约定需先决策。 |

> 挂载扩展点时复用「五处加法」流程，新增不污染既有 domain。

---

## 十二、起步流程（新项目如何用模板）

1. **立项**：写 `docs/spec.md` 的 Requirements 段（哪怕只有 3 条），明确做什么、不做什么。
2. **搭骨架**：照 §4 目录结构建空包；`go mod init`；抄 Makefile / .air.toml / .gitignore。
3. **后端起步**：
   - 写 `app/`（懒初始化骨架）、`config/`（配置加载）、`logger/`（双通道 handler）。
   - 写第一个 domain（实体 + service）+ `web/api_<domain>.go`（用 ApiOutput 信封 + apiGet/apiPost）。
   - 写 `cmd/server`（`huma.DefaultConfig` + `SetUIAssets`）。
   - 跑 `<app> server`，访问 `/docs` 确认 OpenAPI 生成正常。
4. **前端起步**：
   - `cd ui && npm create vite@static .`（React + TS 模板）。
   - 装 shadcn/ui（`npx shadcn-ui@latest init`）+ Tailwind + TanStack Query。
   - 配 `vite.config.ts`：`build.outDir = 'dist'`，dev server proxy `/api` 到后端。
   - 装 `openapi-typescript` + `openapi-fetch`，配 `gen:api` 脚本。
   - `<app> server openapi -o openapi.json` → `npm run gen:api` → 写第一个 view 消费 API。
5. **embed 打通**：`npm run build` 产出 `ui/dist/` → 改 `embed.go` 为 `//go:embed ui/dist` → `make build` → 验证单二进制能 serve 前端。
6. **迭代**：按 spec.md 的 Tasks 推进，每个 domain 走五处加法。改后端 handler 后重跑 openapi + gen:api 保持契约同步。

---

## 附录：决策记录（随迭代补充）

| 日期 | 决策 | 上下文 |
|---|---|---|
| 2026-08-04 | 模板首版，从 cube 提炼。技术栈：huma v2 + cobra + slog + gorm/sqlite；前端 Vite + React + TS + shadcn。 | cube 规划多窗口工作台（workspace），决定引入前端构建链，顺势把项目范式抽象成通用模板。 |
