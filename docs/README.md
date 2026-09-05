# cube 文档

> cube —— 面向个人开发者的本地项目管理工具（CLI 优先 + 本地 Web）。

## 目录索引

### [spec/](spec/) — 项目现状

事实快照，描述当前代码「是什么」。

- [`spec/现状.md`](spec/现状.md) — 定位、分层架构、关键机制、命令列表、API 列表、数据模型、配置文件、测试策略

### [proposals/](proposals/) — 提案

未来需求的提案，每个提案一个目录（`1xxx-提案名/`，ID 为四位递增数字）。活动提案摊平在顶层；挂起的进 `parked/`，完成的进 `archived/`。

**ID 分配规则（硬约束）**：提案 ID 全局唯一、不得重复，三个目录（顶层 / `parked/` / `archived/`）共用一套编号空间。**新建提案前必须先执行 `make last-proposal` 获取当前最大 ID，新提案 ID = 最大 ID + 1**，不得凭记忆或猜测取号。

- [`archived/1040-forge管理/`](proposals/archived/1040-forge管理/) — ✅ 已实现（2026-09-05）：forge（git 托管平台实例）配置管理——`Forge{host,kind,icon}` 存 settings `forges` 节（host 归一化唯一键、kind 四值枚举 github/gitea/gitee/generic），CLI `forge list`、Web settings Forge 分区增删改、API list/save/delete；repo→forge 匹配在前端（repoUrl 解析 host 查表，projects 列表/树名称旁展示 icon），不写 git.json 快照
- [`1041-forge-account/`](proposals/1041-forge-account/) — forge account（`{ forgeHost, username, token }`）管理与 github/gitea/gitee 三方言拉取账号下仓库列表（手动触发 + easycache 缓存，不进 projcache 采集），与本地项目按 clone URL 对账（未 clone / 本地孤儿 / 已 clone+本地状态）；暂不做 owner/org 区分
- [`1042-forge页/`](proposals/1042-forge页/) — 前台 forge 页：以 forge 上仓库为入口的列表视图（类项目列表页、无 tree 模式），展示对账状态、未 clone 可一键 clone（复用 cloneRules）；数据获取/筛选/排序细节开工前另行讨论定稿
- [`archived/1038-opener默认/`](proposals/archived/1038-opener默认/) — ✅ 已实现（2026-08-31）：cmd 字符串化 → per-role actions 动作串（exec:/url:，站内路由拼 BaseURL）→ intent 8 值枚举 + `openerIntents` 节（默认 opener + 候选）→ CLI `-o` 三态（无=默认 / 裸=交互 / name=模糊）→ 前端快捷位意图化 + url 动作当前浏览器直开；已知取舍：url 前端直开不计 usage
- [`1036-server进程管理定调/`](proposals/archived/1036-server进程管理定调/) — 移除 `start -d` 自 fork 后台（argv 不透传 + 启动失败无声的结构性 bug，常驻由 launchd/air 承担后无消费者）+ whoami instance 实例标识（stop 按 instance 判旧实例下线，launchctl 保活占回端口不误判）+ daemon 设计定调（零参数发现配置 / 可管理性三条件 / supervisor-ready 六条 / 桌面 app helper 的 socket 重连与 pipe 生命线）；`server reload` 搁置（解挂条件在提案内）
- [`1030-monorepo-workspace/`](proposals/archived/1030-monorepo-workspace/) — monorepo 子目录打开支持：`.cube/cube.json`（workspaces 显式声明 / workspaceScanRule 探测正选，文件格式层独立 `project/cubefile`），workspace 进 projcache 采集与 `OpenTargets`（gitcache 更名 projcache）；CLI `workspace init`、Web 下拉/展开行/编辑弹窗、工作台副本行 workspace 子行
- [`1015-工作台面板组装/`](proposals/archived/1015-工作台面板组装/) — 工作台自定义布局（面板组装，最后做）
- [`1014-工作台PTY面板/`](proposals/archived/1014-工作台PTY面板/) — 工作台 PTY 终端（WebSocket + xterm.js）
- [`1013-工作台diff面板/`](proposals/archived/1013-工作台diff面板/) — 工作台双源 diff（目录 + 文件对比）
- [`1012-工作台代码阅读面板/`](proposals/archived/1012-工作台代码阅读面板/) — 工作台代码阅读（CodeMirror6 + 轻编辑）
- [`1011-工作台git树面板/`](proposals/archived/1011-工作台git树面板/) — 工作台 git 树（commit 图 + 工作副本状态）
- [`1010-workbench基座/`](proposals/archived/1010-workbench基座/) — 工作台基座（路由 + TreeSource + 核心 API + 面板骨架）
- [`1008-workspace工作台/`](proposals/archived/1008-workspace工作台/) — workspace 工作台总纲（决策记录 + 子提案索引）


