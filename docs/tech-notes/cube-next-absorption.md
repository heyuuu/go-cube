# cube-next 可吸收内容讨论记录

> 本文件记录从 cube-next（已废弃的 OpenSpec + 全程 AI 重构项目）中筛选、讨论、吸收内容的过程。
> 每条结论定稿即写入，不等批次。
>
> 讨论基准：当前 cube 已完成 config 去全局单例、logger 简化（~90 行）、cmd 原生 cobra 工厂函数、web huma + ApiOutput envelope 等重构。cube-next 的部分"改进"是相对更早版本 cube 而言的，对当前 cube 不构成吸收项，已在「对照澄清」段记录。

---

## 对照澄清（cube-next 提及但 cube 已落地，无需再吸收）

| 项 | cube-next 主张 | cube 现状 | 结论 |
|---|---|---|---|
| config 去全局单例 | 走注入 | ✅ `app.New(cfg)` 注入 | 已落地 |
| logger 简化 | 砍定制用 slog+tint | ✅ ~90 行单文件 | 已落地 |
| cmd 原生 cobra | 砍 easycobra | ✅ 工厂函数 | 已落地 |
| `-d` debug 收缩 | 只影响 logger | ✅ 已收缩 | 已落地 |
| web huma + envelope | ApiOutput | ✅ 泛型 `ApiOutput[T]` | 已落地 |
| Error 消息中文 | 强制 | ✅ 已是规则 | 已落地 |

---

## 讨论结论（逐条追加）

### 1. opener 引入 `Executor` 接口注入
- **状态**：✅ 吸收
- **决策**：在 opener 包引入 `Executor` interface，抽象「启动子进程」副作用；`Opener` 持有 executor 字段，`Open()` 委托它执行；默认实现走 `os/exec`（接管 stdio，与现有 `runner.Run` 行为一致）；测试注入 fake 断言组装的命令。
- **理由**：
  - 现状 `Open()` → 全局函数 `runner.Run`（无接口、不可 mock），可测性止于 `BuildArgs`，全链路无法单测。
  - 引入接口后 `Open()` 全链路可测，打开 AGENTS.md「不写单测：opener.Open」的缺口。
  - 纯增量改进，默认行为完全一致（不改变既有语义）。
- **落地动作**：
  1. 新建 `server/opener/executor.go`：`Executor` interface（`Run(bin string, args ...string) error`）+ `osExec` 默认实现（接管 stdout/stderr，搬自 `runner.Run`）+ `NewDefaultExecutor()`。
  2. `Opener` struct 加 `executor Executor` 字段。
  3. `InitOpener(cfg, executor ...Executor)` / `NewService(cfg, executor ...Executor)` 用可变参数，**缺省内部装 `NewDefaultExecutor()`**；测试传 fake 注入。注入点收敛在 opener 包内——app 层不感知 executor（executor 抽象只为 opener 包内单元测试服务，app 层以上调用方的可测性是另一个问题，与 executor 无关）。
  4. 删除 `server/util/runner/` 包（仅 opener 使用，逻辑已并入 executor）。
  5. 补 `Open()` 单测（`TestOpenInvokesExecutor` 3 场景 + `TestOpenSlotCountMismatch`）。
- **来源**：cube-next `server/opener/executor.go`。

### 2. gitcache 采集逻辑提取为纯函数 `collectEntry`
- **状态**：✅ 已落地（无需改动）
- **决策**：不吸收——当前 cube 已是该形态。
- **理由**：
  - `collectEntry(path) *Entry` 已是独立纯函数（`server/project/gitcache/cache.go:249-271`），与 `Refresh` 的并发调度分离。
  - 采集失败用零值降级，单项目 panic 被 recover 吞掉（保留旧 entry）。
  - 已有单测覆盖：`TestCollectEntry_RealRepo` / `TestCollectEntry_NonRepo` / `TestRefresh_RealRepo`（`cache_test.go`）。
  - cube-next 标榜的「采集逻辑函数化」是相对更早版本 cube 的改进，当前 cube 已完成。
