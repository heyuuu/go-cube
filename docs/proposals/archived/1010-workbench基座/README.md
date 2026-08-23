# workbench 基座：路由 + TreeSource 抽象 + 核心 API + 面板骨架

> **状态**：✅ 已完成并验收归档（2026-08-23）
>
> **所属**：[`1008-workspace工作台` 总纲](./1008-workspace工作台/README.md)（实施前先读总纲「已收敛的全局决策」，本提案遵守其中全部约定）。
> **依赖**：无（工作台第一个子提案，1011-1015 都依赖本提案的产出）。

## 背景与目标

工作台是 cube 的独立页面，输入一个本机 git 目录路径，聚合 git 可视化 / 代码阅读 / diff / PTY。本提案只做**基座**：路由、TreeSource 统一抽象、后端核心 API 与领域包、前端固定布局的面板骨架。做完后：访问 `/workbench?path=<git目录>` 能进入一个可用的空工作台，显示项目基本信息与面板占位。

**本提案不做**：commit 图渲染、文件浏览、diff、PTY（分别是 1011-1014）、任何 git 写操作、布局自定义（1015）。

## 方案

### 1. 后端：workbench 领域包

按 AGENTS.md「五处加法」流程新增 domain（本提案无 CLI 子命令、暂无 config 节，实际加三处：领域包 / web handler / app 装配）：

- **领域包 `server/workbench/`**：不依赖 `project` 包（工作台解耦 project scan，输入是任意 git 目录）。git 读能力按项目规则沉淀到 `util/git`（如缺失），workbench 包只做编排。实际文件布局：`service.go`（Service struct + 全部方法）/ `types.go`（TreeSource 等类型与解析）/ `file.go`（文件读取与保存）/ `diff.go`（diff 主题）/ `pty.go`（PTY 会话）/ `helpers.go`。
- **git 读能力**（加到 `server/util/git/`，纯函数 + 子进程读，不写仓库）：
  - `WorktreeList(dir)` —— `git worktree list --porcelain` 解析，返回 `[{Path, Head, Branch}]`。
  - `ListTreeAtRef(dir, ref, path)` —— `git ls-tree` 解析（1012 用）。
  - `ReadFileAtRef(dir, ref, path)` —— `git show ref:path`（1012 用）。
  - 本提案只需要 `WorktreeList`；其余可在本提案一并实现（纯解析函数，表驱动测试友好），也可留给 1012——**由实施者按工作量判断，但 API 契约在本提案定死**。
- **web handler `server/web/api_workbench.go`**：`NewWorkbenchHandler(s *workbench.Service)`，huma 注册，遵循 `ApiOutput` envelope、`/api/workbench/` 前缀、group tag 自动推导（见 `server/web/api.go` 的 `apiRegister`）。nil 切片序列化已由 `nilSliceJSONFormat` 兜底，不要手写 `make([]T,0)`。
- **装配 `server/app/app.go`**：加 `workbench.NewService(...)` + `web.NewWorkbenchHandler(...)` 到装配清单。

### 2. TreeSource 定义与 API 契约

```
TreeSource:
  type: "commit" | "ref" | "worktree"
  id:    commit sha | ref 名（分支/tag）| 工作副本目录绝对路径
```

HTTP 传参用**单字符串序列化**：`source=type://id`（如 `ref://main`、`commit://abc123…`、`worktree:///Users/x/repo`），双源场景 `left=…&right=…`。`ParseTreeSource`（`workbench/types.go`）解析并校验：commit 须 40/64 位 sha、ref 短名按 check-ref-format 核心规则挡 rev 表达式。`path` 始终是项目主目录（用户进来的目录），worktree 的 id 是它自己的绝对路径。前端 URL 参数同构（`params.ts` 的 `source/left/right`，`type://id` 格式）。

核心接口（huma path 即 operationId 来源，注意命名）：

