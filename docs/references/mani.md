# mani 竞品分析

> [alajmo/mani](https://github.com/alajmo/mani) | 官网: manicli.com | 语言: Go | License: MIT
>
> 分析日期: 2026-08-05

## A. 核心功能与定位

**解决的问题**: 开发者拥有多个仓库（微服务、monorepo 拆分后的多库、个人多项目集合）时的"散装管理"痛点 —— 手动逐个 clone、逐个切目录跑命令、记不清哪个项目该跑什么命令、团队无法共享"项目拓扑"。

**核心能力**（6 项）:

1. **声明式配置** —— 一个 `mani.yaml` 描述整个工作区。
2. **批量命令执行** —— `mani exec` / `mani run` 跨项目跑命令，支持并行（`--parallel --forks N`）和多种输出格式（stream / table / markdown / html）。
3. **clone 与 sync** —— `mani sync` 根据 config 一键拉齐所有仓库；支持 worktree、remotes、gitignore 自动更新。
4. **灵活过滤** —— 通过 `tags` / `tags_expr`（布尔表达式）/ `paths` / `projects` / `cwd` 多维筛选目标项目。
5. **任务系统** —— `tasks` 把常用命令（git status、build、test…）命名固化进 config。
6. **TUI** —— `mani tui` 提供交互式选择和执行。

**边界（不做什么）**:

- 不做远程服务器管理（README 明确把这部分推荐给姊妹项目 `sake`）。
- 不做项目生成/脚手架（只是管理已有仓库）。
- 不做容器/编排（不是 `docker compose` 替代品）。
- 定位是"本地多仓库 + 团队共享拓扑"，而非 IDE、不是 Git 图形客户端。

## B. 核心概念与数据模型

mani 的数据模型有 **6 个核心实体**，这是个比单纯"项目列表"丰富得多的设计：

| 实体 | 作用 | cube-next 对标 |
|---|---|---|
| **projects** | 一个 git 仓库（name + path + url + tags + env + worktrees + remotes） | Project 实体 |
| **tasks** | 命名命令，可被 `run` 调用 | 无直接对标，是个亮点 |
| **specs** | 任务执行规范（output 格式、并行、forks、ignore_errors…） | — |
| **targets** | 项目筛选的"命名预设"，可复用 | — |
| **themes** | 输出/TUI 的可定制主题 | — |
| **env** | 全局/项目/任务三级环境变量 | — |

**关键设计点 1：三层 env 优先级**（global → project → task → CLI flag），且支持 `$(...)` 命令替换：

```yaml
env:
  DATE: $(date -u +"%Y-%m-%dT%H:%M:%S%Z")  # 动态求值
projects:
  pinto:
    env:
      branch: main  # 项目级覆盖
tasks:
  git-create:
    env:
      branch: main  # 任务级覆盖
```

**关键设计点 2：project 不是"路径字符串"，而是一个富对象**。一个 project 自带：自定义 clone 命令、分支、single_branch、多个 remotes、多个 worktrees、tags。这远比 "一个 repo = 一个 url" 的模型有用。

**关键设计点 3：配置可组合**。

- `import:` 可引入其他 `mani.yaml`
- `tasks.commands` 里可用 `- task: simple-1` 引用其他 task（只复用 cmd/shell，不复用 target/spec，干净组合）
- `specs` 和 `targets` 可在 config 里定义一次，被多个 task 引用

**mani.yaml 完整结构示例**（精简，保留所有关键字段）:

```yaml
shell: bash
sync_remotes: false
remove_orphaned_worktrees: false
sync_gitignore: true
reload_tui_on_change: false

projects:
  pinto:
    sync: true
    path: frontend/pinto          # 可嵌套目录
    url: git@github.com:alajmo/pinto
    desc: A vim theme editor
    clone: git clone git@github.com:alajmo/pinto --branch main  # 可自定义
    branch:
    single_branch: false
    tags: [dev]
    remotes:
      foo: https://github.com/bar
    worktrees:
    - path: hotfix
    - path: feature-branch
      branch: feature/awesome
    env:
      branch: main

specs:
  default:
    output: stream                # stream | table | html | markdown
    parallel: false
    forks: 4
    ignore_errors: false
    ignore_non_existing: false
    omit_empty_rows: false

targets:
  default:
    all: false
    cwd: false
    projects: []
    paths: []
    tags: []
    tags_expr: ''                 # 布尔表达式，如 (prod || dev) && !test

tasks:
  simple: echo "hello world"      # 最简形式
  advanced:
    desc: complex task
    theme: default
    shell: bash
    spec: default                 # 引用命名 spec
    target: default               # 引用命名 target
    cmd: |
      echo complex
    # 或用 commands: 多步组合
    commands:
    - name: node-example
      shell: node
      cmd: console.log("hi")
    - task: simple                # 复用其他 task
```

## C. 命令设计

命令分组清晰，flag 风格统一（POSIX 风格短/长 flag）:

| 命令 | 作用 | 关键 flag |
|---|---|---|
| `mani init` | 生成 config + .gitignore | `--auto-discovery`（自动扫描 .git 子目录，默认 true） |
| `mani sync` | clone/sync 仓库 | `--parallel` `--status` `--sync-remotes` `--remove-orphaned-worktrees` |
| `mani exec <cmd>` | 跨项目跑任意命令 | `--all` `--parallel` `--output` `--forks` `--tags-expr` |
| `mani run <task>` | 跑 config 里的命名 task | 同 exec + `--describe` `--dry-run` `--tty` `--target` |
| `mani list projects/tasks/tags` | 列出实体 | `--tree` `--headers` `--tags` `--paths` |
| `mani describe projects/tasks` | 详情 | 过滤 flag |
| `mani edit [project/task] <name>` | 用 `$EDITOR` 编辑 config | — |
| `mani tui` | 进入交互界面 | `--reload-on-change` `--theme` |
| `mani check` | 校验 config | — |
| `mani gen` | 生成 man pages | `--dir` |

**命令设计亮点**:

1. **`exec` vs `run` 分离** —— 临时命令用 `exec`（ad-hoc），固化命令用 `run`（命名 task）。
2. **筛选 flag 在多个命令间复用** —— `--all` `--cwd` `--projects` `--paths` `--tags` `--tags-expr` 这套 flag 同时出现在 `exec`/`run`/`list projects`，学习一次到处能用。
3. **`tags_expr` 支持布尔表达式** —— `(prod || dev) && !test`，比单纯 tags 数组"AND"语义强得多。
4. **没有 `clone` 命令** —— clone 统一收敛进 `sync`，避免命令碎片化。

## D. TUI / 交互

mani 有完整 TUI（`mani tui`），基于 `tview`。从 config 里 `themes.tui` 的字段能反推出 TUI 的结构和交互模型：

**TUI 组件结构**（从 theme 配置推断）:

- `border` / `border_focus` —— 带焦点的边框面板
- `title` / `title_active` —— 可聚焦的面板标题（左右居中对齐）
- `button` / `button_active` —— 按钮控件
- `table_header` / `item` / `item_focused` / `item_selected` / `item_dir` / `item_ref` —— 列表/表格选择
- `search_label/text` —— 搜索框
- `filter_label/text` —— 过滤框
- `shortcut_label/text` —— 快捷键提示栏

**交互能力**: 项目选择 + 任务执行 + 实时搜索 + 过滤 + 多面板 + 快捷键。

**热重载**: `--reload-on-change` + `reload_tui_on_change` config 项 —— 改 config 自动刷新 TUI（用 `fsnotify` 监听）。

**`edit` 命令也是交互设计的一部分**: `mani edit project <name>` 直接跳转 `$EDITOR` 编辑，闭环 config 编辑体验。

## E. 技术栈

**语言**: Go（go 1.26.3）

**关键库**（从 go.mod 提取，对 cube 选型很有参考价值）:

| 用途 | 库 | 备注 |
|---|---|---|
| CLI 框架 | `spf13/cobra` v1.10.2 + `spf13/pflag` | Go CLI 事实标准 |
| TUI | `rivo/tview` v0.42 + `gdamore/tcell/v2` | 传统 widget 式 TUI（非 bubbletea 的 Elm 式） |
| 配置解析 | `gopkg.in/yaml.v3` | **没有用 viper**，直接 yaml + 自己组装默认值 |
| 配置热重载 | `fsnotify/fsnotify` | 监听 config 变化驱动 TUI 重载 |
| 表格输出 | `jedib0t/go-pretty` | table/markdown/html 输出 |
| 终端配色 | `gookit/color` | |
| Spinner | `theckman/yacspin` | 长任务进度 |
| 深拷贝 | `jinzhu/copier` / `otiai10/copy` | struct/config 复制 |

**代码分层**（顶层目录）:

- `main.go` —— 入口
- `cmd/` —— cobra 命令定义层
- `core/` —— 业务逻辑（config 解析、task 执行、sync、git 操作）
- `docs/` —— markdown 文档（config.md / commands.md / variables.md）
- `examples/` —— 示例 mani.yaml
- `test/` `benchmarks/` `scripts/` `res/`

## F. 值得 cube-next 借鉴的点

### ✅ 值得抄的设计

1. **富 Project 模型**（最重要）。mani 把 project 做成 "url + path + tags + env + worktrees + remotes + 自定义 clone" 的对象，而非扁平路径字符串。cube 的 Project 实体应该往这个方向走（但渐进式，初期不需要全上）。
2. **三层 env 优先级 + `$(...)` 动态求值**。项目级 env + 命令替换对"批量跑命令"场景非常实用（属于 workspace 阶段能力）。
3. **`tasks` + `specs` + `targets` 三件套抽象**。把"跑什么"(task)、"怎么跑"(spec)、"对谁跑"(target) 正交分离，可命名复用。
4. **`tags_expr` 布尔表达式过滤**。`tags_expr` 远比 tags 数组"AND"强。cube 若做 tag 过滤要定义清楚表达式语法（但初期不做，过度设计）。
5. **`exec` vs `run` 二分**。临时命令 vs 固化任务，命令面清晰。
6. **`sync` 收敛 clone**。不单独搞 `clone` 命令，sync 一个命令搞定"拉齐所有仓库"，幂等可重跑。**（cube 初期不采用——个人工具单次 clone 更直观）**
7. **筛选 flag 跨命令复用**（`--all/--tags/--paths/--projects/--cwd`）。**cube 直接借鉴**：`--group/--tag/--path` 跨命令统一。
8. **配置 import + task 引用**。大型工作区可拆分 config，task 可组合。
9. **TUI 热重载**（fsnotify 监听 config）。开发体验极佳。
10. **`edit` 命令直通 `$EDITOR`**。config 编辑闭环。
11. **声明式 + 团队共享**。`mani.yaml` 可提交到 git，新人 `mani sync` 一键拉齐所有仓库 —— 这是 mani 最核心的产品价值。**cube 是个人工具不直接采用，但思路值得理解。**

### ⚠️ 要避坑的点

1. **config 字段爆炸**。单个 project 有十几个字段，specs/targets/themes 每个又是一堆选项，主题里光 TUI 就 20+ 样式字段。**门槛偏高**，普通用户会被吓退。cube 应该：默认零配置可用，复杂选项渐进式暴露。
2. **themes 把样式塞进主 config**。`mani.yaml` 里混入大量颜色/边框/对齐配置，污染业务语义。cube 坚决不学——样式独立。
3. **YAML 单文件承载一切**。projects 多了之后，单文件会膨胀。虽然有 `import:`，但 import 是扁平合并，没有 namespace。cube 已定的分文件设计（config/rules/history/cache 分离）更好。
4. **`tags_expr` 的表达式语法未明确文档化**。
5. **命令可能过度细分**。`list projects/tasks/tags` + `describe projects/tasks` + `edit project/task` —— list/describe 的边界对用户不直观。
6. **依赖 `tview` 而非 `bubbletea`**。tview 是传统 widget 式，生态活跃度不如 bubbletea（Charm 生态）。cube 新做 TUI 优先 bubbletea/huh。
7. **缺少项目状态/健康度**。mani 是"拓扑 + 命令执行"，但不展示"哪个项目有未提交改动/落后远端多少 commit"。**cube 的 gitcache 正是差异化机会。**

---

**一句话总结**: mani 的**实体建模**（project/task/spec/target/env + import 引用 + 三层 env）和**命令正交设计**（exec/run 分离、flag 复用、sync 收敛 clone）是教科书级的，值得 cube-next 深度借鉴；但要避免它的**配置膨胀**（样式混入业务 config、字段过多）。cube 的差异化在于 **gitcache 项目健康度可视化**——这是 mani 完全没有的。
