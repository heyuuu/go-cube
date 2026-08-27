# worktree 归并为项目打开目标

}> **状态**：✅ 已实施（已归档，2026-08-28）
>
> **实施偏差备忘**（与原方案的差异）：
> 1. `util/git` 未新增封装——1011 已落地的 `WorktreeList` 直接够用，仅新增 `WorktreeMain`（worktree → 主仓库探测，归并链路原语）与 `WorktreePrune`（doctor --fix 用）；testfixture 补 `MakeWorktree` 建真实 linked worktree；
> 2. **worktree 存在性三处防护**（原方案未提，验收实测补强）：git 元数据未 prune 前会一直列出已删目录（幽灵目标，open 报 exit 1）——采集侧 `collectWorktrees` 按存在性过滤、查询侧 `OpenTargets` 同样过滤（覆盖两次采集间的窗口期）、Web `project/open` 失联目标报明确中文错误；
> 3. **doctor 超出原方案**：worktree-lost 检查升级为现场跑 `git.WorktreeList` 探测（不受快照滞后影响）+ 每条 finding 附 prune 建议命令 + 新增 `--fix`（自动对主仓库跑 `git worktree prune`，幂等无损，修复后复核）；
> 4. **workbench `/worktrees` 端点维持现状**（现场跑 git）——重构面表格里的待定项决策为不统一切 gitcache 快照，工作台与项目解耦不动；
> 5. 前端附随增强：`⎇ n` 计数徽标（紫罗兰专属配色）、worktree 筛选 chips（`?wt=1`，与 group/git/tag 同侧）、多目标打开走 Base UI 原生子菜单（SubmenuRoot）、详情抽屉 worktrees 独立分组 table 展示（抽屉同步加宽至 40rem）；
> 6. alfred `project-search` 定为**平铺直达**：每项目展开「根目录 + `name (branch)`」多条目，Arg 传目标目录路径，一步打开体验不变；`opener-search --project` 归一到主项目路径查偏好；
> 7. 伴随重构（验收驱动）：`easycache.Item` 自记计算时间戳（`UpdatedAt`，TTL 预留），`project.Service` 删平行字段 `scanUpdatedAt`；后台刷新改「保证缓存距今不超过 maxExpireTime（5 分钟）」单节奏（冷启动立即刷/热重载整周期等待/失败整周期退避）——「区分启动刷新与定时刷新」的前提随单节奏模型消失。
>
> **关联**：[`1030-monorepo-workspace`](../1030-monorepo-workspace/README.md)（**顺序依赖：本提案先行**，1030 的「打开流程接入」以本提案确立的「project → 打开目标」模型为地基）；[`1004-gitcache常驻化重构`](../archived/1004-gitcache常驻化重构/README.md)（git.json 快照体系，worktree 列表将作为推导数据进入快照）；[`1011-工作台git树面板`](../archived/1011-工作台git树面板/README.md)（workbench 已有「全部工作副本」端点，数据源随本提案切换）。

## 背景与目标

git 主仓库与它的 worktree 目前被 scan 当成**多个独立项目**管理（`.git` 存在即收录，`.git` 为文件时打 `worktree` tag，见 `project/scan.go`）。这是历史妥协：Alfred 等场景需要「直接打开 worktree 目录」，而当时没有「一个项目多个打开目录」的概念，只能靠项目粒度解决。

1030 讨论确立了「project → 多打开目标」模型后，这个前提消失。本提案把 worktree 从独立项目**归并为所属主项目的一个打开目标**，重塑项目身份模型：

**project = git 仓库身份（主仓库 root）；打开目标 = { 根目录, worktrees… }**（1030 落地后加入 workspaces…，即三来源）。

目标：

1. scan 只收录 `.git` 为目录的主仓库项目，worktree 目录不再是项目；
2. 主项目的 worktree 列表经 git 枚举动态推导（永不落盘，走 gitcache 快照），worktree 可位于任意位置（含 scan root 之外）；
3. 打开流程（CLI / alfred / Web）支持目标选择；
4. usage 记录「项目 + 目录」两个维度，同时满足 worktree 与后续 monorepo workspace 的记账需求。

## 决策记录（讨论定稿）

1. **扫描归并边界**：scan 只收录 `.git` 为**目录**的项目；`.git` 为**文件**的目录（linked worktree）跳过。规则：**主仓库必须在 scan-rule 之下，worktree 可以在任何位置**——worktree 可见性来自主项目枚举，不来自扫描。推论：主仓库不在任何 scan-rule 下的 worktree 整体不可见（接受，不做孤儿降级）。
2. **worktree 无独立项目身份，全部归并无例外**：「把 worktree 当独立项目」（列表独立条目、独立 history、独立入口）无实际诉求；「直接打开 worktree 目录」的需求由「选主项目 → 选 worktree 目标」满足。不为少数场景加例外机制（cube.json 不声明 worktree 归并/独立）。
3. **usage 双维度**：usage 记录（[`1034`](../archived/1034-最近使用排序与usage统一/README.md) 已落地的 JSONL `Record`）的 `dir` 字段承载目标维度——`project` 恒记主项目**绝对路径**（归并键），`dir` 记目标的**绝对路径**（主根打开省略 / worktree 为其路径 / workspace 为其绝对路径，1030 落地后生效）。usage 是流水信号而非配置，目录重命名后旧记录仅展示不全，不构成脏数据。~~原方案的 `ProjectOpenLog` 加列（sqlite AutoMigrate）已废弃~~——1034 已将 history 整体重写为 usage JSONL 并预留 `dir` 字段，本提案只负责打开入口写入时传目标路径。
4. **`worktree` tag 移除**：归并后 tag 失去载体。