#### [proposals/parked/](proposals/parked/) — 挂起提案

方向认可但触发条件未到，暂不实施；README 内记录解挂条件。

- [`1007-sqlc代替gorm/`](proposals/parked/1007-sqlc代替gorm/) — sqlc 代替 gorm（未来方向）
- [`1027-工作台暂存区分组展示/`](proposals/parked/1027-工作台暂存区分组展示/) — 工作台变更清单按暂存区/未暂存分组展示 + stage/unstage（SourceTree 式）
- [`1028-desktop壳与wails评估/`](proposals/parked/1028-desktop壳与wails评估/) — 不做 desktop 壳；wails 与「本地 server + 通用 HTTP API」结构性不匹配（私有 RPC vs 开放 API），个人工具线转投 Swift/SwiftUI
- [`1035-opener排序策略/`](proposals/parked/1035-opener排序策略/) — opener 按最近使用排序对「固定组合开多个 opener」场景反向轮转；候选方案已收集（滑动窗口频次 / frecency / n-gram 条件预测 / 显式分层等），待选型讨论
- [`parked/1039-opener适用范围match/`](proposals/parked/1039-opener适用范围match/) — opener 适用范围谓词（仓库根 vs 普通目录 / 文件后缀 / 项目语言）：已定不进 role、open-path 按普通目录、语言粒度到 workspace；intent 拆分已随 1038 落地，match 本体挂起——且方向可能转为「调用点按目标属性选更细 intent」，解挂时先重估前提
- [`1037-desktop薄壳重新评估/`](proposals/parked/1037-desktop薄壳重新评估/) — 壳需求重新成立（快捷键与浏览器冲突为硬需求，pty 保持已由超时独立解决）；前提收敛为自用/macOS 独占/薄壳（连已有 server，不动 1036 生命周期）；wails v2 功能不匹配（全局快捷键/多窗口系 v3 独有）、对 wails 整体持保留意见，候选路线对比已定倾向：**Swift 原生通用参数化壳**（`--url --hotkey`，零 cube 逻辑、独立仓库，与个人 Swift 工具线合流；Swift > Tauri ≈ Wails v3 > Electron）；解挂主条件为 Swift 工具线推进到成熟

#### [proposals/archived/](proposals/archived/) — 已完成提案归档

已实现的需求总结（从 proposals 顶层移入，记录最终落地形态与方案演变）。

