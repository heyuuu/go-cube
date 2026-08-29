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

- 1038 已落地（默认机制就绪）。
- match 方案 A/B 选型讨论完成；若选 A，需确认前端硬编码策略（stree / cube-workbench）迁移方案一并设计。
