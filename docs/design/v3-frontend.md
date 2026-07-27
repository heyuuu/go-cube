# Cube · v3 前端设计

> `cube server` 的 Web 前端设计。配套总体设计见 [v3-design.md](./v3-design.md)。
>
> **现状对齐**：本文基于 `develop` 分支真实代码。后端 huma 已落地（`web/server.go`，`/docs` Scalar、`OpenAPIJSON()` 可导 spec）、git 信息走 `project/gitcache` 异步缓存。**前端工程尚未创建**（`frontend/`、`web/dist/`、`go:embed` 均未接入，属本文档定义的目标）。当前后端 API 偏少（project 仅 `list/info/scan-rules/clone-rules`，全只读），**git 操作 / open / 批量等 API 尚未实现**——前端设计标注了这些 gap，按"前端能做的前做、缺的后端 API 留接口位"推进。
>
> ⚠️ **`openapi.json` 过期**：仓库根的 `openapi.json` 只含 2 个路由（`/api/project/info`、`/api/project/list`），**缺 scan-rules/clone-rules/opener/config 共 5 个**。它是手动 `cube server openapi` 生成的产物，新增路由后未重新生成。**codegen 前必须先跑 `cube server openapi --out openapi.json` 重新生成**，否则生成的 SDK 会缺方法。

## 一、技术栈

| 层 | 选型 | 说明 |
|----|------|------|
| 构建 | **Vite** | dev server proxy `/api` → `cube server`；build → `web/dist/` |
| 语言 | **TypeScript** | strict |
| 框架 | **React 18** | |
| 路由 | **react-router v6** | `/:domain/*` 顶层分段 |
| UI | **shadcn/ui** + Tailwind | 组件源码进仓，可改 |
| 服务端状态 | **TanStack Query v5** | 缓存 / 失效 / 轮询 |
| 客户端状态 | `useState` + Context | 侧栏折叠、勾选集合；**不引 zustand**（够用再加）|
| 调用层 | **OpenAPI codegen** | 后端 huma 可生成 `openapi.json`（`cube server openapi`），codegen 出 typed SDK。注意：仓库内 `openapi.json` 当前过期，codegen 前需重生 |
| 测试 | **Vitest** + Testing Library | |
| 嵌入 | `go:embed web/dist` | 单二进制分发 |

**不引入**：zustand（现阶段）、状态机库、虚拟化（量级不到）、SSR。

> 调用层说明：上一轮文档设想的"手写 fetch 先行、huma 就绪换 codegen"**已过时**——huma 现已落地，`openapi.json` 在仓里，前端直接 `openapi-typescript-codegen` 生成 SDK 即可，无需手写。

## 二、布局

B 端看板：左 sidebar + 右正文，无全局 header。

```
┌──────────────┬──────────────────────────────┐
│ cube         │                              │
│ [project ▾]  │   页内标题区(面包屑+action)   │
│ ───────────  │   ──────────────────────────  │
│ Projects     │                              │
│              │      <Outlet/> 正文           │
│              │                              │
│ ───────────  │                              │
│ [⚙]          │                              │
└──────────────┴──────────────────────────────┘
   sidebar              正文(无全局 header)
```

**三个结构决策（已定）**：

1. **全局 domain switcher**：sidebar 顶部卡片。现仅 `project`，未来 `db` 等。切换 → sidebar 换菜单 + 路由跳 `/{domain}`。加 domain = 加一个 switcher 项 + 一个子路由树。对应后端"每 domain 一个 `Handler`，`/api/{domain}/*` 前缀"。
2. **无全局 header**：cube 无用户/通知系统。改为每页正文顶部**页内标题区**（面包屑 + 页面 action）。domain switcher 放 sidebar 顶部。
3. **project sidebar 极简 1 项（当前）→ 2 项（批量就绪后）**：
   - 现在：`Projects`（→ `/project`）。仅此一项。
   - 后端批量 API 就绪后加：`Groups`（→ `/project/groups`，按 group 批量，对应未来 `POST /api/project/group/{g}/pull`）。

