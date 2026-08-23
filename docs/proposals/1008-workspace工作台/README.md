# workspace 工作台（总纲）

> **状态**：📋 已完成需求拆解，按子提案顺序实施（2026-08-18 讨论定稿）
>
> 本文是工作台需求群的**总纲**：沉淀已收敛的全局决策，索引全部子提案。各子功能的设计细节在子提案目录里，单独可执行。
> 设计方法论讨论（工具选型 / 设计稿即契约）见 [`design-tooling.md`](./design-tooling.md)，不影响实施。

## 需求本质

**多窗口是困境，不是目标。** 痛点：同时开发时桌面散落多个窗口（IDE / SourceTree / 终端 / 浏览器 / Finder），切换累、心智负担重。目标：在 cube 的一个 Web 界面里聚合更多日常信息（git 可视化 / 代码阅读 / 终端），减少窗口数量，只在需要深度操作时才切出去开专门工具。

工作台要取代的核心场景是 **SourceTree**：commit 树、当前状态、分支/commit 对比；外加代码阅读（轻编辑）与命令行 PTY。

## 已收敛的全局决策（2026-08-18 讨论，子提案必须遵守）

### 定位与入口

- **以单个 git 目录为入口参数**。不要求该目录在 project scan 管理范围内——输入任意 git 目录即可（MVP 先解耦，若日后需要再绑回，保留兼容）。
- **独立路由页面**：`/workbench`，路径通过 URL 参数传入。主目录和 worktree 目录进来是同一个项目内容（worktree 归属用 `git worktree list` 现场发现，不依赖扫描数据）。
- **默认入口面板 = git 树面板**（commit 图 + 各工作副本状态）。理由：它是项目的「主页状态」（SourceTree 的默认视图），且 diff / 代码阅读通常从 git 上下文「进入」——git 面板是 hub，其余面板是可达的叶子。

### 核心交互：「选择」模型

- 选中**一个**目标 → 看它的文件结构 / 代码。
- 选中**两个**目标 → 看它们的 diff。
- **可选目标统一为三类：分支（ref）、commit、worktree 工作副本**，任意组合（如 worktree 目录 vs 某 commit）。

### 数据层

- **不走 gitcache**。工作台的 git 信息全部直接调 git 获取；实时性与内容范围都与 gitcache 不同，不混用，避免 gitcache 受影响大改。
- 前端缓存用 **TanStack Query**（key 去重 + staleTime + 手动 invalidate）足够；后端 MVP 不建缓存。唯一注意点：commit 全量图可能秒级，接口需支持分页/限量加载，而不是服务端缓存。
- worktree 状态按目录独立采集（每个工作副本有独立 HEAD/暂存区/脏状态，共享同一对象库）。

### 能力边界（MVP）

- **纯读**：所有 git 写操作（commit / checkout / pull-push / stage）一律不做，后续另立提案。
- **轻编辑**：只改工作区文件；默认只读模式，开启编辑和保存**都需要弹窗确认**。
- 编辑器用 **CodeMirror 6**（不用 Monaco，已在其他讨论定案）。
- diff 是 Beyond Compare 级别：两个目标的**目录树对比 + 文件级对比**，带筛选项。原设想的「包含 `.gitignore` 忽略文件」开关**已砍掉**——产品不考虑 ignored 文件；但对比 worktree 工作区（含未提交/untracked）仍要求文件系统扫描对比模式。
- PTY：后端 WebSocket + `creack/pty`，前端 **xterm.js**（term.js 是弃用前身）。技术成熟、边界清晰，作为独立面板。

### 架构形态

- **面板化**：git 树 / 代码阅读 / diff / PTY 全部做成**可组装的独立面板**，后续支持同一页面自定义布局（同时看 git + 代码 + 终端）。
- **URL 是面板间唯一总线**：每个面板自带数据 hook（Query key 自包含，从 TreeSource/路径参数派生，不依赖父级传选中态），面板间通信只通过 URL search params。这同时保证可分享 / 刷新恢复。子提案从第一天就按此写，避免整合时重构。
- MVP 交付固定默认布局（左 git 树 + 右内容区 + 底部 PTY 抽屉），不做任何布局自定义代码（那是最后的整合提案）。

