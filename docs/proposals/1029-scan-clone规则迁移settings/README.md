# scan / clone 规则迁移 settings.json

> **状态**：📝 待评审
>
> **关联**：[`1016-opener改造`](../archived/1016-opener改造/README.md)（settings.json 节级 API + 领域 Service 直读模式的首个落地）；[`1025-settings配置页`](../archived/1025-settings配置页/README.md)（已预留「项目·扫描」分区占位，本提案即其注明的「project 域另立提案」）；[`1026-环境分离`](../archived/1026-环境分离/README.md)（settings.json 保持文本形态、可人工 diff 合并）。

## 背景与目标

扫描规则（`project.scan`）与克隆规则（`project.clone`）目前存 config.json，走「启动加载 → `app.go` 装配 → `project.Service` 构造期固化」链路，无热更，Web 端只能只读展示（settings 页 Config 过渡分区 + `GET /api/project/scan-rules` / `clone-rules`）。

1025 已确立职责边界：**settings = 用户可管理、可运行中变更的数据（openers、扫描规则…）；config = 启动期读一次的事实**。scan/clone 规则正是典型的用户可管理数据，本提案把它们迁入 settings.json，并补齐 Web 增删改能力，落地 settings 页的「项目·扫描」分区。

目标：

1. scan / clone 规则数据源从 config.json 迁到 settings.json 的 `scanRule`/`cloneRule` 节；
2. `project.Service` 对规则改为**直读不缓存**（同 opener.Service 模式），Web 保存即生效，无缓存不一致；
3. settings 页「项目·扫描」分区实装（扫描 + 克隆两组规则的增删改）；
4. 存量数据自动迁移，用户无感。

## 方案

### 1. settings.json 节设计

两个顶层节，各自归 project 域所有（分节存储可独立读取、校验与写入，与 openers 节同构）：

```json
{
  "openers": [ ... ],
  "scanRule":  [ { "group": "work", "path": "~/Code/work", "maxDepth": 3 } ],
  "cloneRule": [ { "repoHost": "github.com", "repoPrefix": "heyuuu", "localPath": "~/Code/gh/heyuuu" } ]
}
```

- 节名常量归领域包（`scanRuleSection` / `cloneRuleSection`），读写走 settings 节级 API（`LoadSection` / `SaveSection`），一次保存写一节；
- 存储形态与领域形态同构（`ScanRule` / `CloneRule` 同一 struct），条目字段不变（group/path/maxDepth 与 repoHost/repoPrefix/localPath）；读侧转换做 `~/` 展开与降级校验。`config.ProjectConfig` 等类型删除。

### 2. project.Service 直读化

- `NewService` 不再接收 `config.ProjectConfig`，规则读取改为每次调用时从 settings.json 直读（读侧条目级坏数据跳过、目录不存在降级——沿用现 `NewService` 内的 `~/` 展开与校验逻辑，只是从构造期挪到读期）；
- `ScanRules()` / `CloneRules()` / `MatchScanRule` / `MatchCloneRule` 签名不变，消费者（cmd 的 init/check/clone、handlers）零改动；
- 写侧新增 `SaveScanRule` / `DeleteScanRule` / `SaveCloneRule` / `DeleteCloneRule`（含 reorder，见下），写前走领域校验（路径可展开、maxDepth 合法、clone 规则 host/prefix 合法），坏数据返回中文错误、落不了文件——校验收敛在读写边界，同 opener 模式。

### 3. Web API（规则 15：GET/POST，语义进 API 名）

现有：

- `GET /api/project/scan-rules`、`GET /api/project/clone-rules`（保留，改读 settings）

新增（对齐 opener 的一组）：

- `POST /api/project/scan-rule/save` / `scan-rule/delete` / `scan-rule/reorder`
- `POST /api/project/clone-rule/save` / `clone-rule/delete` / `clone-rule/reorder`

reorder 的顺序即 settings.json 数组序，与 opener 分区拖拽排序语义一致（clone 规则顺序影响 `MatchCloneRule` 的「prefix 最长优先」前的兜底序，scan 规则顺序影响 group 归属优先级，用户可见可排）。

### 4. 存量迁移（手动，同 opener 先例）

不做代码迁移。上线后由用户手动把 config.json 的 `project` 节拆成 `scanRule` 与 `cloneRule` 两个数组搬到 settings.json 顶层（jq / 编辑器均可），再从 config.json 删除 `project` 节——与 1016 opener 迁移同先例（「迁移未做代码，用户手动 jq 迁移」）。dev/prod 双环境（1026）各搬一次。

代码侧行为：settings.json 的 `scanRule`/`cloneRule` 节是唯一数据源；config.json 残留的 `project` 节不再被读取（`config.ProjectConfig` 已删），留着无害。

### 5. settings 页「项目·扫描」分区实装

- 替换现有占位 tab（`web/src/pages/settings/index.tsx` 的 `scan` 分区），内容为两组表格：**扫描规则**（group / path / maxDepth）与**克隆规则**（repoHost / repoPrefix / localPath）；
- 交互复用 1025 定型模板：Sheet 抽屉编辑、删除前 ConfirmDialog、name/操作列冻结、grip 拖拽排序（调 reorder API）；
- Config 过渡分区移除「扫描规则（project.scan）」只读表格（迁移后 config.json 已无 project 节）。

## 不做的事

- 不改 CLI 命令面（init/check/clone 消费的是 Service 接口，不变）；
- 不做规则的热 reload 机制（直读天然即时，无需 reload 概念）；
- 不在本提案收编 Config 分区的其他内容（log/dataDir 等仍留 config.json，属后续提案）。

## 实施顺序

> 逐步实施、逐步验收；每步可独立收工。步骤 1 上线后需要用户手动迁移一次数据（见方案 4）。

1. `config.ProjectConfig` / 规则 struct 迁至 project 包 + Service 直读化 + 单测（`project/scan_test.go` 的 `newServiceAt` 改为写 settings.json fixture）——**本步上线后用户手动迁移 config.json → settings.json**；
2. Web API 四对 save/delete/reorder + handlers 测试；
3. 前端「项目·扫描」分区实装 + Config 分区移除 scan 表格；
4. `docs/spec/现状.md` 同步，验收后归档提案。

## 验收标准

1. 用户手动迁移后，settings.json 的 `scanRule`/`cloneRule` 节生效，`cube project list` / `cube check` / `cube clone` 行为与迁移前一致；config.json 残留 `project` 节不被读取；
2. settings 页「项目·扫描」分区完成 scan/clone 两组规则的增删改 + 拖拽排序，保存后 `cube project list --status` / `cube clone` 匹配立即反映新规则（无重启、无缓存不一致）；
3. `pnpm -C web build`、`cd server && go vet ./... && go test ./...` 通过；
4. `docs/spec/现状.md` 已同步。
