# 1039 opener 适用范围 match

## 状态

挂起，待讨论。前置依赖：`1038-opener默认`（本提案的 match 命中是「条件默认」，默认机制是兜底层）。

## 背景

opener 目前只有 role（能力/槽签名），没有「适用范围」概念。三类场景缺表达：

1. **仓库根 vs 普通目录**：JetBrains 系只想吃仓库根，Finder 想通吃所有目录——现在前端靠硬编码策略筛（stree 仅仓库根、cube-workbench 仅主根），无法配置。
2. **按后缀**：`.md` 想 default 用 Typora，`.go` 想 default 用 Goland。
3. **按项目语言**：golang 项目优先 goland，前端项目优先 webstorm——个人多语言工作流的刚需。

## 已定结论（讨论收敛点）

- **不把适用范围塞进 role**：role 保持「槽签名」纯度（dir/file × 1/2 槽，`$0/$1` 占位符校验依据），repo-root / 后缀 / 语言是 dir/file 的子范围，不属于 role。
- **open-path 无项目语义**：非项目内路径按普通目录/文件参与匹配，`langs` 谓词不命中，自然回退默认。
- **语言粒度到 workspace**：monorepo 天然多语言，语言是 workspace（含 worktree）级属性而非项目级；主根与各 workspace / worktree 各自判定。
- **match 方案本体待议**（见下），挂起到需要时再讨论。

## intent 拆分（✅ 已由 1038 落地，2026-08-31）

本章的 intent 拆分已随 `1038-opener默认`（已归档）实现：intent 8 值枚举多对一映射 role、`openerIntents` 节（默认 + 候选）、CLI `-o` 三态、前端快捷位意图化。原设计差异备查：槽签名未随 intent 拆分（role 保持 4 值承担槽约束 + 能力声明）；「特化 intent 未配默认不回落」按本章结论落地。本章余下的讨论记录保留如下。

> 以下为原始讨论：

讨论中发现 role 身兼两职——**业务意图**（open vs diff vs terminal vs git 客户端）与**槽签名**（dir/file × 1/2 槽，`$0/$1` 校验依据）——导致 per-role 默认粒度不够：用户需要「terminal 的默认」「git 客户端的默认」等更细场景默认，而它们的槽签名都是 `[dir]`。且现状 `ParseRoles` 要求 opener 全 role slotCount 一致，使 vscode 无法同时声明 open-dir（1 槽）与 diff-dir（2 槽）——槽个数本是调用时的属性。

**拆分模型**：

- **槽签名**（机械维度，调用时携带）：`[dir]` / `[file]` / `[dir,dir]` / `[file,file]`。
- **intent**（场景维度，defaults 的键空间，按 cube 实际入口枚举）：

| intent | 场景（谁在发起） | 槽形态 | 现状 | 默认举例 |
|--------|-----------------|--------|------|---------|
| `dir` | 通用打开目录：项目列表打开、workbench/目录树目录节点、open-path 目录、open | `[dir]` | role open-dir | vscode |
| `file` | 通用打开文件：目录树文件节点、open-path 文件 | `[file]` | role open-file | vscode |
| `diff` | 对比两个路径：cube diff、diff 面板外部对比 | 调用时定 | role diff-dir/diff-file（被 slotCount 一致性绑死） | bcompare |
| `terminal` | 在终端打开目录 | `[dir]` | 无 | ghostty |
| `git` | 在 git 客户端打开仓库（恒仓库根） | `[dir]` | 前端硬编码 stree 快捷位 | stree |
| `workbench` | 在 cube 工作台打开 | `[dir]` | 前端硬编码 cube-workbench 快捷位 | cube-workbench |
| `doc` | 文档查看打开 | `[dir]`/`[file]` | 前端 cube-md | cube-md |

