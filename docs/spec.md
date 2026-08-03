# cube · Spec

> 本项目以 **Spec-Driven Development (SDD)** 管理演进。本文是项目当前的**契约快照 + 未来工作清单**，三段式结构：
>
> - **Requirements** — 系统"应当"做什么（EARS 语法，反向提取自已落地功能）
> - **Design** — 技术契约（架构/数据模型/关键机制，指向详述文档）
> - **Tasks** — 未来工作的可执行 checklist（按里程碑分组）
>
> **与既有文档的关系**：本文是事实源（source of truth）。`docs/design/v3-design.md`（总体详述）、`docs/design/v3-frontend.md`（前端详述）、`AGENTS.md`（编码规则）是补充参考；与本文冲突时以本文 + 代码为准。
>
> **维护节奏**：每次合并一个有意义变更后，同步更新对应段落（实现的需求从 Tasks 移到 Requirements；过时的 Design 段落修正）。`docs/design/` 下的文档记录"为什么这么设计"的历史与推理，本文记录"当前是什么"。

---

## 一、Requirements（需求）

> EARS 语法：`WHEN <触发> THE SYSTEM SHALL <行为>` / `IF <条件> THEN ...` / `THE SYSTEM SHALL <无条件>`。
> 标 ✅ = 已实现并稳定；🟡 = 已实现但有已知缺陷/局限；⏳ = 计划中（见 Tasks）。

### 1.1 定位与边界

- **R-001** ✅ THE SYSTEM SHALL 提供面向个人开发者的本地多项目管理能力，不依赖任何云服务。
- **R-002** ✅ THE SYSTEM SHALL 不自身集成 AI，但可被外部 AI 平台编排（作为 MCP server，见 Tasks M5）。
- **R-003** ✅ THE SYSTEM SHALL 提供三个出口：CLI（人用 / alfred）、本地 Web HTTP server、（未来）MCP。
- **Out of Scope**：云同步、多用户、用户认证（Web 仅 localhost）、SSR、移动端、GUI 客户端。

### 1.2 项目扫描与识别

- **R-101** ✅ WHEN 用户配置 `ScanRule{Group, Path, MaxDepth}` THE SYSTEM SHALL 扫描 Path 目录树，把含 `.git` 条目的目录识别为项目，归属该 Group。
- **R-102** ✅ THE SYSTEM SHALL 跳过以 `.` 或 `_` 开头的目录名（如 `.git` 本身、`.idea`、`_draft`）。
- **R-103** ✅ WHEN 项目目录的 `.git` 是文件（非目录）THE SYSTEM SHALL 打 `worktree` tag。
- **R-104** ✅ WHEN 项目目录含 `*.godot` 文件 THE SYSTEM SHALL 打 `godot` tag。
- **R-105** ✅ THE SYSTEM SHALL 命中项目后停止下钻该子树（`SkipDir`），并按 MaxDepth 剪枝未命中路径。
- **R-106** ✅ THE SYSTEM SHALL 把 scan 规则的 Path（含 `~/`）展开为绝对路径；目录不存在的规则降级跳过，不阻断其它规则。

### 1.3 git 信息采集

- **R-201** ✅ THE SYSTEM SHALL 为每个项目采集 git 快照（RepoUrl / CurrentBranch / DefaultBranch / Branches / Ahead / Behind / Dirty / WorktreeMain / CollectedAt）。
- **R-202** ✅ WHEN 读命令（list/info/Web list）执行 THE SYSTEM SHALL 返回 git 缓存快照（几乎零开销），**不得阻塞**采集。
- **R-203** ✅ WHEN 读命令返回前 THE SYSTEM SHALL fork 后台子进程异步采集回写缓存，TTL（默认 1 分钟）内不重复触发。
- **R-204** ✅ THE SYSTEM SHALL 用跨进程 flock 保证同一时刻只有一个采集进程。
- **R-205** ✅ WHEN 长驻进程（web server）发现磁盘缓存比内存新 THE SYSTEM SHALL 主动 Reload 内存缓存。

### 1.4 项目操作

