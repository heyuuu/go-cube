# monorepo workspace 打开支持

> **状态**：📝 待评审
>
> **关联**：[`1016-opener改造`](../archived/1016-opener改造/README.md)（opener 接口化 + role 体系，本提案不改 opener，仅消费其 open-dir 语义）；[`1008-workspace工作台`](../archived/1008-workspace工作台/README.md)（名字撞车但概念无关：那是 Web 工作台面板，本提案是 monorepo 子目录打开入口）；[`1029-scan-clone规则迁移settings`](../1029-scan-clone规则迁移settings/README.md)（同为 settings/配置边界相关，见「不做的事」）；[`1032-worktree归并为项目打开目标`](../1032-worktree归并为项目打开目标/README.md)（**顺序依赖：1032 先行**——它确立「project → 打开目标」模型，本提案实施顺序 3/4（打开流程接入）待其落地后实施，地基部分（cube.json 解析 / 候选探测）可并行）。

## 背景与目标

cube 自身这类 monorepo 常有「用 goland 开 `server/`、用 cursor 开 `web/`」的需求——很多 IDE 的自动配置只有以子目录为根打开才生效。而 cube 的 project 判定以 `.git` 为前提，monorepo 整体是一个 project，子目录不是独立 project，现有打开流程只能打开项目根。

目标：

1. project 支持 **workspace 声明**：项目根 `.cube/cube.json` 声明一组「可作为打开入口的子目录」（名字 + 相对路径）；
2. CLI 与 Web 打开流程接入：有声明的项目多一步 workspace 选择（含根目录兜底），声明了默认 opener 的可跳过 opener 选择；
3. 提供基于标准 monorepo 声明文件（pnpm-workspace.yaml 等）的**候选探测**，辅助生成 cube.json，探测结果不落盘。

## 数据模型（讨论定稿）

三层去处分明，判别式：**声明 or 推导；人的 or 机器的；仓库的 or 个人的**。

| 层 | 内容 | 存哪 | 性质 |
|---|---|---|---|
| 人的声明（仓库事实） | workspace 成员清单 | `.cube/cube.json`，进 git | 持久、跟仓库走、跨机器共享 |
| 机器推导（结构探测、语言识别） | 标准声明文件解析结果等 | 全局 cache 快照（或不落盘，现算） | 可丢弃、按路径 key、重命名即 miss 重扫 |
| 个人偏好（默认 opener 等） | 「这个项目永远用 goland 开」 | 第一期不做；将来走本地层（不跟 git） | 本地、不进 cube.json |

关键取舍（已写入 AGENTS.md「关键机制」）：

- **全量 project 列表永不落盘**，project 一切由 scan-rule 推导。因此**禁止在 settings.json / 全局配置里按项目路径记录 project 内容**——目录重命名会失联留脏数据。project 自身的配置放项目内（`.cube/`）跟仓库走。
- cache 按路径 key 没问题（可丢弃快照，同 `cache/git.json` 先例）；配置按路径 key 不行。两者本质区别是「可不可丢弃」。
- 语言识别这类推导**永远现算**（贵的进 cache），修正发生在探测规则（代码）或 cube.json（声明），不发生在某个仓库的缓存值里。cube.json 只存「代码读不出来、且换个人打开也成立」的内容。
- 个人偏好不进 cube.json（提交即替协作者做选择），`.git/info/exclude` 式的 git 本地口子是将来候选，不在本提案。

## 方案

### 1. `.cube/cube.json` 文件设计

仓库根（git root）下单文件，第一期只有一个节：

```json
{
  "workspaces": [
    { "name": "server", "path": "server" },
    { "name": "web 前端", "path": "web" }
  ]
}
```

- `path` 为相对 git root 的路径，解析时校验存在且在项目内（防 `../` 逃逸）；
- `.cube/` 以目录形态起步（预留 scan 调优等项目内声明的演进位），本期只有 `cube.json` 一个文件；
- 文件随 git 管理（声明的是仓库结构事实，该共享）；
- `.gitignore` / 内部工具不受影响；无 `.cube/` 的项目行为完全不变。