- **落地动作**：无。
- **备注**：相关的「砍 fork/flock/TTL 改 goroutine」是另一条独立议题（见后续讨论），与本条无关。

### 3. history 加 `PurgeBefore` 清理 API
- **状态**：⏸️ 暂不做
- **决策**：现阶段不加清理 API，等 history 写入面铺开后再评估。
- **理由**：
  - 当前 history 写入**只在 alfred 流程**（`cmd/alfred/project_search.go` 写 select、`cmd/alfred/project_open.go` 写 open），CLI 的 `open` / web 的 `/api/project/open` 路径都没接 history。写入面比 cube-next 设想的（全入口事件流）小得多。
  - 写入量极小：单用户、单 alfred 交互，估算一年 ~7300 行/表，sqlite 十年都不会有压力。
  - 清理策略（按时间 vs 按条数、阈值多少、触发时机）现在没有实际数据支撑，提前定是凭空猜。
  - history 的核心用途是「最近用排序」（`Order("max(id) desc")`），长期记录有频率信号价值，过早清理反而损信号。
- **未来触发条件**：当 history 写入面铺开到 CLI/web 全入口后，重新评估数据增长。若确需清理，倾向按时间（如 `PurgeBefore(90天前)`），触发点挂 `app.New()` 启动时跑一次。
- **来源**：cube-next `server/history/store.go` 的 `PurgeBefore`。

### 4. cmd/context 懒构造 App
- **状态**：⛔ 不吸收
- **决策**：不引入懒构造，维持 `app.New(cfg)` 在启动期一次性装配、`*app.App` 显式传入所有命令工厂的现状。
- **理由**：
  - 收益很窄：只有 `version` 这类不需要 App 的命令能省启动开销，而绝大多数命令都需要 App。
  - 本地工具性能要求不高，启动期一次性装配（含 db.Init）没有实际可感知的差异。
  - 不做预防性设计：懒构造的「架构价值」（为将来按需装配子集留口子）属于猜测性需求，等问题真出现再加，成本不会变高。
  - 引入 Context/appLoader 会增加间接层，十几个命令文件签名都要改，是纯负担。
- **未来触发条件**：若某条新命令确实强烈需要跳过 App 装配（如 `cube create` 只需 config 不需 db），届时再针对性引入，不提前铺。
- **来源**：cube-next `server/cmd/context/context.go`。

### 5. sqlc 代替 gorm
- **状态**：⏸️ 暂不做（记录为未来方向）
- **决策**：当前 cube 不换 sqlc，继续用 gorm。sqlc 作为「未来解决 AI 审计」的方向在新项目里实验。
- **理由**：
  - **触发条件不成立**：cube 数据层极简（2 表 3 字段、固定 CRUD、零动态查询）。sqlc 相对 gorm 的优势（编译期类型安全、AutoMigrate 改/删列不漂移、schema 显式）在当前规模几乎不触发；gorm 的两个短板（AutoMigrate 漂移、动态查询弱）当前都没踩到。
  - **AI 契约价值下降**：sqlc 在 cube-next 的核心价值是「schema 作为防 AI 出错的审查契约」——但那依赖 OpenSpec + 全程 AI 模式。当前 cube 是人在主导重构，契约价值减半。
  - **换的成本不低**：codegen 工具链 + 重写 history + migration runner + 调试体验变化 + 未来每张新表走 migration→schema→query→generate 四步。当前收益覆盖不了成本。
- **未来方向（记录）**：sqlc 的「SQL 作为显式契约、便于 AI 审计」方向值得探索，但不在 cube 当前阶段验证。若未来 cube 数据层显著复杂化（新表多、字段频繁演进、复杂查询），或重新尝试 spec 先行 + AI 辅助模式时，再回头评估。
- **轻量改进备选**：若仅担心 gorm AutoMigrate 漂移，可加 migration 文件 + CI 校验，不上 codegen 即可拿到「schema 显式」的一半好处。未实施，按需启用。
- **来源**：cube-next `docs/design.md`「SQLite 库选型」、`server/sql/` 全套。