- **R-301** ✅ THE SYSTEM SHALL 提供模糊搜索（fuzzy，DP 算法 + bonus 加权 + 分词）按项目名匹配。
- **R-302** ✅ THE SYSTEM SHALL 支持用配置的 opener（编辑器/IDE/Finder/对比工具）打开项目路径。
- **R-303** ✅ WHEN 用户提供 repoUrl THE SYSTEM SHALL 按 CloneRule 算出本地落地路径并执行 git clone（兼容 ghq `host/path` 镜像约定）。
- **R-304** ✅ THE SYSTEM SHALL 提供 `project init`（git init + .gitignore 模板）。
- **R-305** ✅ THE SYSTEM SHALL 提供 `project check`（clone-rules 合规 / git-dirty / 分支偏离检查）。
- **R-306** ✅ THE SYSTEM SHALL 提供项目目录树视图（CLI tui + Web + Alpine 前端三种渲染，共用后端 BuildTree）。

### 1.5 opener 机制

- **R-401** ✅ THE SYSTEM SHALL 用 `Opener{Name, Cmd, Roles}` 声明打开方式；`Cmd` 用 `$0/$1...` 占位路径槽位。
- **R-402** ✅ THE SYSTEM SHALL 用 `Role` 枚举（`open-dir`/`open-file`/`diff-dir`/`diff-file`）约束 slotCount（1 或 2）；同 Opener 的所有 Role slotCount 必须一致。
- **R-403** ✅ THE SYSTEM SHALL 缺省 `Roles` 视为 `["open-dir"]`。
- **R-404** ✅ WHEN Cmd 无占位符 THE SYSTEM SHALL 把路径追加到 Cmd 末尾（兼容 `["code"] + path`）。

### 1.6 配置管理

- **R-501** ✅ THE SYSTEM SHALL 从 `~/.config/cube/config.json` 加载配置（`-c` 覆盖目录）；按 domain 分节（log / project{scan,clone} / openers）。
- **R-502** ✅ THE SYSTEM SHALL 配置缺失/解析失败时不阻断启动（降级优先）。
- **R-503** ✅ THE SYSTEM SHALL 提供配置写接口（Web 增删 scan/clone/opener；CLI `config edit` 手改）。
- **R-504** ✅ WHEN 配置文件变更 THE SYSTEM SHALL（web server 运行时）热 reload 到 project/opener service，无需重启。
- **R-505** ✅ THE SYSTEM SHALL 原子写配置（tmp + rename）。

### 1.7 Web 前端

- **R-601** ✅ THE SYSTEM SHALL 提供本地 Web UI（无构建链，Alpine.js + 纯 HTML/JS/CSS，go:embed 进二进制）。
- **R-602** ✅ THE SYSTEM SHALL 提供 Projects 页（表格/树双模式切换，共享筛选条件；搜索 + group/git/tag 筛选 + git 状态徽章）。
- **R-603** ✅ THE SYSTEM SHALL 提供项目详情抽屉（点行滑出，展示基本信息 + git 状态 + 动作）。
- **R-604** ✅ THE SYSTEM SHALL 提供 hash 路由（`#/projects[/tree]` / `#/config` / `#/p/<path>`），F5 刷新停留当前页。
- **R-605** ✅ THE SYSTEM SHALL 提供 Config 页（增删 scan/clone/opener 规则，写回 config.json）。
- **R-606** ⏳ THE SYSTEM SHALL 提供批量操作（Pull / Open 多项目）—— 待后端批量 API（见 Tasks M2）。

### 1.8 Alfred 集成

- **R-701** ✅ THE SYSTEM SHALL 提供 Alfred Script Filter JSON 输出（项目搜索按 frecency 排序、opener 搜索）。
- **R-702** ✅ THE SYSTEM SHALL 在 Alfred 流程记录项目选中/打开日志（history），用于 frecency。

### 1.9 git 增强（gitx）

- **R-801** ✅ THE SYSTEM SHALL 提供批量推送（push 分支/tag 到多 remote，TTY 多选，支持 force-with-lease）。
- **R-802** ✅ THE SYSTEM SHALL 提供多 remote ahead/behind 宽表查看（remote-status）。

---

## 二、Design（设计）

> 本节是技术契约的浓缩。详细推理与"为什么"见 `docs/design/v3-design.md`。

### 2.1 分层架构与依赖纪律

```
基础设施  config / db / logger / version                              所有层共享
能力      opener / util(git / gogit / fuzzy / easycache / pathkit / slicekit)  通用动作, 不含业务实体
领域      project (含 gitcache / scan / clone)                        业务 domain, 含实体和规则
出口      cmd / web                                                   把领域包成 CLI/Web
装配      app / main                                                  接线
```

