# 1038 opener 默认

## 状态

活动，**设计定稿，未实现**（曾完成一次实现后整体回退——改动面偏大，设计继续演进中，另起会话重新开发）。设计定稿如下。

## 背景

opener 列表目前是纯能力清单，没有任何「默认」概念：CLI `open` / `diff` 不传 `-o` 时进入交互选择，Web `opener/open` / `project/open` 不传 opener 时前端弹全量下拉。对固定工作流（如 diff 恒用同一工具）是重复操作。

## 需求

每个 role 一个默认 opener，未显式指定时可自动使用。

## 设计定稿

### 数据：settings.json `openers` 节（不兼容变更）

默认与列表**同节存储**——读写都需联动校验（默认 opener 必须在列表中存在），放一节内做单写者一致性最自然：

```json
{
  "openers": {
    "list": [
      { "name": "finder", "cmd": ["open", "$0"], "roles": ["open-dir"] }
    ],
    "defaults": { "open-dir": "finder", "diff-file": "kaleidoscope" }
  }
}
```

- **旧形态（openers = Spec 数组）硬切不兼容**：读侧解析失败按既有节级降级为空，存量 openers 用户手工迁移（无代码兼容读）；写侧恒写新形态。
- defaults 键 = role 枚举（4 个），值 = opener name；读侧失效条目降级跳过，写侧校验 name 必须现存。

### 特殊标识 `<default>`

各 open 调用点（HTTP 与 CLI 的 opener 选择参数）**不接收空字符串回退**——空/缺省维持报错（避免误传）；请求默认时显式传特殊标识 **`<default>`**，服务端解析为「该 role 的默认 opener」（失效时报中文错误）。此标识机制为后续更多特殊标识预留。

### opener name 校验规则

保存 opener 时校验 name：`^[a-z](?:[a-z0-9_-]*[a-z0-9])?$`（首字符小写字母，中间 `[a-z0-9_-]`，末字符字母/数字，不允许 `-`/`_` 结尾）——排除 `<` 等字符，与特殊标识（`<...>` 形态）结构性区分。写侧拦截，读侧沿用坏条目跳过。

### 领域逻辑（`opener` 包）

- `Service` 增方法：`DefaultOpener(role) (Opener, error)` / `SaveDefaultOpener(role, name)` / `DeleteDefaultOpener(role)` / `ResolveOpener(name, role)`（`<default>` 标识解析收敛单点）。
- 节形状改为 struct（list + defaults），加载/保存联动：`DeleteOpener` 连带清指向被删名的默认。
- name 校验收敛进 `InitExecOpener` 构造校验（写侧入口）。

### Web API

- `GET /api/opener/overview` — **统一输出** `{openers: [...], defaults: {...}}`，代替旧 `GET /api/opener/list`（旧接口暂留，前端替换后移除）。
- `POST /api/opener/default/save` — body: role + opener。
- `POST /api/opener/default/delete` — body: role。
- `opener/open` / `project/open` 的 opener 参数：空/缺省报错（维持现状），`<default>` 走默认解析。

### CLI

`open` / `open-path` / `diff` 不传 `-o` 时**恒交互**（保持现状，不静默用默认）；`-o <default>` 显式请求默认（收敛在 `pickOpener` 的标识解析）。是否改为「无 -o 时默认直开」留待完成后单独讨论（单纯入口问题，后续好改）。

### 前端

- `queries/opener.ts`：overview 查询 + default/save/delete mutation。
- 设置页 Opener 分区：每个 role 一行「默认」下拉（候选 = 该 role 的 opener + 无默认，悬挂引用标失效）。
- 打开调用处：全量「打开方式」下拉中默认项标「默认」徽标；传给后端的 opener 恒显式（名字或 `<default>`），不依赖空串语义。

## 与 intent 演进的关系（重要）

per-role 默认在讨论中被发现粒度不够（见 1039 新增的「intent 拆分」章节）：用户还需要「terminal 的默认」「git 客户端的默认」等更细场景的默认，而这些都是 `[dir]` 槽——role 键空间装不下。**重新开发前需先决策**：直接按 1039 的 intent × 槽签名模型做（defaults 键 = intent，role 退役），还是先落本提案的 role 版再演进。倾向前者，避免两次不兼容变更。

## 验证

- 纯函数 + settings 读写：表驱动测试（节形态解析、默认失效降级、name 正则、`<default>` 解析、写侧联动校验）。
- CLI 回退链与 API：手动验证。