### 6. 前端栈整体迁移（Vite + React + TS + React Query）
- **状态**：📋 后续待办（单独大需求）
- **决策**：前端从当前 vanilla JS 整体迁移到 Vite + React + TypeScript + React Query + Tailwind + shadcn 风格组件。作为独立大需求排期，不在本次吸收过程中动代码。
- **关键技术决策**：
  1. **前后端契约不走 HTTP**：用 `cube openapi` 命令本地生成 `openapi.json` 文件，再用该文件通过 openapi-typescript 生成 TS 类型。避免「构建前端必须先起 server」的依赖（cube-next 的 `gen:api` 脚本依赖 server 在线，是个痛点）。
     - 当前 cube 已有 `cube openapi` 命令（`server/cmd/` 下），天然支持此工作流。
  2. **React Query 替代手写 fetch 封装**：cube 有「projects 列表轮询 git 状态」场景（gitcache 定期刷新），react-query 的 `refetchInterval` 正好契合，比 cube-next 的手写 `apiGet<T>` 更合适。
  3. **UI 组件用 shadcn 风格**：cva + clsx + tailwind-merge + lucide-react（cube-next 已验证）。
- **不涉及**：本次不做任何前端代码改动，现有 vanilla JS 前端维持运行。
- **来源**：cube-next `ui/` 工程栈 + cube-next `gen:api` 脚本（工作流改进版）。

### 7. 命令树镜像文件树（cmd 子包化）
- **状态**：⛔ 不吸收
- **决策**：维持 cmd 平铺，不引入「组命令建子包、叶子留根」的镜像结构。
- **理由**：
  - cube-next 主张组命令（有子命令）建子包 `cmd/<命令名>/`、叶子命令留根，让命令树在文件树上镜像呈现。
  - 但 cube 已主动选择**反向**方向：commit `d84d076`「cmd 命令结构调整，展平常用命令」把 `cmd/project/`、`cmd/opener/`、`cmd/gitx/`、`cmd/server/`、`cmd/ui/`、`cmd/ugly/` 等子包全部拆掉，命令文件上提到 `cmd/` 根。
  - 当前只有 `cmd/alfred/`、`cmd/dev/` 两个子包（它们有独立语义：alfred 是外部 workflow 集成、dev 是开发调试命令）。
  - 两个方向冲突，cube 的平铺方向是有意选择，不跟 cube-next。
- **来源**：cube-next `server/cmd/` 目录结构 + AGENTS.md「命令树镜像文件树」规则。

### 8. paths 包独立集中化
- **状态**：⛔ 不吸收（现状可接受）
- **决策**：维持 `Paths` 放在 `app` 包的现状，不拆成独立的 `paths` 基础设施包。
- **理由**：
  - cube 已有 `app.Paths`（`server/app/paths.go`）集中了 DataDir + DataDbFile + CacheDir，路径计算没真正「散落」。
  - `logFileName`（"cube.log"）留在 logger 包、`DataDir` 推导留在 config 包，是**分层纪律的合理结果**——logger/config 是基础设施层，不能反向依赖 app 拿 Paths。这个分割不是 bug。
  - 路径常量就 3 处，改动频率极低，「单一数据源」收益很小，拆包是纯组织调整不解决实际问题。
- **未来触发条件**：若新增多个存储子项（workspace 状态文件、模板缓存目录等），届时可顺手把 paths 独立成基础设施包。
- **来源**：cube-next `server/paths/` 包。