## 三、路由表

| 路由 | 页面 | 后端 API（现状 / 待补） |
|------|------|------------------------|
| `/` | redirect → `/project` | — |
| `/project` | `ProjectsListPage` | ✅ `GET /api/project/list`；筛选靠前端（无 ws 实体，按 `group` 字段聚合） |
| `/project/info` | `ProjectDrawer`（点行滑出，不占路由） | ✅ `GET /api/project/info?name=`；git 信息已在 `list` 的 `gitInfo` 里 |
| `/project/groups` | `GroupsPage`（**待后端批量 API**） | ⏳ 需 `POST /api/project/group/{g}/pull` 等 |
| `/project/p/:path+/diff` | `DiffPage`（**待后端 diff API**） | ⏳ 需 `GET /api/project/diff?path=` |

> **path 处理**：`Project.path` 是绝对路径（如 `/Users/heyu/Code/cube`），是后端主键。`name` 是展示名 `group:subpath`。前端列表用 `path` 作 React key；详情/diff 路由用 `:path+`（react-router v6 捕获剩余段）+ `encodeURIComponent`。

**无 project 详情路由**：详情走右侧 Drawer，点行滑出，上下文不丢。唯独 diff 内容重、需可分享 URL，给独立页（待后端 diff API）。

## 四、页面分解

### 4.1 Projects 列表页 `/project`（核心页，当前可完整落地）

**扁平表格**（`group` 既列又筛，无 workspace 实体，前端按 `group` 字段聚合）：

```
[🔍 搜索]  [group: 全部 ▾]  [git: 全部 ▾]          (批量就绪后: [N selected → Pull  Open ▾])
┌───┬──────────┬─────────┬───────────────┬────────┬─────┐
│   │ name     │ group   │ git           │ tags   │     │
├───┼──────────┼─────────┼───────────────┼────────┼─────┤
│   │ github:cube │ github│ ahead 2       │ git    │ ⋯   │
│   │ github:foo  │ github│ clean         │ git    │ ⋯   │
│   │ pers:bar    │ pers  │ dirty         │ git    │ ⋯   │
└───┴──────────┴─────────┴───────────────┴────────┴─────┘
```

- **数据源**：`GET /api/project/list` 一次性返回所有 project（含 `gitInfo` 快照）。后端 `Service.Projects()` 走 `scanCache` + `gitcache`，几乎零开销。
- **搜索/筛选**：当前后端 `list` 不带 query 参数 → **前端 filter**（`fuse.js` 或简单子串；量级几十个，前端做无压力）。`Search()` 后端能力留给 CLI/alfred 路径用。
- **git 列**：渲染 `gitInfo` 的 `ahead/behind/dirty/branch`（badge 形式）。**实时性复用后端机制**（见第六节）。
- **勾选 + 批量**：当前后端**无批量/pull API**，BatchActionBar 先做 UI 骨架 + disabled 状态，后端补 API 后激活。
- **行操作 `⋯`**：`Open ▾`（需 `POST /api/project/open`，**待补**）、`Pull`（**待补**）。当前可只放"复制路径"等纯前端能力占位。

### 4.2 Project 详情 Drawer（点行滑出，当前可落地）

右侧 `Sheet`，内容来自 `GET /api/project/info?name=` + list 已带的 `gitInfo`：

- **Overview**：path / repoUrl / group / tags。
- **Git**：currentBranch / defaultBranch / ahead / behind / dirty / collectedAt（来自 `gitcache.Entry`）。
- **Worktree**：占位块（后端 worktree 能力未实现）。
- **动作**：`Open ▾` / `Pull` / `View diff →`（均待后端 API，先 disabled/隐藏）。

### 4.3 Groups 页（待后端批量 API）