- [`1031-worktree管理/`](proposals/archived/1031-worktree管理/) — 工作台 worktree/分支写侧：新增 worktree（新建分支/检出已有/detached 三形态，默认同级 `<repoName>.worktrees/<分支名>/`）+ 删除（非 force 预检 dirty/untracked/ahead 返回 denied+reasons，force 强删 + prune，主目录恒拒）+ 分支增删（被检出硬拒、未合并 force 语义；新建为验收前补充）；写后经 `RefreshOne` 定向刷新 gitcache；伴随定位修订「Web UI 优先，CLI 简单可行兜底」；前端默认分支名 `worktree-%02d`、回车提交等验收优化
- [`1032-worktree归并为项目打开目标/`](proposals/archived/1032-worktree归并为项目打开目标/) — worktree 从独立项目归并为主项目打开目标：scan 只收录 `.git` 为目录的主仓库（`.git` 文件 SkipDir），worktree 经 `git.WorktreeList` 枚举进 gitcache 快照（`Entry.Worktrees`，任意位置可见，存在性三处防护）；打开流程三入口选目标（CLI tui / alfred 平铺直达 / Web 子菜单），usage 记「项目+目录」双维度；`pickProject` 归并链路（pull/push/info 在 worktree 内=操作主仓库）；doctor worktree-lost + `--fix` 自动 prune；`worktree` tag 移除
- [`1034-最近使用排序与usage统一/`](proposals/archived/1034-最近使用排序与usage统一/) — 统一项目使用记录：history 两表重写为 usage JSONL（`state/usage.jsonl`，O_APPEND 无锁追加 + 启动 compaction，`dir` 字段为 1030/1032 预留）；打开入口收口到 `POST /api/project/open`（三入口全接入）；列表排序前端化（表头点击 + 最近使用列）；gorm/sqlite/data.db 整体移除；伴随项目查询键统一为 path（FindByName 删除）
- [`1033-util-store文件存储/`](proposals/archived/1033-util-store文件存储/) — util/store 文件存储原语包（WriteFileAtomic 原子写 / SaveJson·LoadJson 缩进 JSON + ErrFileMissing 哨兵 / JSONL 追加·全量读·全量重写·正向逆向流式迭代 iter.Seq2）；gitcache·config·settings 三处手写原子写全部收敛；1034 usage JSONL 的地基
- [`1029-scan-clone规则迁移settings/`](proposals/archived/1029-scan-clone规则迁移settings/) — scan/clone 规则迁 settings.json（`scanRules`/`cloneRules` 分节，Service 直读不缓存、写侧校验、保存即重扫）；settings 页「项目·扫描」+「项目·Clone」两分区（增删改 + 拖拽）；超额增强：scanRule 可选 icon + icon 语义下沉 util/iconkit + 前端 renderIcon/IconField 共享件；group 筛选改规则序
- [`1025-settings配置页/`](proposals/archived/1025-settings配置页/) — settings 配置页：`/settings` 分区导航（URL 记忆）+ ⌘,/rail 新 tab 入口；Config 过渡分区收编 `/config` 只读展示（默认分区，旧路由移除）；Opener 分区完整增删改（Sheet 抽屉 + 冻结列 + icon 渲染 + 拖拽排序 reorder API + lucide 搜索点选）
- [`1016-opener改造/`](proposals/archived/1016-opener改造/) — opener 改造：数据迁 settings.json（节级 API + 直读）+ 接口化 + icon 全链路（lucide/base64/`.app` 提取）+ Web 增删改；web 形态最终拆除，收敛为 `cube web workbench` exec 组合
- [`1009-模板引擎cube-create/`](proposals/archived/1009-模板引擎cube-create/) — `cube create` 模板引擎（template.yaml 协议 + glob 替换 + init 执行；本地目录/git 仓库/模板集收纳式；引擎是机制，模板是数据）
- [`1001-server后台常驻与HTTP管理/`](proposals/archived/1001-server后台常驻与HTTP管理/) — server 后台常驻（`start -d`）+ 基于 HTTP API 的进程管理（whoami/shutdown）
- [`1002-前端栈迁移/`](proposals/archived/1002-前端栈迁移/) — Alpine.js → Vite+React+TS（web/ 工程）整体迁移，三页落地 + 移除后端 tree 接口
- [`1005-md渲染/`](proposals/archived/1005-md渲染/) — `cube md` Web 渲染 markdown：零模板渲染 + 目录浏览模式 + opener 打开归一
- [`1003-web层测试补全/`](proposals/archived/1003-web层测试补全/) — httptest 集成测试基建 + 全部 handler 用例
- [`1006-history清理API/`](proposals/archived/1006-history清理API/) — history 数据清理

### [references/](references/) — 参考文献目录

同类工具的深度分析。

- [`mani.md`](references/mani.md) — alajmo/mani 竞品分析
- [`gitbatch.md`](references/gitbatch.md) — isacikgoz/gitbatch 竞品分析（含批量操作避坑点）

### [misc/](misc/) — 杂项

暂时不好划分的文件。

- [`project-template-spec.md`](misc/project-template-spec.md) — 从 cube 提炼的通用项目设计规范模板
- [`alpine-intro.html`](misc/alpine-intro.html) — Alpine.js 可交互演示

## 文档规则

- 目录下最重要的索引文件叫 `README.md`，其他文件一律小写无大写。
- 除 `README.md` 外，所有 markdown 文件名用小写（如 `现状.md` 可保留中文但不用大写英文）。
- 提案目录命名：`NNNN-提案名/`（4 位递增计数，自 1001 起，发号不回收；归档/挂起只挪目录不改号），内部主文件叫 `README.md`，细节放其他文件引用。
