# 工作台 git 树面板：commit 图 + 工作副本状态 + 选择交互

> **状态**：✅ 已实现（2026-08-18）
>
> **所属**：[`1008-workspace工作台` 总纲](../1008-workspace工作台/README.md)（先读总纲「已收敛的全局决策」）。
> **依赖**：[`1010-workbench基座`](../1010-workbench基座/README.md)（路由、面板骨架、API 注册模式已就绪）。

## 背景与目标

git 树面板是工作台的**默认入口面板**，取代 SourceTree 的核心视图。它回答两个问题：这个仓库的提交历史长什么样（commit 图），以及每个工作副本现在处于什么状态（分支/暂存区/脏文件）。同时它承载工作台的**核心交互——「选择」**：选中一个目标（分支/commit/worktree）→ 进入代码阅读；选中两个 → 进入 diff。

**本提案不做**：任何 git 写操作（总纲 MVP 纯读）、diff/代码阅读本身（1012/1013）、面板布局自定义（1015）。

## 方案

### 1. 数据接口（后端）

**commit 图分页**：`GET /api/workbench/commits?path=&ref=&cursor=&limit=`

- 实现：`git log` 带拓扑与分页。推荐 `git log --all --topo-order --pretty=<结构化格式> --skip=<cursor> -n <limit>`（或用 `--max-count` + 末条 sha 作 cursor，实施时选更稳的方案并在代码注释里写明取舍）。
- 每条 commit 返回：`sha`、`parents`、`shortMessage`、`author`、`date`、`refs`（该 commit 上挂的分支/tag，`git log --decorate` 解析）。
- 分支拓扑信息（哪条线合并进哪条）由前端根据 `parents` 渲染连线，后端不预计算图。
- 默认 `--all` 还是当前 HEAD 起单线，作为 query 参数（如 `scope=all|ref`）；大仓库首屏只取单线 + 懒展开是可接受的降级，实施时可调，但接口必须分页。
- worktree 各自检出的分支：从 `git worktree list` 结果（1010 已有）标注，不额外查。

**工作副本状态**：`GET /api/workbench/status?path=&dir=<工作副本目录>`（每个 worktree 目录一次请求；不指定 `dir` 时默认主目录）

- 内容：当前分支、ahead/behind（`util/git` 已有读能力，复用）、dirty 布尔、变更文件摘要（staged 数 / unstaged 数 / untracked 数，来自 `git status --porcelain` 解析）。
- **实时性**：不走 gitcache（总纲决策），直接调 git。前端用 TanStack Query 控制：面板聚焦时 refetch、提供手动刷新按钮、staleTime 设短（如 30s）。
- 每个状态项可加「刷新」粒度到整个工作副本，不做到单文件。

### 2. 面板 UI（前端，`web/src/pages/workbench/panels/git-tree/`）

布局自上而下两段：

```
┌─────────────────────┐
│ 工作副本状态区        │  ← worktree 分组：主目录 + 各 worktree
│ ▾ <主目录> develop ↑2│     每组显示 分支/ahead-behind/dirty 摘要
│ ▸ <wt-hotfix> hotfix │
├─────────────────────┤
│ commit 图            │  ← 拓扑连线 + 分支/tag 标签 + 无限滚动分页
│ ● abc1234 feat: ...  │
│ ●● def5678 fix: ...  │
└─────────────────────┘
```

- **工作副本状态区**：`git worktree list` 结果分组展示；每项显示该目录的状态（上面的 status 接口）。点击 worktree 名 = 选中该 worktree 为 TreeSource。
- **commit 图**：按 `parents` 画拓扑连线（MVP 可先做「缩进单线 + 合并点标记」的简化拓扑，不追求 SourceTree 级平行线；是否升级平行线留待使用反馈）。无限滚动加载下一页。分支/tag 用彩色标签（`refs` 字段）。
- **选择交互**（核心）：
  - 单选：点击 commit / 分支 / worktree 项 → 选中态写入 URL（`sourceType/sourceId`），内容区（1012 的代码阅读）随之切换。
  - 双选：按住修饰键（如 cmd/ctrl）点击第二个目标 → URL 写入 `leftType/leftId + rightType/rightId`，内容区切到 diff（1013）。两个目标任意组合：分支 vs 分支、commit vs worktree 目录等。
  - URL 参数驱动一切：刷新/分享后选择态恢复；面板自身只是 URL 的渲染者。
- 选中高亮、hover 提亮参考 `web/src/pages/md/` 树组件的既有风格。

### 3. URL 参数约定（本提案引入，1012/1013 复用）

```
/workbench?path=<主目录>
           &sourceType=commit|ref|worktree&sourceId=<sha|ref名|目录>
           &leftType=…&leftId=…&rightType=…&rightId=…
```

单选与双选互斥（有双选时忽略 source）。写一个 `web/src/pages/workbench/params.ts` 的读写工具模块统一管理，面板不得自行 `useSearchParams` 拼参数。

## 验收标准

1. `git log` 输出解析函数（结构化 pretty 格式解析、decorate refs 解析、porcelain status 解析）有表驱动测试（`server/workbench/` 或解析沉淀处）。
2. httptest 用例：commits 分页（cursor 翻页两次）、status 正常路径。
3. 打开 `/workbench?path=<真实多 worktree 仓库>`：状态区正确分组显示各副本状态；commit 图滚动加载；单选/双选交互符合上述定义，URL 随之变化且刷新可恢复。
4. `cd server && go vet ./... && go test ./...`、`pnpm -C web build` 通过。

## 实施注意

- 遵循 1010 的面板铁律：数据 hook 自包含（key 从 URL 参数派生），面板间只走 URL。
- 解析函数按项目规则优先沉淀为纯函数（可进 `util/git`，纯解析不读环境），表驱动测试。
- 错误消息中文；UI 用 Base UI 组件。