- opener 声明「支持哪些 intent」+ 占位符最大个数，slotCount 一致性约束消失；`<default>` 解析按 intent（`ResolveOpener(name, intent)`）。
- **前端 quickOpens 三个硬编码快捷位（finder/stree/cube-workbench）= `dir`/`git`/`workbench` 三个 intent 的具名化**——落地后快捷位改为「取该 intent 的默认 opener」，配置 defaults 即换快捷位，消掉硬编码。
- 特化 intent（terminal/git/...）未配默认时**不回落** `dir` 默认（避免「终端按钮打开了 VS Code」），入口隐藏或提示配置。
- role 枚举整体退役（倾向双轨更乱），`ParseRoles`/roleSlots 随之拆除。
- 与 match 的关系：两层正交，解析顺序 match 路由 > intent 默认 > 交互；match 谓词同样按 intent 路由，共用 opener 声明。

## actions 动作串（2026-08-31 定稿，已实现）

opener 的 per-role `commands` 进一步升级为 `actions: {role: "<kind>:<模板>"}`：

- **`exec:`** = 原命令形态（sh 风格分词、`$N` 占位符、Executor 子进程），行为不变。
- **`url:`** = 打开链接：`/` 开头为站内路由（`url: /workbench?path=$0`），服务端拼 baseURL（config `server.port` 装配注入）+ 占位符 query encode 后经系统 opener（darwin `open`）打开——**解决 dev/prod 端口不同导致站内链接无法静态配置的问题**；`http(s)://` 开头为外部 URL，占位符照常替换。
- 前缀必填（无默认 kind），只认首个冒号，冒号后空格可选；非法 kind / 非法 url 值域 / 占位符越界在构造边界报中文错误。
- 仍是 `execOpener` 单实现内按 kind 分流（`BuildArgs` 统一返回 bin+args），未新增第二个 Opener 实现。

## 候选方案（待选型）

### 方案 A：声明式 match + 复用列表序当优先级（当前倾向）

opener Spec 增加可选 `match` 谓词：

```json
{
  "name": "goland",
  "cmd": ["goland", "--new-window", "$0"],
  "roles": ["open-dir"],
  "match": { "langs": ["go"], "scopes": ["repo-root"] }
}
```

- `scopes`: `repo-root` / `dir` / `file`；`suffixes`: `[".md"]`；`langs`: `["go"]`。
- 选择算法：openers 列表按序找第一个「role 匹配 + match 命中」的条目——**列表本就支持拖拽排序，顺序即优先级**，无需独立优先级字段或中心化路由表。
- 查找顺序：match 命中 > `openerDefaults` 默认（1038）> 现有交互/全量下拉。
- 优点：配置就地、UI 复用（编辑表单加 match 区块 + 已有拖拽）、可顺带消掉前端硬编码策略（stree / cube-workbench 筛选迁到 match）。
- 缺点：多 opener 各带 match 时优先级靠列表序隐式表达，「为什么选了它」不直观。

### 方案 B：中心化路由表

settings.json 增有序规则节（first-match-wins），规则 = `{match, role, opener}`。

- 优点：优先级显式集中，规则可表达例外/覆盖。
- 缺点：多一个概念、一套 UI；个人工具场景规则量级存疑。

倾向 A，理由：复用现有拖拽序即优先级，概念增量最小。

## 语言判定的数据来源（无论 A/B 都需要）

- 语言来自 **projcache 采集侧**：扫 `go.mod` / `package.json` / `Cargo.toml` 等声明文件（规则格式可类比 `workspaceScanRule` 的 `|`+`+`），结果作为快照字段写 git.json——读路径零探测（架构纪律：读路径不得阻塞采集）。
- 粒度到 workspace / worktree（见已定结论），主根与子目录各自带语言。
- 文件后缀不需要采集，pathkit 现算。

## 解挂条件

- ~~intent 拆分先行落地~~（✅ 已随 1038 归档完成；前端硬编码策略（stree / cube-workbench 快捷位）也已随之意图化，不再需要本提案迁移）。
- match 方案 A/B 选型讨论完成。注意：讨论方向已转为「调用点按目标属性选更细 intent」（如未来 workspace 语言落地后的 `dev-go`），match 谓词可能整体不再需要——解挂讨论时应先重估这个前提。