### 9. 目录结构 flat vs 分层表述
- **状态**：⛔ 不吸收
- **决策**：保留 cube 的分层心智模型（基础设施/能力/领域/出口/装配），物理组织维持 flat。不跟 cube-next「砍 domain 词汇、纯 package 平级」的方向。
- **理由**：
  - 物理组织上 cube 已经是 flat（`server/` 下各包平铺），和 cube-next 无差异。
  - cube-next 真正主张的是**砍心智模型**——弃用「领域」等词、所有 package 平级表述。这对 cube 无益。
  - cube 的分层表述是**依赖纪律的锚点**：「基础设施不依赖上层；能力层只依赖基础设施；cmd 与 web 不互调」这套规则的载体就是「基础设施/能力/领域」这些词。砍掉它们，依赖纪律失去表述工具。
  - cube-next 砍 "domain" 的理由（定位变成项目工作台、project 不再是领域之一）不适用于 cube——cube 里 project 就是核心领域，opener 是另一个领域，「领域」用词准确。
- **来源**：cube-next `docs/design.md`「目录结构」+「词汇决策」。

### 10. OpenSpec SDD 工作流
- **状态**：⛔ 不吸收（工具）；✅ 已落地（轻量 spec 管理 + 架构/功能分离）
- **决策**：不引入 OpenSpec 工具（openspec/ 目录 + propose/apply/archive 命令循环）。维持 cube 现有的轻量 spec 管理。
- **理由**：
  - **OpenSpec 工具是 cube-next 失败的核心载体**：cube-next 用 OpenSpec + 全程 AI 的模式，结果「AI 生成代码不可控，没法验收」（用户原话）。工具负担 > 收益。
  - cube 当前已有轻量 spec 管理在运行：`docs/spec.md`（Requirements/Design/Tasks 三段契约快照）+ `docs/design/`（设计推理）+ AGENTS.md 开篇引用。这套够用，不需要 OpenSpec 的重型流程。
  - cube-next 的一个观察值得保留——「架构决策留 design.md（项目宪法），spec 只管功能级 change」。cube 已经是这个结构（`docs/design/` 放架构，`docs/spec.md` 放功能契约）。
- **来源**：cube-next `docs/design.md`「OpenSpec SDD 工作流」+ AGENTS.md「OpenSpec 操作约定」。

### 11. 砍 gitcache fork/flock/TTL 改 goroutine
- **状态**：⛔ 不吸收
- **决策**：维持 fork/flock/TTL 整套异步采集机制，不改 goroutine 定时采集。
- **理由**：
  - cube-next 砍 fork/flock 的前提是「Web server 常驻单进程采集」，但这个前提在 cube 当前版本不成立，**且 Web server 常驻单进程采集目前不是 cube 的方向**。
  - cube 的 alfred 场景是**高频短命进程**（每次按键 fork 一次 cube），fork/flock/TTL 是为这个场景设计的正确机制：父进程 TTL 判断（1 分钟内不重复）→ fork 子进程脱离会话异步采集 → flock 跨进程互斥（防并发 alfred 重复采集）→ 子进程落盘后下次父进程读快照。
  - 改 goroutine 的前提是「采集进程能存活」，但 alfred 每次按键是新进程，goroutine 活不到下次采集。
  - cube-next 砍它的另一个前提（CLI list 不展示 git 信息、只 Web 展示）也不符——cube 的 CLI `project list --status` 和 alfred 都读 git 缓存。
  - 相关的「采集逻辑函数化」是另一条，已在第 2 条确认 cube 已落地。
- **未来触发条件**：仅当 cube 方向明确转向「Web server 常驻为主入口、alfred 短命进程场景弱化」时，才重新评估。目前不是方向。
- **来源**：cube-next `docs/design.md`「CLI/Web 分工 + gitcache 简化」+ `server/gitsnapshot/`。

### 12. 扔掉的若干项（逐条过）

#### 12.1 cube-next AGENTS.md 大段编码规则
- **状态**：不写入（非吸收范畴）
- **决策**：不作为 cube-next 吸收项记录。这些规则（格式化、Error 中文、构造函数命名等）cube AGENTS.md 大部分已有，少数缺口（`0o` 前缀、空切片 nil、Cobra 工厂函数）是 cube 自身编码规范问题，独立于本次吸收。

