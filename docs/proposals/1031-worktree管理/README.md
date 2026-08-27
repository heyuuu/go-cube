# worktree 管理（工作台写侧）

> **状态**：✅ 已实施（待验收）——5 步全部落地，go vet / go test（17 包）/ pnpm build / 前端 44 测试全通过；验收通过后归档
>
> **关联**：[`1016-opener改造`](../archived/1016-opener改造/README.md)（打开 worktree 复用 RoleOpenDir，本提案不改 opener）；[`1011-工作台git树面板`](../archived/1011-工作台git树面板/README.md)（worktree 作为 TreeSource 的读侧已就绪）；[`1032-worktree归并为项目打开目标`](../1032-worktree归并为项目打开目标/README.md)（**已实施**：worktree 不再是独立项目，是主项目的打开目标，可见性来自 gitcache 快照的 `worktrees` 枚举；本提案已按此口径修订——验收 1 的「项目列表」改为主项目打开目标，目录位置决策的 maxDepth 论据失效（worktree 可放任意位置），写后定向刷新快照更加必要）。

## 背景与目标

workbench 已能**看** worktree：`/api/workbench/worktrees` 返回全部工作副本状态快照，diff / tree / file 面板已把 worktree 作为 TreeSource；opener 已能**打开** worktree（RoleOpenDir）。但写侧缺失——新增、删除 worktree、删除分支都只能去终端敲 git。

目标四个能力：

1. **新增 worktree**：以指定 commit/分支为基点，在默认目录建新 worktree（支持新建分支、检出已有分支、detached 三种形态）；
2. **打开 worktree**：已有能力（opener），UI 上接现成 open 动作即可；
3. **删除 worktree**：删目录 + `worktree prune`，带安全性预检；
4. **删除分支**：分类上是独立的分支管理功能，但「开分支建 worktree → 用完删 worktree → 删分支」是一条常用链，需要在本期一起落地。

## 讨论定稿（关键决策）

1. **worktree 目录位置：repo 同级容器 `<repoName>.worktrees/<branch>/`**。1032 落地后 worktree 不再走扫描（可见性来自主项目快照枚举，可放任意位置），原「同级扁平避免 maxDepth 卡掉」的约束消失，改用业内更通行的同级容器方式（worktree 集中收纳、按分支名分目录）。分支名含 `/` 时目录层级处理（如替换为 `-`）属实现细节。Web UI 新建对话框**预填**该默认路径、允许手改；不做配置化模板（真有需求再另加，届时挂全局 config，只记模式不记实例路径）。
2. **删除类操作统一 force 开关**：非 force 先预检（worktree 的未提交改动/未 push、分支的未合并/被检出），不满足时返回结构化中文原因，由 UI 二次确认后升级 force；force 直接强删（`--force` / `-D`）。强删是高频场景，force 必须一步可达（对话框直接给勾选项），不能藏在深层。
3. git 子进程写调用全部收敛为 `util/git` 类型化函数（规则 14）。
4. 写操作成功后失效/刷新 gitcache 对应路径，新 worktree 的出现/消失即时可见，不等 TTL 自愈。1032 落地后项目页目标展开 / open 目标选择全走快照的 `worktrees` 字段，本条从「增强」变为「必需」——不刷新则新建的 worktree 在打开目标里选不到。
5. **定位修订（伴随，已单独落地）**：出口优先级从「CLI 优先 + 本地 Web」调整为「**Web UI 优先，CLI 简单可行兜底**」。CLI 的目标是 web 不可用时兜底不至于寸步难行，便利性操作以 Web UI 为主。本需求的 CLI 兜底方案就是直接用 git 命令，因此 **CLI 子命令本期不做**（将来作为便利性补充可加）。AGENTS.md 与 `docs/spec/现状.md` 的定位表述已修订（独立文档提交，先行于本提案实施）。

## 方案

### 1. util/git 写侧类型化封装

- `WorktreeAdd(dir, targetPath, branch, commitish)`：branch 为空 → 以 commitish 建 detached；branch 不存在 → `-b branch commitish`；branch 已存在 → 检出该分支。已存在的分支若被其他 worktree 检出则预检报中文错误——数据复用 `WorktreeList`（其输出已含各副本检出分支），不加新调用。
- `WorktreeRemove(dir, targetPath, force)`：`git worktree remove [--force]`。
- `WorktreePrune(dir)`：删除后统一收尾。
- `BranchDelete(dir, branch, force)`：非 force 用 git 原生 `-d` 语义（已合并到上游或 HEAD 才允许，拒绝时错误翻译为中文提示上抛）；force 用 `-D`。

