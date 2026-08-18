# workbench 基座：路由 + TreeSource 抽象 + 核心 API + 面板骨架

> **状态**：✅ 已实现（2026-08-18）
>
> **所属**：[`1008-workspace工作台` 总纲](../1008-workspace工作台/README.md)（实施前先读总纲「已收敛的全局决策」，本提案遵守其中全部约定）。
> **依赖**：无（工作台第一个子提案，1011-1015 都依赖本提案的产出）。

## 背景与目标

工作台是 cube 的独立页面，输入一个本机 git 目录路径，聚合 git 可视化 / 代码阅读 / diff / PTY。本提案只做**基座**：路由、TreeSource 统一抽象、后端核心 API 与领域包、前端固定布局的面板骨架。做完后：访问 `/workbench?path=<git目录>` 能进入一个可用的空工作台，显示项目基本信息与面板占位。

**本提案不做**：commit 图渲染、文件浏览、diff、PTY（分别是 1011-1014）、任何 git 写操作、布局自定义（1015）。

## 方案

### 1. 后端：workbench 领域包

按 AGENTS.md「五处加法」流程新增 domain（本提案无 CLI 子命令、暂无 config 节，实际加三处：领域包 / web handler / app 装配）：

- **领域包 `server/workbench/`**：不依赖 `project` 包（工作台解耦 project scan，输入是任意 git 目录）。git 读能力按项目规则沉淀到 `util/git`（如缺失），workbench 包只做编排：
  - `service.go` —— `Service` 结构 + 构造（`NewService`），聚合下面各文件的能力。
  - `source.go` —— `TreeSource` 类型与解析。
  - `tree.go` —— 目录树 / 文件内容读取。
  - `diff.go` —— 本提案只定义接口形态，具体 diff 实现在 1013 落地（可先返回空实现或 `errors.New("diff 尚未实现")`，接口签名先定死）。
  - `info.go` —— 项目信息（worktree 列表、refs 列表）。
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

HTTP 传参用扁平 query 参数：`sourceType` + `sourceId`（双源场景 `leftType/leftId` + `rightType/rightId`）。`path` 始终是项目主目录（用户进来的目录），worktree 的 id 是它自己的绝对路径。

核心接口（huma path 即 operationId 来源，注意命名）：

| Method | Path | 说明 |
|---|---|---|
| GET | `/api/workbench/info` | `?path=` → 主目录是否 git 仓库、`git worktree list` 结果、默认分支 |
| GET | `/api/workbench/refs` | `?path=` → 分支 + tag 列表（`util/git` 已有 Branches/Tags 能力则复用） |
| GET | `/api/workbench/tree` | `?path=&sourceType=&sourceId=&dir=` → 目录树一层（或整树，见下） |
| GET | `/api/workbench/file` | `?path=&sourceType=&sourceId=&file=` → 文件内容（文本，二进制返回标记） |
| GET | `/api/workbench/diff` | `?path=&leftType=&leftId=&rightType=&rightId=&…filters` → 目录级 diff 摘要（1013 实现） |
| GET | `/api/workbench/file-diff` | `?path=&left…&right…&file=` → 单文件 diff（1013 实现） |
| GET | `/api/workbench/commits` | `?path=&ref=&cursor=&limit=` → commit 图分页（1011 实现，本提案可先定签名） |

错误语义：`path` 不是 git 仓库 → 中文错误（如 `"path 不是 git 仓库: path=..."`，遵循项目错误消息规则）；`TreeSource` 解析失败/对象不存在 → 404 语义错误信息。

**安全注脚**（总纲已定）：本组接口可读任意本机 git 目录，MVP 接受此边界，不做路径白名单。

### 3. 前端：路由与面板骨架

- **路由**：`web/src/App.tsx` 加 `<Route path="/workbench" element={<WorkbenchPage />} />`（独立于主应用 `Layout`，同 `/md` 的做法——工作台自带整体布局）。页面在 `web/src/pages/workbench/`。
- **URL 参数**：`?path=<git目录绝对路径>`。本提案只消费 `path`；选中态参数（`source`/`left`/`right` 等）在 1011/1012 引入，命名届时定为 `sourceType/sourceId` 系列，与 API 参数一致。
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
- **空态处理**：无 `path` 参数 → 引导输入目录路径的输入框；`path` 非 git 目录 → 错误提示。
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