## 方案

### 1. 扫描改造（`project/scan.go`）

- 项目判定收紧：`.git` 为目录才收录；`.git` 为文件的目录直接跳过（不再遍历其子目录，同隐藏目录待遇）；
- `TagWorktree` 常量与打点删除；tags 只剩 `godot`（及未来扩展）。

### 2. worktree 枚举（util/git + gitcache）

- `util/git` **复用既有 `WorktreeList(dir)`**（1011 已落地的类型化封装，解析 `git worktree list --porcelain`，返回全部工作副本的路径/分支/detached 标记），无需新增；
- **git.json 快照新增 worktree 字段**：后台采集时顺带枚举并回写快照；读路径（项目列表、打开流程）只读快照，不现场跑 git（遵守「读路径不得阻塞采集」）；
- 枚举结果属**推导层**：永不落盘为配置、目录重命名/删除后快照自然淘汰，与「cache 按路径 key 可丢弃」判别式一致。

### 3. project 层查询

`project.Service` 新增打开目标查询：返回项目的目标列表（根目录 + worktrees，1030 后含 workspaces）。worktree 展示名取**分支名**（冲突或 detached 时回退目录名）。

### 4. 打开流程（CLI / alfred / Web）

- `cube open`：选定项目后，若有多目标则 `tui.SelectItem` 选目标（含「根目录」），再走 opener 选择；单目标项目流程不变；
- alfred：列表交互形态（项目层级展开 vs 目标平铺 `cube: repo (branch)`）执行过程中边改边实测手感再定，倾向平铺直达以保留现有一步打开体验；
- Web 项目页 / workbench 打开入口同构接入。

### 5. 作用域边界与路径归并

非 open 命令不引入目标选择，但需要一条**路径归并链路**——`pickProject` / 项目反查在命中 worktree 目录（`.git` 为文件）时，顺着 git 元数据定位主仓库并选中主项目（worktree 内跑 pull / push 等同于在主目录操作）。分命令语义：

- **pull / push**：pickProject 归并到主项目，作用于主仓库根目录；
- **info**：同上归并，但输出**注明当前所属 worktree**（分支/路径）；
- **diff**：纯目录对比操作（参数是两个路径），与项目身份无关，不归并不变。

open 链路（选主项目 → 选目标）是本提案的重心；其他命令的目标化留待后续提案。

## 重构面盘点（影响清单）

| 位置 | 变化 |
|---|---|
| `project/scan.go` | `.git` 为文件跳过；`TagWorktree` 删除 |
| `pickProject` / 项目反查 | 命中 worktree 目录时顺着 `.git` 文件定位主仓库，归并选中主项目（pull / push / info 等命令入口） |
| `cmd/info.go` | 输出注明当前所属 worktree |
| `cmd/open.go` / `cmd/alfred` | 插入目标选择步 |
| `usage` | `Record.dir` 字段已由 1034 预留（无需存储改动），本提案在打开入口写入时传目标绝对路径 |
| `gitcache` | 快照结构加 worktree 列表；worktree 目录不再作为独立项目采集，改由主项目采集时附带 |
| `cmd/doctor.go` | 「指向已删除主仓库的 worktree 残骸」检查项语义变化：悬空 worktree 不再被收录，该项改为检查快照内 worktree 路径失联 |
| `handlers/workbench_handler.go` | 「全部工作副本」端点（`/api/workbench/worktrees`）现状即现场跑 `git.WorktreeList`、与 project scan 无关；实施时决策是否统一切换到 gitcache 快照（可选，维持现状也成立） |
| 前端 projects 页 / workbench | 列表展示从平铺 worktree 行改为主项目 + 目标展开 |
| alfred workflow | 输出形态适配 |

## 与 1030 的顺序关系

本提案**先行**：它建立「project → 打开目标」模型与交互骨架；1030 的 cube.json 解析 / 候选探测地基可并行开发，但其「打开流程接入」一步在本提案落地后实施（作为目标列表的第三个来源插入，纯增量）。

## 实施顺序

> 逐步实施、逐步验收；每步可独立收工。

1. testfixture 建 linked worktree 辅助 + `util/git.WorktreeList` 的 worktree 场景单测（复用既有封装，无新代码）；
2. gitcache 快照加 worktree 字段 + 采集/读取改造 + 单测；
3. scan 收紧（跳过 `.git` 文件）+ `TagWorktree` 移除 + scan 单测更新；
4. project 层目标查询 + 打开入口记录 usage `dir`；
5. CLI `cube open` / alfred 目标选择；
6. Web API + 前端（projects 页 / workbench / doctor 语义切换）；
7. `docs/spec/现状.md` 同步，验收后归档提案。

## 验收标准

1. worktree 目录（无论在 scan root 内外）不再出现在 `cube project list`，其主项目条目可展开选择 worktree 打开，alfred 可一步直达 worktree；
2. 主仓库被删的悬空 worktree 不产生幽灵项目；doctor 的失联检查改为面向快照；
3. usage 记录含目标维度（`dir` 为实际打开目录绝对路径，`project` 恒为主项目路径）；
4. 普通 git 项目（无 worktree）全流程行为不变；
5. `cd server && go vet ./... && go test ./...`、`pnpm -C web build` 通过；
6. `docs/spec/现状.md` 已同步。
