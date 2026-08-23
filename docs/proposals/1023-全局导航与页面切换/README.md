# 全局导航与页面切换

> **状态**：📝 待确认（2026-08-23）
>
> **范围**：仅前端（`web/`），无后端改动。

## 背景与问题

前端目前有四类独立页面，但导航结构是割裂的：

- `/projects`、`/config` 挂在 `Layout`（w-52 文字侧栏）下；
- `/workbench` 完全独立于 `Layout`（自带整屏多面板布局）；
- `/md` 完全独立（文档查看器性质）。

后果：一旦进入 workbench 或 md，**没有任何全局入口回主后台**，只能改 URL 或浏览器返回。同时三者的产品心智其实不同——Projects 是「主后台/家」，workbench 是「沉浸在某一个项目里」，md 是「从 workbench 弹出的独立文档查看器」——现状把这三种心智都用「脱离全局壳」表达了，导致全局切换缺失。

## 心智模型（本提案的设计基准）

- **Projects = 家**：项目列表是所有工作的入口，全局导航永远能一键回这里。
- **workbench = 项目内部**：从 Projects 行下钻进去的沉浸界面。沉浸感的正确表达方式是「入口在项目上 + 内部不堆全局导航」，而不是「脱离全局壳导致回不去」。
- **md = 查看器**：类似 `/docs` 的从属页面，不套全局壳，但要有明确的「归宿」返回。

## 方案（按优先级分四步，1 是必须，2-4 可分期）

### 1. 全局壳：48px 图标栏（icon rail）替代现有侧栏【核心】

把 `Layout` 的 w-52 文字侧栏改为全局细栏杆，**所有页面（含 workbench）共享**：

```
┌──┬─────────────────────────────┐
│▣ │                             │
│  │                             │
│▣ │      当前页面内容            │
│▣ │      (projects/workbench…)  │
│  │                             │
│  │                             │
│⚙ │                             │
│☾ │                             │
└──┴─────────────────────────────┘
 48px；图标 hover 出 tooltip 标签
```

- 栏内项目：Projects（`/projects`，未来可带脏仓库数 badge 强化「家」心智）、Workbench（`/workbench`，记住上次目录）。
- 栏底固定区：Config（`/config`）、主题切换（沿用现有 ⌘D）。「API Docs」外链可收进 Config 页或保留图标。
- `/workbench` 从独立路由挪进全局壳（`App.tsx` 路由调整 + workbench 自身布局让出左侧 48px）。md **不进壳**（见第 3 点）。
- 成本评估：48px 对 workbench 多面板布局基本无感；换来任何页面一键回 Projects。
- 原 `Layout` 侧栏的文字标签改为 tooltip（shadcn tooltip，Base UI 版）。

### 2. workbench 内的项目切换：顶栏项目下拉

沉浸场景里真正的切换痛点是「换一个项目的工作台」——现状要退回列表再下钻。在 workbench 顶栏的项目名处做下拉（复用现有 projects 列表数据源），直接横跳到其他项目的 workbench。全局 rail 管「页面级」切换，这里管「项目级」切换。

### 3. md：保持壳外，加轻量返回

- md 常是「在 workbench 点开文件」打开的（甚至新 tab），套全局 rail 反而奇怪，维持独立路由。
- 加一个极轻的返回逻辑：同 tab 导航进来时（`location.key !== 'default'` 或显式 `from` 参数）左上角浮一个返回按钮；直接开 URL 进来的不显示。

### 4. ⌘K 命令面板（可后置成独立提案）

全局命令面板：搜项目名直达 workbench、切换页面、切主题。个人工具 + CLI 优先的气质很适合；rail 管可见性，palette 管速度。**本提案不含此项**，仅记录方向，页数再多时也不用挤 rail。

## 不做的事

- 不引入 tab 系统（页面级多 tab 是另一个量级的复杂度）。
- 不做 workbench 内的全局导航重复入口（顶栏只放项目切换，不放页面切换——那是 rail 的职责）。
- 不引入新路由库或嵌套路由重构，只调整 `App.tsx` 的挂载关系。

## 验收标准

1. 从任意页面（projects / workbench / config）通过左侧 rail 一键切换到其他页面，当前页高亮。
2. workbench 进入全局壳后，多面板布局（树/diff/PTY/代码阅读）无错位，48px 占位无感。
3. md 独立性不变：直接打开 URL 无返回按钮；从 workbench 内点开（同 tab）有返回按钮。
4. `pnpm -C web build` 通过；`make build` 产物正常（go:embed 前端）。

## 实施注意

- shadcn 基于 Base UI：tooltip 用 `@base-ui/react/tooltip` 命名空间写法，勿照搬 Radix。
- workbench 挪进壳时注意它自带的 `h-dvh` 布局与 `Outlet` 滚动容器的衔接（workbench 应占满、不吃 Layout 的 `max-w-7xl` 收敛——壳需要区分「收敛型内容页」与「铺满型应用页」两种 main 形态，可按路由约定或 Outlet context）。