按 `group` 聚合（替代旧"workspace 概览"），每组一行：group 名 / 扫描根 / 项目数 / `[全部 pull]`。需后端补 group 维度的批量 API。

### 4.4 Diff 页（待后端 diff API）

独立路由 `/project/p/:path+/diff`，展示 `GET /api/project/diff?path=`。

## 五、组件树

```
<App>
  <QueryClientProvider>
    <AppLayout>
      <Sidebar>
        <DomainSwitcher/>
        <NavMenu domain="project"/>
      </Sidebar>
      <main><Outlet/></main>
    </AppLayout>
    <Toaster/>
  </QueryClientProvider>
</App>

ProjectsListPage
 ├─ PageHeader("Projects")
 ├─ ProjectsToolbar (搜索 + group 筛选 + git 筛选)
 ├─ BatchActionBar  (待后端 API，先 disabled)
 └─ ProjectsTable → ProjectRow → onClick 开 ProjectDrawer
ProjectDrawer(Sheet)
 ├─ OverviewBlock / GitStatusBlock / WorktreeBlock(占位) / ActionBar(待 API)
```

## 六、数据流与实时性（关键：复用 gitcache）

### Query keys

```
['project', 'projects']                  // 列表（含 gitInfo 快照）
['project', 'projects', name]            // 单项目详情
['project', 'scan-rules']
['project', 'clone-rules']
['opener', 'openers']
```

### 实时性：复用后端 gitcache，前端只做轻轮询

后端已有成熟的 git 信息异步缓存机制（`project/gitcache`）：
- `Service.GitInfo(path)` 读**缓存快照**（不阻塞、不触发采集）。
- `Service.TriggerAsyncRefresh()` fork 子进程后台采集，TTL 1 分钟内不重复触发。
- `list`/`info` 读命令返回前会调 `TriggerAsyncRefresh()` 触发后台刷新。

**前端策略**：
- 列表/详情的 `gitInfo` 就是后端缓存快照，TanStack Query 用 `refetchInterval`（如 30s）重新 `GET /api/project/list` 即可拿到**已被后台子进程刷新**的新快照。
- **前端不自己采集 git 信息，也不需要 SSE**。后端 fork 子进程采集 + 前端轮询拉快照，是已验证的成熟链路。
- 预留 `useEventStream` hook 位（空实现），仅当未来"批量 pull 进度"等场景真需要实时推送时再填，调用点签名不变。

### Mutations → 失效（API 就绪后）

| mutation | 失效 |
|----------|------|
| `openProject(path, app)` | 不失效 |
| `pullProject(path)` | `['project','projects',path]` + 整表（gitInfo 变）|
| `pullGroup(group)` | 该 group 下所有 project |

## 七、目录结构

前端源码独立顶层 `frontend/`（**待创建**），构建输出 `web/dist/`（**待创建**）供 `go:embed`（**待接入**）。三者均属 M4 前端工程范畴，当前不存在。

```
cube/
├── frontend/                      # ★ 前端工程
│   ├── index.html
│   ├── package.json / tsconfig.json / vite.config.ts
│   └── src/
│       ├── main.tsx / App.tsx
│       ├── api/
│       │   ├── gen/               # ★ openapi-typescript-codegen 生成产物（huma spec → SDK）
│       │   └── client.ts          # 配置 baseURL + ApiOutput envelope unwrap
│       ├── routes/project/        # project domain 页面
│       │   ├── projects-list-page.tsx
│       │   ├── project-drawer.tsx
│       │   ├── groups-page.tsx       # 待后端 API
│       │   └── project-diff-page.tsx # 待后端 API
│       ├── components/
│       │   ├── layout/            # AppLayout/Sidebar/DomainSwitcher/NavMenu
│       │   ├── project/           # ProjectsTable/ProjectRow/BatchActionBar/GitBadge
│       │   └── ui/                # shadcn 组件源码（进仓）
│       ├── hooks/
│       │   ├── use-projects.ts
│       │   └── use-event-stream.ts  # SSE 占位（空实现）
│       └── lib/{query-client,types}.ts
├── web/
│   ├── server.go ...              # Go 后端（huma 已落地）
│   └── dist/                      # ★ frontend build 产物 → go:embed
└── openapi.json                   # huma 生成的 spec（codegen 输入）
```