输出解析、环境兼容注入（规则 14 列举的 quotePath / locale / pager 等）全部收在包内。

### 2. workbench Service 写方法（service.go 薄编排）

- `WorktreeAdd(path, opts)`：推导预填路径（repo 父目录 + `<repoName>.worktrees/<branch>/`）、目标目录存在性校验（git 要求不存在或为空）、预检被检出分支冲突 → 调 git → 刷新 gitcache → 返回新 worktree 信息（供 UI 直接发起 open）。
- `WorktreeRemove(path, force)`：非 force 先预检（复用 WorktreeStatuses 逻辑取 dirty / untracked / ahead，有风险项即拒绝并返回原因列表）；force 直接 remove + prune；成功后刷新 gitcache。
- `BranchDelete(path, branch, force)`：**被任一 worktree 检出的分支无论 force 与否都拒绝**（git `-D` 也删不掉，属硬约束而非安全策略），说明是哪个副本检出；其余走 force 开关语义。

gitcache 刷新落点：gitcache 增加包级定向刷新入口（1032 后快照已含 `worktrees` 字段，主项目采集时附带枚举），workbench 写操作成功后定向触发对应主项目路径的重新采集回写，避免 workbench 对 project 域的反向依赖过深、也避免全量失效抖动。

### 3. Web API（规则 15：GET 查询 / POST 动作）

- `POST /api/workbench/worktree/add`
- `POST /api/workbench/worktree/remove`
- `POST /api/workbench/branch/delete`

查询侧已有 `GET /api/workbench/worktrees`；打开 worktree 复用 opener 既有 API，均不新增。

### 4. Web UI

- 工作副本列表每项加「删除」动作：对话框展示预检状态（dirty / untracked / ahead 徽标）+ force 勾选 + 可选「同时删除其上检出的分支」（前端串联 remove 与 branch/delete 两次调用，服务端不做复合 API）；
- commit 图 / refs 面板加「在此新建 worktree」入口：选基点（commit / 分支 / HEAD）+ 分支名（留空即 detached），预填目标路径可改；
- refs 列表加分支删除入口；
- 删除当前 workbench 正在查看的 worktree 后，前端导航回主项目根目录目标（1032 后的「主项目 + 目标展开」模型）。

## 不做的事

- **不改 opener 体系**：打开 worktree 就是 open-dir，role / slot / executor 全不动；
- **不做 worktree 目录模板配置化**：预填可改即可，需要时另案；
- **不做分支管理全功能**：只做删除这一个动作，新建/改名/merge 等分支操作仍走终端或 IDE；
- **不做 CLI 子命令**（见讨论定稿 5，定位修订后 CLI 只保证兜底可行，本需求兜底 = 直接用 git）；
- **不做批量删除**。

## 实施顺序

> 逐步实施、逐步验收；每步可独立收工。

1. `util/git` 写侧函数 + 单测（testfixture 真仓库，覆盖三种 add 形态 / 被检出分支拒绝 / force 与非 force 删除；1032 已给 testfixture 加 linked worktree 辅助，直接复用）；
2. workbench Service 写方法 + 预检 + gitcache 刷新 + 单测；
3. Web API 三个 POST + web 层测试（`newTestEnv` 模式）；
4. Web UI：新建对话框、删除对话框（force 勾选 + 联动删分支）、删除后导航；
5. 现状.md API 清单同步，验收后归档提案（定位修订已单独落地）。

## 验收标准

1. 从任一 commit/分支可新建 worktree（新建分支 / 检出已有分支 / detached 三种形态），默认路径 `<repoName>.worktrees/<branch>/`，新副本即时出现在主项目的打开目标 / worktrees 展示中（无需等 TTL）；
2. 删除有未提交改动的 worktree：非 force 返回明确中文原因，force 成功删除且 prune 干净（`worktree list` 无残留）；
3. 删除分支：被检出的分支无论 force 均拒绝并说明；未合并非 force 提示、force 成功；
4. 打开 worktree 复用既有 opener 流程无回归；
5. `cd server && goimports -w . && go vet ./... && go test ./...`、`pnpm -C web build` 通过；
6. 现状.md 的 API 清单已同步（定位表述已单独落地修订）。
