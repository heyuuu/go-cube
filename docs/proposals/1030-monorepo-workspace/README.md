# monorepo workspace 打开支持

> **状态**：📝 已讨论定稿，待实施
>
> **关联**：[`1016-opener改造`](../archived/1016-opener改造/README.md)（opener 接口化 + role 体系，本提案不改 opener，仅消费其 open-dir 语义）；[`1008-workspace工作台`](../archived/1008-workspace工作台/README.md)（名字撞车但概念无关：那是 Web 工作台面板，本提案是 monorepo 子目录打开入口）；[`1032-worktree归并为项目打开目标`](../archived/1032-worktree归并为项目打开目标/README.md)（已落地：`Service.OpenTargets` 打开目标模型，workspace 作为第三来源纯增量插入）；[`1034-最近使用排序与usage统一`](../archived/1034-最近使用排序与usage统一/README.md)（已落地：`usage.Record.dir` 恒记实际打开目录绝对路径，本提案直接复用）。

## 背景与目标

cube 自身这类 monorepo 常有「用 goland 开 `server/`、用 cursor 开 `web/`」的需求——很多 IDE 的自动配置只有以子目录为根打开才生效。而 cube 的 project 判定以 `.git` 为前提，monorepo 整体是一个 project，子目录不是独立 project，现有打开流程只能打开项目根。

目标：

1. project 支持 **workspace 声明**：项目根 `.cube/cube.json` 声明一组「可作为打开入口的子目录」（名字 + 相对路径）；
2. **无显式声明时按标准 monorepo 声明文件自动探测**（pnpm-workspace.yaml / package.json workspaces），探测结果是正选来源而非仅编辑候选；
3. CLI 与 Web 打开流程接入：workspace 作为打开目标第三来源进入 `OpenTargets`，选择交互复用 1032 的既有泛化链路；
4. `cube workspace init`：交互式探测 → 挑选 → 固化为显式 `workspaces` 落盘。

## 数据模型（讨论定稿）

三层去处分明，判别式：**声明 or 推导；人的 or 机器的；仓库的 or 个人的**。

| 层 | 内容 | 存哪 | 性质 |
|---|---|---|---|
| 人的声明（仓库事实） | workspace 成员清单、workspaceScanRule | `.cube/cube.json`，进 git | 持久、跟仓库走、跨机器共享 |
| 机器推导（结构探测） | 标准声明文件解析结果 | projcache 快照（git.json 后继） | 可丢弃、按路径 key、重命名即 miss 重扫 |
| 个人偏好（默认 opener 等） | 「这个项目永远用 goland 开」 | 第一期不做；将来走本地层（不进 git） | 本地、不进 cube.json |

关键取舍：

- **不变量的准确表述**：禁止在 settings.json / 全局**持久配置**里按项目路径记录 project 内容（目录重命名会失联留脏数据）；**短生命周期、自动刷新、可丢弃的 cache 不在此列**（git.json 先例）。project 自身的配置放项目内（`.cube/`）跟仓库走。
- 探测结果进 cache 快照（采集侧算一次，读路径纯快照），探测**函数**是纯函数（输入文件内容，输出成员目录）；修正发生在探测规则（代码）或 cube.json（声明），不发生在某仓库的缓存值上。
- 个人偏好不进 cube.json（提交即替协作者做选择），`.git/info/exclude` 式的 git 本地口子是将来候选。

## `.cube/cube.json` 文件设计

仓库根下单文件（linked worktree 各分支检出各自的 cube.json 内容，故 worktree 根有自己的 workspaces 是自然成立的；`path` 相对**各自所属的根**，不是相对主 git root）：

```json
{
  "workspaces": [
    { "name": "server", "path": "server" },
    { "name": "web 前端", "path": "web" }
  ],
  "workspaceScanRule": "pnpm,npm"
}
```

- `workspaces`：显式声明。`path` 相对所属根，解析时校验存在且在项目内（防 `../` 逃逸）；条目逐个校验，坏的跳过；
- `workspaceScanRule`：探测规则组合，逗号分隔按序生效（`"pnpm,npm"` = 先查 pnpm-workspace.yaml，没有再查 package.json workspaces），第一个命中的生效；
- **两字段互斥生效，`workspaces` 优先**。

### 解析优先级（单一定序）

1. cube.json 存在且 `workspaces` 字段存在 → 只用它。**空数组是显式声明「没有任何 workspace」**，不回落探测；条目全坏等价于空数组；
2. cube.json 不存在，或存在但无 `workspaces` 字段 → 探测：有 `workspaceScanRule` 用它，没有则用程序内 const 默认值（**`"pnpm,npm"` 起步**，其余探测器实现了也不默认开——误报率高，想要的人显式写 scanRule）；
3. 探测在采集侧执行（见下），结果进快照，读路径不重复探测。

### 探测器（正选来源，非仅候选）

业界无跨生态通用 monorepo 声明，各生态自带。cube 解析这些文件展开成员目录：首期 `pnpm`（pnpm-workspace.yaml）、`npm`（package.json workspaces，覆盖 yarn）。成员粒度是「包」，比「想开的目录」多（30 个包的 repo 只想开 apps/web），所以探测结果自动生效为打开目标全集；要裁剪就 `workspace init` / Web 编辑固化为显式 `workspaces`。

## 采集与读取：workspace 进 projcache（原 gitcache）

