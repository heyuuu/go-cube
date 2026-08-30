# 1038 opener 默认与意图

## 状态

**已实现并归档**（2026-08-31）。实现过程分五步落在 develop 分支：`4e08750`（cmd 字符串化）→ `66e3d77`（per-role commands）→ `a654ee0`（actions 动作串 exec:/url:）→ `b016d1b`（actionOpener 更名）→ `6c6affa`（intent + openerIntents 节）→ `de74a2d`（前端快捷位意图化）。设计经三轮讨论演进，最终形态与最初定稿差异较大，以本文「最终设计」与现状.md 3.3 为准。

## 背景

opener 列表原本是纯能力清单，无「默认」概念：CLI 不传 `-o` 进交互选择、Web 打开弹全量下拉。固定工作流（diff 恒用同一工具、终端恒用同一终端）每次都要选一遍。讨论中发现更深的结构问题：role 身兼「槽约束」与「场景」两职，且一个 opener 只有一条 cmd、slotCount 一致性约束让 vscode 无法同时声明打开与对比。

## 最终设计

### 一、数据模型演进（前置改造）

1. **cmd 数组 → sh 风格字符串**：`tokenizeCmd` 严格分词（未闭合引号构造期中文报错）；`$0/$1` 占位符支持 token 内子串替换（`--wd=$0`）；替换发生在分词后，路径含空格安全。
2. **cmd/roles → per-role actions map**：`actions: { [role]: cmd }`，声明 roles = 键集合（能力声明：stree 只声明 open-dir 则不出现在文件打开候选里）；slotCount 一致性约束删除（同 opener 不同 role 可配不同命令）。
3. **commands → actions 动作串**：`<kind>:<模板>`，kind ∈ {`exec:`, `url:`}，前缀必填、只认首个冒号。`url:` 支持 `/` 开头站内路由（拼 `web.BaseURL(port)` + 占位符 query encode，解决 dev/prod 端口不同无法静态配置站内链接）与 `http(s)://` 外部链接；实现仍为 `actionOpener` 单实现内分流。
4. **前端 url 直开**：页面内打开入口对 url 动作不回后端（同源 + 不跳出当前浏览器），`tryOpenUrlAction` 与后端同规则渲染，分流收敛在 `useOpenerOpen().run()` / `useProjectOpen().run()`。

### 二、intent（打开意图）+ openerIntents 节

- **role 保持 4 值不变**（open-dir/open-file/diff-dir/diff-file）：纯槽约束 + 能力声明。dir/file 之分是真实的（不是所有程序都同时支持目录与文件）。
- **intent 开放枚举**（多对一映射 role）：`dir/file/diff-dir/diff-file/terminal/git/workbench/doc`。intent 是选择层概念，`Opener.Open(role)` 签名不动；加 intent 不要求任何 opener 配置跟着改。
- **settings 新增 `openerIntents` 节**（openers 节保持纯清单）：`{ [intent]: {defaultOpener?, openers?} }`，候选缺省 = 声明对应 role 的全部 opener（为后续按意图筛选 opener 做准备）。
- **双重校验**：写侧 opener 须存在且声明对应 role（必死配置不落盘）；读侧失效条目跳过 + slog.Warn。`DeleteOpener` 连带清引用。
- **特化 intent 未配默认不回落**（terminal 不回落 dir），前端槽位隐藏。

### 三、入口行为

- **CLI `-o` 三态**：不传 → 该 intent 默认直开（未配置报错引导）；裸 `-o`（NoOptDefVal 哨兵）→ 交互选择；`-o <name>` → 模糊匹配（多项 TTY 交互，原语义保留）。
- **Web**：新增 `GET /api/opener/intents` + `intent-default/save|delete`；设置页 Opener 分区尾部「打开意图」区块（每 intent 一行默认下拉）。
- **前端快捷位意图化**：`quickOpens` 硬编码（finder/stree/cube-workbench）退役，`quickIntents` 槽位（workbench/git/terminal/dir × 目标策略 root-only/repo-roots/all）取默认 opener；workbench 副本行同步（dir/git）。

## 已知取舍

- **usage 缺口**：url 动作前端直开不经 `project/open`，不计 usage（影响「最近使用」排序信号）。决定先不管，等有排序异常的实际痛感再补上报。
- **兼容读始终未做**：每轮形态变更都直改 dev 配置（`~/.config/cube-dev/settings.json`）迁移；正式配置由用户在安装新版本时手工迁移。
- **`Opener` 接口保留**：虽已只剩 `actionOpener` 一个实现，保留接口以减少对外暴露面、方便内部重构。

## 演进中被否决的方案（备查）

- **intent 与槽签名合并成单一枚举 / role 收敛为纯槽数（open/diff 两值）**：丢失「opener 声明支持目录还是文件」的能力表达（stree 只能开目录）。
- **`<default>` 特殊标识**：改为 CLI 不传 -o 即默认、前端自己解析 intents 数据，标识机制取消。
- **defaults 与 openers 同节 `{list, defaults}`**：改为独立 `openerIntents` 节，解耦更浅。
- **1039 match 谓词路由**（按语言/仓库根条件路由）：被「调用点按目标属性选 intent」替代——未来 workspace 语言落地后加 `dev-go` 类 intent 即可，opener 声明不用带谓词。