#### 12.2 砍 `-d` debug 模式
- **状态**：⛔ 不吸收（已用更温和方式落地）
- **决策**：维持现状——保留 `-d` 但语义收缩为只影响 logger 初始化（commit `3d81cf1`）。
- **理由**：cube-next 主张彻底砍 debug，理由是它「不是业务概念，是日志配置」。但 cube 已用更温和的方式处理：commit `3d81cf1` 移除了 `config.IsDebug()`，收缩 debug 只影响 logger 包初始化（是否叠加 stdio 彩色通道）。当前形态既保留了对调试有用的开关，又消除了 debug 渗透业务的问题，合适。

#### 12.3 `internal/` 强制全收进
- **状态**：⛔ 不吸收
- **决策**：维持 Go 代码在 `server/` 下平铺的现状，不引入 `internal/` 包裹。
- **理由**：`internal/` 的价值是编译器层面的「防外部 import」，但 cube 是终端应用、module path 是 `cube`、不对外提供库，这个保护无意义。加一层目录嵌套（`cube/internal/project/...` vs `cube/project/...`）是纯负担，无实际收益。

#### 12.4 `main.go` 放根目录 + `//go:embed` 写在 main
- **状态**：⛔ 不吸收
- **决策**：维持 `main.go` 在 `server/main.go`、`//go:embed ui/*` 在 `web/static.go` 的分离现状。
- **理由**：
  - cube-next 把 embed 塞进根 main.go 是为绕 go:embed 的路径约束（不能引用 `../..`），是妥协不是设计优势。
  - cube 的 module 根是 `server/`，embed 在 `web/static.go`（embed `web/ui/*` 在 web 包内），路径约束已满足。
  - 静态资源挂载是 web 包的职责，放 web 包比放 main.go 更内聚；main.go 只管装配。
  - 仓库根还有 `ui/`、`docs/`、`Makefile`，main.go 放仓库根会让顶层文件混杂。

#### 12.5 Conventional Commits 强制 + 每轮提交
- **状态**：⛔ 不吸收
- **决策**：不写进 cube AGENTS.md，维持当前自然习惯。
- **理由**：
  - **规范本身**：cube 的 git log 已是 conventional commits 风格（`feat:` / `fix:` / `refactor:` 都在用），是已养成的习惯。写不写进 AGENTS.md 是风格选择，倾向不写——这是个人偏好不是项目强制规则，写进去反而约束未来可能的灵活提交。
  - **「每轮 AI 回复必须提交」**：这是 cube-next 针对「全程 AI 重构」模式设计的纪律（让 AI 每轮产出都有 git 记录可回溯）。当前模式是「人主导重构、AI 辅助」，提交节奏由人掌控，这条不适用。

#### 12.6 samber/lo 强制引入
- **状态**：⛔ 不吸收
- **决策**：维持 cube 自己的 `util/slicekit`，不引 samber/lo，不写进 AGENTS.md 作为强制规则。
- **理由**：
  - cube 的 slicekit 极轻（几个泛型函数），维护成本近零，不构成自维护负担。
  - samber/lo 是大库（hundreds of functions），引入它来替换几个 Filter/Map 是过度。
  - 标准 `slices`/`maps` + 自家 slicekit 的组合已够用。
  - **用户明确表态不喜欢 samber/lo，倾向自己的封装更可控。**
  - 不写进 AGENTS.md 作为强制规则——「优先用哪个工具库」是编码时的判断，cube 现有「优先复用 util/」规则已覆盖精神。

### 13. 未来需求的参考材料（记为待办）
- **状态**：📋 待办（启动对应需求时回看）