- 基础设施不依赖上层；能力层只依赖基础设施；领域层依赖能力+基础设施；**cmd 与 web 不互调**；`app` 是唯一接线点（`app.Default()` 用 `sync.Once` 懒初始化）。
- **加一个新 domain = 五处加法**：①领域包 ②`cmd/<x>` 子命令组 ③`web.NewXxxHandler` ④config 加节 ⑤`app/init.go` 装配清单加构造。**五处都是加法，不碰现有 domain**。

### 2.2 数据模型

**project 域**（`project/`）
- `Project{path, group, name, tags, gitInfo}` — path 是绝对路径主键，name = `group:subpath`。
- `ScanRule{Group, Path, MaxDepth}` — 值对象（v2 的 Workspace 已降级为 group 字符串）。
- `CloneRule{RepoHost, RepoPrefix, LocalPath}` — host 相等 + path 前缀匹配，多条取 prefix 最长者。
- `gitcache.Entry{RepoUrl, CurrentBranch, DefaultBranch, Branches, Ahead, Behind, Dirty, WorktreeMain, CollectedAt}`。
- `Service` — 聚合根，持 scanRules/cloneRules/gitCache/scanCache，并发安全（RWMutex + easycache）。

**opener 域**（`opener/`）
- `Opener{name, cmd, roles, slotCount}` — cmd 用 `$0/$1` 占位，slotCount 由 roles 推导。
- `Role` 枚举：`open-dir`/`open-file`（slotCount=1）、`diff-dir`/`diff-file`（slotCount=2）。

**history 域**（`history/`，gorm + sqlite）
- `ProjectSelectLog{Project, Alfred}`、`ProjectOpenLog{Project, Opener, Alfred}` — 都嵌 `gorm.Model`。

### 2.3 关键机制

- **gitcache 异步采集**：读命令从 `~/.config/cube/cache/git.json` 读快照（不阻塞）；后台 fork 子进程采集回写，TTL 1 分钟内不重复，flock 串行化。详见 `docs/design/v3-design.md` 第四节。
- **opener role + slotCount**：能力由 `roles []Role` 声明，`slotCount = roles[0]` 推导（同 opener 所有 role slotCount 一致）。详见 `opener/role.go`。
- **config 热 reload**：`app.WatchConfig`（fsnotify 监听配置目录，300ms 去抖）→ `App.Reload`（重载 config + 调 project/opener Service.Reload）。web.Server 端口/路由不重建。
- **App 懒初始化**：`app.Default()` 首次调用才构造各 service。`cmd/*` 通过 `app.Default().XxxService()` 访问。

### 2.4 配置契约

默认目录 `~/.config/cube/`，`config.json` 结构（详见 `config/config.go`）：

```json
{
  "log":     { "path": "...", "level": "...", "format": "..." },
  "project": {
    "scan":  [{ "group": "...", "path": "~/Code", "maxDepth": 3 }],
    "clone": [{ "repoHost": "github.com", "repoPrefix": "/heyuuu", "localPath": "~/src" }]
  },
  "openers": [{ "name": "code", "cmd": ["code", "$0"], "roles": ["open-dir"] }]
}
```

运行期状态（不在 config.json）：sqlite `data.db`、`cache/git.json`、`cache/git.lock`、日志文件，均在配置目录下。

### 2.5 出口契约

**CLI**：`cube` + 子命令组（`project` / `opener` / `gitx` / `alfred` / `server` / `ui` / `config` / `version` / `debug`）。全局 flag `-c`（配置目录）/ `-d`（debug）。完整命令树见 `docs/design/v3-design.md` 第六节。

**Web**：`web.NewServer(handlers ...Handler)`，每个 domain 是一个 `Handler`，`Register(api huma.API)` 注册路由。统一 `ApiOutput{ok,message,data}` envelope。强制 `/api/` 前缀。huma 自动生成 OpenAPI 3.1（`/docs` Scalar + `/openapi.json`）。当前 15 个路由（11 GET + 4 写），详表见 v3-design.md 第五节（**注：该节"全 GET 只读"描述已过时，以代码 `web/api_*.go` 为准**）。

**前端**：无构建链，Alpine.js（v3.14.1，go:embed）。资源在 `ui/`，main 包 `//go:embed ui` 注入 web。hash 路由。详见 `docs/design/v3-frontend.md` 顶部"方案切换说明"。

### 2.6 测试策略