### 2. 标准声明文件探测（生成候选，不落盘）

业界无跨生态通用 monorepo 声明，各生态自带（npm/yarn `package.json` workspaces、`pnpm-workspace.yaml`、Cargo 根 `[workspace]`、`go.work`、`deno.json` workspaces）。cube 读取这些文件**展开成员目录作为候选列表**供用户挑选/裁剪，确认后写入 cube.json——成员粒度是「包」，比「想开的目录」多（30 个包的 repo 只开 apps/web），所以只做候选不做默认值。

探测规则细节（如 turbo 根 package.json 是编排清单而非 js 项目证据）属代码实现，全局共享修正，不逐仓库配置。

### 3. project 层查询

`project.Service` 新增 workspace 查询：读 project 根 `.cube/cube.json` → 解析校验 → 返回 workspace 列表（或空，表示单目录项目）。读取时机与缓存策略实现期定（倾向直读不缓存，文件极小）。

### 4. CLI 出口

`cube open` 交互：选定 project 后，若其有 workspace 声明，插一步 `tui.SelectItem` 选择 workspace（含「根目录」选项），再走现有 opener 选择；无声明则流程不变。alfred 出口同链路受益。

### 5. Web 出口

- 项目页 / workbench 打开入口接入 workspace 选择；
- settings 页或项目页提供 cube.json 的可视化编辑（含「检测到标准 monorepo 声明，导入为候选」入口）——具体落点（哪个页、什么交互）实装时按 1025 定型模板定；
- Web API 遵循规则 15：GET 查询 / POST 动作，语义进 API 名（如 `project/workspace/list`、`project/workspace/save`）。

## 不做的事

- **不改 opener 体系**：workspace 最终只产出一个目录路径，open-dir 语义不变，role/slot/executor 全不动；
- **不改 scan**：workspace 不是独立 project（无 `.git`），只是挂在 project 上的附加元数据，不参与扫描判定与 tags；
- **不做语言/技术栈识别落盘**（属推导层，另案）；
- **不做个人偏好层**（默认 opener per 项目，将来另案）；
- **不做 workspace 级别的更多编排**（任务、依赖图等，超范围）。

## 实施顺序

> 逐步实施、逐步验收；每步可独立收工。

1. `.cube/cube.json` 解析 + project 层 workspace 查询 + 单测（testfixture 建含子目录的工程）；
2. 标准声明文件探测（候选生成）+ 单测；
3. CLI `cube open` 插入 workspace 选择步骤；打开记录 usage 时带 `dir`（实际打开目录的**绝对路径**），`project` 恒记主项目路径（格式见 [`1034-最近使用排序与usage统一`](../1034-最近使用排序与usage统一/README.md)——`Record.dir` 字段已随 1034 落地预留。~~原定的 `subPath` 相对路径方案已废弃~~：worktree 目标（1032）物理上在主项目根之外，相对路径须带 `../` 前缀、无意义，故统一为单一 `dir` 字段恒记绝对路径，避免「有时相对有时绝对」的分支语义）；
4. Web API + 打开入口接入 + cube.json 可视化编辑（usage 记录同样带 `dir`）；
5. `docs/spec/现状.md` 同步，验收后归档提案。

## 验收标准

1. cube 仓库自身配置 `.cube/cube.json`（server / web 两个 workspace）后，`cube open` 可分别以子目录为根打开，普通项目打开流程不变；
2. pnpm-workspace.yaml 的 monorepo 可通过探测生成候选并挑选落盘 cube.json；
3. 目录重命名后：cube.json 因相对路径继续生效，cache 类推导数据 miss 重扫，无脏数据残留；
4. `cd server && go vet ./... && go test ./...`、`pnpm -C web build` 通过；
5. `docs/spec/现状.md` 已同步（新增 .cube/cube.json 机制段落）。