**构建接线**（目标，待 M4 实现）：`frontend/` 跑 `vite build`（outDir `../web/dist`），Go 侧 `go:embed web/dist`。dev 时 vite proxy `/api` → `localhost:7331`。codegen 前先跑 `cube server openapi --out openapi.json`（仓库内的已过期），再 `openapi-typescript-codegen` → `frontend/src/api/gen/`。

**envelope 处理**：后端统一 `ApiOutput{ok,message,data}` 包裹。`api/client.ts` 在 generated SDK 之上包一层，统一 unwrap：`ok` 为 false 时 throw（让 TanStack Query 走 error 分支），否则返回 `data`。业务代码只见纯数据。

## 八、加一个新 domain 的前端步骤

对应 v3-design 多 domain 扩展模型。前端侧加 domain（如 `db`）= 纯加法：

1. 后端加 `web/db_api.go`（一个 `Handler`）→ `openapi.json` 自动含新路由 → 重新 codegen。
2. `frontend/src/routes/db/`：db 子路由树 + 页面。
3. `frontend/src/components/db/`：db 专用组件。
4. `DomainSwitcher` 注册表加 `{ id:'db', label:'Database' }`。
5. `NavMenu` 的 `domain` 分支加 db 菜单项。

现有 project 代码零改动。与后端"加 domain = 多传一个 `Handler` 给 `NewServer`"对称。

## 九、前端落地阶段（对齐后端 API 进度）

| 阶段 | 前端做 | 依赖的后端 API |
|------|--------|----------------|
| **F1 骨架** | 工程脚手架 + 布局 + 路由 + Query client + codegen 接线 + ProjectsListPage（只读）+ ProjectDrawer（只读） | ✅ 现有 `list/info/scan-rules/clone-rules` 已够 |
| **F2 操作** | 行操作 Open▾ / Pull 激活 + BatchActionBar 激活 | ⏳ 待 `POST /api/project/open`、`POST /api/project/pull`、批量 |
| **F3 diff** | DiffPage | ⏳ 待 `GET /api/project/diff` |
| **F4 worktree** | Drawer Worktree 块 | ⏳ 待 worktree 能力 |

**F1 当前可立即开工**，不阻塞后端。

## 十、默认决策记录

| 项 | 默认 | 理由 |
|----|------|------|
| 搜索 | 前端 filter | 当前 `list` 不带 query；量级几十个前端无压力。后端 `Search()` 留给 CLI |
| 主题 | shadcn 默认 light/dark | 个人工具，一键切 |
| 列表虚拟化 | 不引入 | 量级不到；上百再上 TanStack Virtual |
| 客户端状态库 | 不引 zustand | `useState`+Context 够用 |
| 轮询间隔 | ~30s | 复用后端 gitcache TTL（1min）节奏，前端轮询拉快照 |
| 实时推送 | 不做 SSE | 后端 fork 子进程采集已够；仅留 hook 占位 |

## 十一、待定（与后端联动收敛）

- group 维度批量 API 的路径形态（RESTful `/api/project/group/{g}/pull` vs 动词式 `/api/project/pull-group`）——跟随当前后端"动词式 + envelope"风格，但批量语义待定。
- 批量操作并发/失败处理的前端表现——与后端策略联动。
- diff API 的返回结构（patch 文本 vs 结构化 hunks）——影响前端 diff 渲染组件选型。
- `open` API 是否需要返回结果（当前 opener 是 fire-and-forget）。