#### 13.1 模板引擎（`cube create`）
- **决策**：启动 `cube create` 时直接回看 cube 自己的 `docs/template-engine/`（README.md / discussion.md / glob-rules.md）。
- **说明**：设计产出在 cube 自己仓库，cube-next 只是引用，无新增内容。核心设计：「引擎=机制，模板=数据」；占位符由模板自定义、引擎不认识固定占位符；精确字符串替换不用正则；glob 匹配文件路径+内容统一替换。不属于 cube-next 吸收范畴。

#### 13.2 workspace 工作台 + diff / 伪终端
- **决策**：启动 workspace / diff / 伪终端讨论时，回看 cube-next 的 `deferred-features.md` 作为起点。
- **关键洞察**（来自 cube-next 分析，启动讨论时先定）：
  - 三方向：代码阅读 / Git 操作 / 命令行，各有浅中深三档。
  - 真 PTY 终端是**架构临界点**——引入 WebSocket 长连接后不可逆。
  - 代码编辑深度档要嵌 Monaco Editor（VSCode 同款）。
  - Git 深度档要嵌 go-git 完整能力（冲突解决/rebase）。
  - 深度选择不只影响「做多少功能」，还直接影响 Web 出口层架构——**启动讨论时必须先定深度档位再动手**。
- **来源**：cube-next `docs/deferred-features.md`。

### 12. 明确扔掉的若干项（批量）
- **状态**：⛔ 不吸收
- **决策**：以下 cube-next 主张一律不吸收，理由简述如下。

| 项 | 扔掉理由 |
|---|---|
| OpenSpec 工具链 + changes/ 目录 | 已在第 10 条定稿（工具不引入） |
| cube-next AGENTS.md 大段已落地规则 | cube 已覆盖，见本文件「对照澄清」段 |
| 砍 `-d` debug 模式 | cube 已收缩 debug 语义（commit `3d81cf1`，只影响 logger），保留对调试有用的一面，不彻底砍 |
| `internal/` 强制全收进 | 风格选择，cube 当前 `server/` 布局运行良好，迁移纯负担 |
| `main.go` 放根目录 + `//go:embed` 在 main | cube 当前 `server/main.go` + `web/static.go` 分离更清晰，不照搬 cube-next 的「embed 写根 main.go」 |
| Conventional Commits 强制 + 每轮提交 | cube 的提交习惯已成型（git log 已是 conventional commits 风格），不必列为强制规则 |
| samber/lo 强制引入 | 可选引入，不列为强制；标准库 `slices`/`maps` 优先，缺了再考虑 samber/lo |

### 13. 未来需求的参考材料（不立即动）
- **状态**：📋 参考材料（启动对应需求时回看）
- **决策**：cube-next 推迟的 workspace 工作台、模板引擎、diff/伪终端等，作为未来需求的参考材料保留，不在本次吸收。

**① 模板引擎（`cube create`）**
- 设计产出在 cube 自己的 `docs/template-engine/`（README.md / discussion.md / glob-rules.md），设计完整。
- 核心设计：「引擎=机制，模板=数据」；占位符由模板自定义、引擎不认识固定占位符；精确字符串替换不用正则；glob 匹配文件路径+内容统一替换。
- cube-next 只是引用这套设计，无新增。启动时直接用 cube 自己的文档，不需要回看 cube-next。

**② workspace 工作台 + diff / 伪终端**
- cube-next 的分析框架有参考价值：三方向（代码阅读 / Git 操作 / 命令行）× 各自的浅中深三档。
- 关键洞察（启动讨论时的起点）：
  - 真 PTY 终端是**架构临界点**——引入 WebSocket 长连接后不可逆。
  - 代码编辑深度档要嵌 Monaco Editor（VSCode 同款）。
  - Git 深度档要嵌 go-git 完整能力（冲突解决/rebase）。
  - 深度选择不只影响「做多少功能」，还直接影响 Web 出口层的架构改造——**启动讨论时必须先定深度档位**。
- 来源：cube-next `docs/deferred-features.md`。