| Method | Path | 说明 |
|---|---|---|
| GET | `/api/workbench/info` | `?path=` → 主目录是否 git 仓库、`git worktree list` 结果、默认分支 |
| GET | `/api/workbench/refs` | `?path=` → 分支 + tag 列表 |
| GET | `/api/workbench/worktrees` | `?path=` → 全部工作副本状态聚合快照（1011 的 status 并入此接口，无独立 status） |
| GET | `/api/workbench/tree` | `?path=&source=&…` → 文件清单（实现为全量扁平相对路径列表，前端组树，见 1012） |
| GET | `/api/workbench/file` | `?path=&source=&file=` → 文件内容（文本，二进制返回标记） |
| POST | `/api/workbench/file/save` | body 传参 → 保存工作区文件（1012；符合仓库「只用 GET/POST」规则，非早期设想的 PUT） |
| GET | `/api/workbench/diff` | `?path=&left=&right=&…filters` → 目录级 diff 摘要（1013） |
| GET | `/api/workbench/file-diff` | `?path=&left=&right=&file=` → 单文件 diff（1013） |
| GET | `/api/workbench/commits` | `?path=&cursor=&limit=` → commit 图分页（1011；恒 `--all`，无 scope/ref 参数） |
| GET | `/api/workbench/changes` | worktree 的变更文件清单（1012 差异模式树） |
| GET(WS) | `/api/workbench/pty` | WebSocket 升级（1014；huma 之外原生 mux 挂载） |

错误语义：`path` 不是 git 仓库 → 中文错误（如 `"path 不是 git 仓库: path=..."`，遵循项目错误消息规则）；`TreeSource` 解析失败/对象不存在 → 404 语义错误信息。

**安全注脚**（总纲已定）：本组接口可读任意本机 git 目录，MVP 接受此边界，不做路径白名单。

### 3. 前端：路由与面板骨架

- **路由**：`web/src/App.tsx` 加 `<Route path="/workbench" element={<WorkbenchPage />} />`（独立于主应用 `Layout`，同 `/md` 的做法——工作台自带整体布局）。页面在 `web/src/pages/workbench/`。
- **URL 参数**：`?path=<git目录绝对路径>`。本提案只消费 `path`；选中态参数在 1011/1012 引入，定稿为 `source` / `left` / `right`（`type://id` 单参数序列化，与 API 参数一致）。
- **固定布局骨架**（不做任何可拖拽/自定义）：
  ```
  ┌────────────┬──────────────────────────┐
  │ git 树面板  │  内容区（代码阅读/diff 占位） │
  │ (1011 占位) │                          │
  ├────────────┴──────────────────────────┤
  │  PTY 抽屉（1014 占位，默认收起）           │
  └───────────────────────────────────────┘
  ```
- **面板架构铁律**（本提案建立，后续提案遵守）：每个面板是 `web/src/pages/workbench/panels/` 下的独立组件，**自带数据 hook**（Query key 从 URL 参数派生、自包含），不依赖父级 props 传选中态；面板间通信只通过 URL search params。
- **TanStack Query**：query key 统一以 `['workbench', path, …]` 开头，封装在 `web/src/queries/` 或面板内 hook。本提案至少有 `useWorkbenchInfo(path)`。
- **空态处理**：无 `path` 参数 → 引导输入目录路径的输入框；入口页提交时先调后台校验，`path` 非 git 目录时就地显示错误不跳转；带非法 `path` 进入工作台 → 退回入口页并展示错误（路径调整的唯一入口就是这个输入框，URL 里转码后的 path 参数对用户不可编辑）。
- UI 组件用仓库内 shadcn（`web/src/components/ui/`），**Base UI 非 Radix**（不用 `asChild`、不用 `data-state`，详见 AGENTS.md 规则 12）。

## 验收标准

1. `cd server && go vet ./... && go test ./...` 通过；新增 git 解析函数有表驱动测试（worktree porcelain 解析必须测）。
2. web 层按 `server/web/server_test.go` 的 httptest 模式补用例：`/api/workbench/info|refs` 正常/非 git 目录错误、envelope 格式。
3. 访问 `/workbench?path=<某真实仓库>`：显示 worktree 列表与 refs，布局骨架完整，各面板占位不报错；无 `path` 与非法 `path` 有清晰空态/错误态。
4. 前端 `pnpm -C web build` 通过，类型检查无错。

## 实施注意

- go 源码在 `server/`（goimports / go vet 都在那跑）；module path 是 `cube`。
- workbench 领域包不得 import `project` 包；`util/git` 只加读函数。
- 错误消息中文；构造函数命名遵循 `NewXxx`/`MakeXxx`/`InitXxx` 约定。