- `internal/testfixture`：共用测试辅助（Workspace 落 `runtime/test/`、BuildGitRepo 建真仓库、MakeProjectDir 建工程目录）。
- 纯函数：表驱动。IO 类（gogit/git/gitcache）：testfixture 建真实临时仓库。sqlite 类（history）：`:memory:`。扫描类（project）：testfixture + 真 Service。
- 不测的（靠手动/集成）：`git.Run`（透传 stdio）、`gitcache.TryAsyncRefresh`（fork 子进程）、`opener.Open`（实际启动）、`config`/`db` 单例、`cmd/*`、`web` handler。详见 `AGENTS.md` 测试章节。

---

## 三、Tasks（未来工作）

> `- [ ]` checklist，按里程碑分组。每项应足够小、可独立完成、可验证。

### M2 · git 操作 + 批量

- [ ] project 暴露 pull/fetch 操作（util/gogit 补 fetch/pull）
- [ ] `POST /api/project/pull`（单项目）
- [ ] group 维度批量 API（路径形态待定：动词式 `POST /api/project/group/{g}/pull` vs `POST /api/project/pull-group`）
- [ ] 批量操作并发/失败策略（决策点：全失败回滚 vs 部分成功 vs best-effort 报告）
- [ ] Web 前端激活批量条 + Pull 按钮（当前 disabled 占位）

### M3 · worktree

- [ ] `cube project worktree add/list/remove`
- [ ] worktree 物理位置约定（决策点：固定 `~/worktrees/` vs 项目 `.git/worktrees/`）
- [ ] worktree 是否纳入扫描的约定（当前扫描会打 worktree tag，但无管理入口）
- [ ] Web 前端激活详情抽屉的 worktree 块（当前占位）

### M4 · Web 前端完善

- [ ] `GET /api/project/diff?path=`（返回结构待定：patch 文本 vs 结构化 hunks）
- [ ] Web 前端 DiffPage
- [ ] 行操作 Open▾ 激活（后端 `/api/project/open` 已就绪，前端按钮当前 disabled）
- [ ] group 批量 Open▾

### M5 · MCP 出口

- [ ] 接入官方 go-sdk
- [ ] 按 domain 分 toolset（project / opener / gitx）
- [ ] cube 作为 MCP server 被外部 AI 平台编排

### M6+ · 新 domain

- [ ] `cube db`（sqlite 管理）
- [ ] 本地端口查看

### 文档与一致性纠偏（技术债）

- [ ] **重新生成 `openapi.json`**：当前只含 2 个路由，实际有 15 个。跑 `cube server openapi --out openapi.json`。
- [ ] **v3-design.md 第五节"当前 API 全 GET 只读"修正**：实际已有 POST `/api/project/open` + 6 个 config 写接口。
- [ ] **v3-design.md 第八节 M4 状态更新**：F1 骨架已落地（标 ✅ 而非 ⏳）。
- [ ] **AGENTS.md opener slot 描述修正**：提到 `opener/slot.go` 和 `command_test.go` 实际不存在；opener 模型是 role + slotCount（`opener/role.go`）。

### 行为一致性

- [ ] **history 写入不一致**：当前仅 alfred 命令写 history（select/open log），CLI/Web 的 open 不写。决策点：是否让所有 open 路径都记 history？（影响 frecency 准确性）
- [ ] **`project info` 命名名实不符**：Use 写"打开项目"但代码只打印信息（`cmd/project/project.go:132`）。决策点：改名 / 改实现 / 拆分。
- [ ] **CLI config 写命令缺失**：Web 有增删 scan/clone/opener，CLI 只能 `config edit` 手改。决策点：是否补 `cube config scan add/remove` 等。

### 已知 gotcha（长期关注）

- [ ] logger 用 `runtime.Callers` + `../../` 推算项目路径，移动 logger 源文件会让日志 file 相对路径错位。
- [ ] history 的 gorm `Where(&struct{Alfred:false})` 把 false 当零值忽略，导致 `LeastSelectedProjects(alfred=false)` 返回含 alfred=true 的记录（测试已标注现状，待修）。

---

## 四、变更日志

| 日期 | 变更 |
|------|------|
| 2026-08-03 | 初始反推版。基于 develop 分支代码现状提炼 Requirements（已落地功能）+ Design（契约浓缩）+ Tasks（M2-M6 + 技术债）。发现并记录 4 处文档与代码不一致。 |