- **采集侧**（唯一写者，异步）：`collectEntry` 内读所属根的 `.cube/cube.json` 按上述优先级解析（声明优先、否则跑探测器），结果作为 `Workspaces` 字段进 entry——`collectWorktrees` 枚举 worktree 时同样读各 worktree 根的 cube.json。依赖保持无环：scan → cache key 集 → 采集（git 子进程 + cube.json/探测文件读取）→ 单一快照，所有读路径只读快照；
- **读侧**：`OpenTargets` = 四段排序（见下），`project/list` DTO 顺带 workspaces，前端 `projectTargets` 加一段；
- **滞后处理**：Web 保存 cube.json 的 API 与 `cube workspace init` 写盘后定向触发该项目重采集（复用 RefreshOne），不等 TTL；
- **gitcache 更名 projcache**：纯语义修改（包名/注释/`cacheVersion` 升 v3，文件名 git.json 是否随改实现期定）——快照自此含 workspace 信息，不再是纯 git 信息，名要副实。

## 打开目标：OpenTargets 第三来源

`OpenTarget` 增加 `Kind` 字段（`root` / `worktree` / `workspace`）——Kind 描述「目录以什么身份成为打开目标」：worktree 根是 `worktree`，workspace 子目录**不管长在主根还是 worktree 根下都是 `workspace`**（归属关系由排序分组的天然结构表达，前端按根分组渲染，不用 Kind 编码）。

排序（讨论定稿）：

```
主项目根目录 > 主项目 workspaces > 每个 worktree 根 > 该 worktree 的 workspaces
```

CLI（`pickOpenTarget`）与 alfred 的多目标选择是 1032 已泛化的链路，workspace 进 `OpenTargets` 后**自动获得选择步骤，无需插步改动**；非 TTY 报错文案覆盖。打开后 usage 记录沿用 1034 契约（project 恒记主项目路径，dir 记实际打开目录绝对路径，等于根由 RecordOpen 归一为空）——**零改动**。

## CLI：`cube workspace init`

交互式：探测（按当前生效的 scanRule 顺序）→ `tui.SelectItem` 挑选/裁剪成员 → 写盘 `.cube/cube.json` 的 **`workspaces`**（固化为显式清单；不提供只写 scanRule 的选项——init 的语义就是固化）→ 触发该项目重采集。新建目录/文件遵循 `.cube/` 目录形态。

## Web 出口

- 项目列表打开下拉接入 workspace 目标（前端 `projectTargets` 从 DTO workspaces 拼装，按 Kind/分组渲染）；
- cube.json 可视化编辑：含「检测到标准 monorepo 声明，导入为候选」入口（把探测结果固化）；**入口必须能从项目上下文直达**（cube.json 是仓库级事实，从全局 settings 页编辑会误导为个人配置）；保存后触发重采集；
- Web API 遵循规则 15：GET 查询 / POST 动作，语义进 API 名（如 `project/workspace/list`、`project/workspace/save`）。

## 不做的事

- **不改 opener 体系**：workspace 最终只产出一个目录路径，open-dir 语义不变，role/slot/executor 全不动；
- **不改 scan**：workspace 不是独立 project，不参与扫描判定与 tags；scan/gitcache 两套 cache 结构不动（合并为大 cache 曾讨论过，无需求牵引、刷新语义各异，不合并）；
- **不做语言/技术栈识别**（属推导层，另案）；
- **不做个人偏好层**（默认 opener per 项目，将来另案）；
- **不做 workspace 级别的更多编排**（任务、依赖图等）；
- **不做 git 采集子进程批量化优化**（与本期正交，另案：如 `status --porcelain=v2 --branch` 一次拿分支+dirty+ahead/behind、`for-each-ref --format=...%(upstream:trackshort)` 合并 rev-list，7~9 次调用可压至 ~4 次）。

## 实施顺序

> 逐步实施、逐步验收；每步可独立收工。

1. **gitcache → projcache 更名**（纯语义：包名/引用/注释/cacheVersion v3），全量测试绿；
2. cube.json 解析（两字段 + 优先级 + 逐条校验/降级语义）+ 探测器（pnpm/npm 纯函数）+ 单测（testfixture 建含子目录与声明文件的工程）；
3. 采集侧接入：`collectEntry`/`collectWorktrees` 组合 workspaces 进 entry，`OpenTarget.Kind` + 四段排序，CLI/alfred 自动获得选择步骤（补非 TTY 文案），全链路单测；
4. `cube workspace init` 命令；
5. Web API + 列表下拉接入 + cube.json 可视化编辑 + 保存触发重采集；
6. `docs/spec/现状.md`、AGENTS.md 相关段落同步，验收后归档提案。

## 验收标准

1. cube 仓库自身配置 `.cube/cube.json`（server / web 两个 workspace）后，`cube open` 与 Web 列表下拉可分别以子目录为根打开，普通项目打开流程不变；
2. 无 cube.json 的 pnpm monorepo：不改任何配置，打开目标自动含探测出的 workspace 成员；`cube workspace init` 可挑选固化；
3. 降级语义：`workspaces: []` 不回落探测；条目坏（路径逃逸/不存在）逐条跳过；坏 JSON 等价于无声明（走探测）；
4. 目录重命名后：cube.json 相对路径继续生效或按新声明生效，快照 miss 重扫，无脏数据残留；
5. `cd server && go vet ./... && go test ./...`、`pnpm -C web build` 通过；
6. `docs/spec/现状.md` 已同步（.cube/cube.json 机制 + projcache 更名）。