### 安全注脚

「任意 git 目录」意味着文件读取接口可读本机任意 git 目录下的文件。本地 localhost 无鉴权服务是 cube 现状，风险可接受，MVP 记录此已知边界、不做限制。

## 核心抽象：TreeSource

commit / 分支 / worktree 目录都可被「选中」和「对比」，统一抽象为 **TreeSource**：

```
TreeSource = { type: "commit" | "ref" | "worktree", id: string }
  commit:   id = commit sha
  ref:      id = 分支名 / tag 等 ref 名
  worktree: id = 工作副本目录绝对路径（当前文件状态，含未提交改动）
```

由此工作台后端 API 面收敛为（传参定稿为单参数序列化 `source=type://id`，双源 `left`/`right`）：

| 接口 | 作用 |
|---|---|
| `tree(path, source)` | 某 TreeSource 的文件清单（全量扁平相对路径，前端组树） |
| `readFile(path, source, file)` | 读某 TreeSource 下某文件内容 |
| `saveFile(path, dir, file)` | 保存工作区文件（唯一写，仅 worktree 源） |
| `diffTrees(path, left, right, filters)` | 两个 TreeSource 的目录级对比 |
| `readFileDiff(path, left, right, file)` | 两个 TreeSource 的单文件 diff |
| `commits(path, cursor, limit)` | commit 图分页（恒 `--all`） |
| `worktrees(path)` / `refs(path)` | 工作副本状态聚合 / 分支 tag 列表 |
| `changes(path, dir)` | worktree 变更文件清单（差异模式树） |
| `pty(path)` | WebSocket 终端 |

- 代码阅读 = 单 TreeSource 浏览；diff = 双 TreeSource 对比；编辑器/diff 里「切换分支」= 换 TreeSource。
- 前后端统一用 `source` / `left` / `right`（`type://id` 格式）传 TreeSource。

## 子提案索引（按实施顺序）

| # | 提案 | 内容 | 依赖 |
|---|---|---|---|
| 1 | [`1010-workbench基座`](../archived/1010-workbench基座/README.md) ✅ | 路由 `/workbench`、TreeSource 抽象与核心 API、后端 workbench 领域包、固定布局面板骨架 | 无 |
| 2 | [`1011-工作台git树面板`](../archived/1011-工作台git树面板/README.md) ✅ | commit 图（分页）、工作副本状态区（worktree 分组）、单选/双选交互 | 1010 |
| 3 | [`1012-工作台代码阅读面板`](../1012-工作台代码阅读面板/README.md) 🔶 | 虚拟树 + 真实树、CodeMirror6 只读 + 确认式轻编辑（目录树已验收，代码展示待开发验收） | 1010 |
| 4 | [`1013-工作台diff面板`](../1013-工作台diff面板/README.md) | 双 TreeSource 对比：git 模式 + fs 扫描模式、目录级 + 文件级 | 1010、1012（复用文件查看底座） |
| 5 | [`1014-工作台PTY面板`](../1014-工作台PTY面板/README.md) | WebSocket + pty + xterm.js、会话生命周期 | 1010（仅路由；可任意插队） |
| 6 | [`1015-工作台面板组装`](../archived/1015-工作台面板组装/README.md) ✅ | 自定义布局、面板注册、URL 状态总线收口 | 2-5 全部完成 |

每个子提案单独可交付、可验收。1010 做完是能进的空工作台；1011 做完已可用（取代 SourceTree 的核心）；1013-1015 任何一个延期不伤其他。

**必须由基座守住的两点**（否则整合提案返工）：

1. TreeSource 统一抽象先落地，后续面板不得各自造选中态模型。
2. 面板从第一天就自带数据 hook、只通过 URL 通信。

## 实施前必读

- [`AGENTS.md`](../../AGENTS.md) —— 分层依赖纪律、五处加法流程、编码规则、前端 Base UI（非 Radix）注意点。
- [`docs/spec/现状.md`](../../spec/现状.md) —— 架构 / API / 前端工程现状（与代码冲突时以代码为准）。
