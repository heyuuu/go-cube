# 最近使用排序与 usage 统一

> **状态**：📝 待评审
>
> **关联**：[`1027-util-store文件存储`](../1027-util-store文件存储/README.md)（前置需求，步骤 2 依赖其 JSONL 原语）；[`1030-monorepo-workspace`](../1030-monorepo-workspace/README.md)（其打开子目录时 usage 需带 `subPath`，见「格式演进约定」）。

## 背景与目标

现状问题：

1. history 包两张表（`ProjectSelectLog` / `ProjectOpenLog`）带 `Alfred` 区分字段，但**只有 alfred 三条命令在读写**，`Alfred` 恒为 true，是事实上的死字段；
2. Web `opener/open` 与 CLI `cube open` **不记任何使用记录**，「前端点了 opener」不产生使用信号；
3. 项目列表（Web `project/list`、CLI）没有最近使用排序，最近使用信号只服务 alfred 搜索；
4. sqlite + gorm 重依赖**只服务 history 一个 domain**。

目标：统一为一份「项目使用记录」（usage），所有打开入口都记录，项目列表默认按最近使用置顶排序，并借此移除 gorm + sqlite 依赖，改 JSONL 文件存储。

## 方案

### 1. 数据模型：一张记录，三字段

一次「使用」= 一次 opener 打开事件。原两张表合并为一种记录，`ProjectSelectLog` 整体废弃（选中未打开对「最近使用」无信号价值），`Alfred` 字段删除：

```json
{"time":"2026-08-27T21:03:14+08:00","project":"/Users/heyu/Code/heyuuu/cube","opener":"idea"}
```

- `time`：RFC3339，行序与它单调一致，承担原表自增 id 的排序职责；
- `project`：**项目绝对路径**（不再是旧表的 name——name 跨 group 可撞名，path 是 project 的唯一标识，列表排序 join 用它）；
- `opener`：opener 名（settings openers 节的 key）。

两个查询语义与原 SQL 等价，读侧全量读 + 内存去重（同 key 留更新行）：

- 按 project 去重取最新 → 项目列表置顶排序（原 `LeastSelectedProjects`，改用 open 记录）；
- 限定 project 按 opener 去重取最新 → opener 排序偏好（原 `LeastProjectOpenApps`）。

### 2. 记录入口（三处，`open_path` 不记）

| 入口 | 动作 |
|---|---|
| Web `POST /api/opener/open` | 打开成功后追加一条 usage |
| CLI `cube open` | 同上 |
| alfred `project_open` | 已有记录逻辑，改写新格式 |

`open_path`（按路径直开）**不记录**：语义是「path 的打开记录」而非「项目的使用记录」——目标路径可能不是项目（脏 key 永不消亡），信号强度也弱。将来若有 open_path 自身的 history 需求（含 opener.Role、多 path 参数等），另案另做。

### 3. 存储：JSONL 文件，全程无锁

- 文件：配置目录下 `usage.jsonl`（**不放 `cache/`**——它参与排序语义，非可随意丢弃的缓存）；
- 追加：`O_APPEND` 写一行，POSIX 小块 append 原子，CLI 短进程与常驻 server 并发追加安全，**无需 flock**；
- 清理：server 启动时 compaction 一次（沿用现 `OnServerStart` 时机）——tmp+rename 整体重写，每个 `(project, opener)` 组合留最新一条 + 30 天内记录。重写瞬间并发 append 的极少数记录会丢，usage 是 best-effort 信号，接受；
- 历史数据**不迁移**：旧表只有 alfred 记录且 30 天轮转，价值低，新文件从空开始，排序数日内自然收敛。

读写原语走 `util/store`（见前置提案）。

### 4. 移除 gorm + sqlite

usage 落地后，`db` 包、gorm 依赖、`data.db` 运行期状态、`app.go` 的 AutoMigrate 接线一并删除。history 包重写为无 db 的 Service（或更名 usage 包），保留 30 天保留期常量与 `OnServerStart` 钩子形态。

### 5. 项目列表默认排序

- 语义：最近使用的 **N=10** 条置顶（按最近使用倒序），其余保持原序——与 alfred `sortProjectsWithHistory` 语义一致；
- 分层：project 包不依赖 history——排序在出口层做（handler / cmd 拿 `map[path]time` 做稳定排序）；
- 前端：置顶行加 **badge 显示相对时间**（如「2 小时前」），不用背景色（避免把列表切成两个视觉区块）。

## 格式演进约定（给后续提案）

行结构允许增量加字段，**读取方必须容忍缺失字段**（JSONL 无 schema，Go 侧加 `omitempty` 字段即前后向兼容，不预先预留无写入方的字段）。已知的第一个演进：1030 monorepo workspace 落地后，打开子目录时追加 `subPath`（相对项目根），`project` 恒为主项目路径——排序与 opener 偏好仍 keyed 于主项目。1030 实施步骤已补此接线义务。

## 不做的事

- 不迁移 sqlite 旧数据；
- 不给 `open_path` / diff 类多 path 打开记 usage（见上）；
- 不做「最近使用」的 Web 可视化页面（只做列表排序 + badge）。

## 实施顺序

> 步骤 1 独立有价值且不被步骤 2 推翻；前置：`1027-util-store文件存储` 已完成。

1. history 包重写为统一 usage Service（仍在 sqlite 上，去掉 Alfred / ProjectSelectLog），三入口接入，列表置顶排序（CLI + Web）+ badge；
2. 换 `usage.jsonl` 文件存储（store.AppendJsonl / LoadJsonl + 启动 compaction），删 `db` 包 + gorm 依赖 + `data.db`；
3. `docs/spec/现状.md` 同步，验收后归档提案。

## 验收标准

1. 任一入口（web / CLI / alfred）打开项目后，`usage.jsonl` 追加对应记录；项目列表最近使用的 10 条置顶且带相对时间 badge，其余保持原序；
2. alfred 的 opener 排序偏好行为与迁移前一致（同一项目的常用 opener 靠前）；
3. `go.mod` 无 gorm 依赖，配置目录无 `data.db`；server 重启后 `usage.jsonl` 被 compaction（每个 project×opener 组合至多留最新一条 + 30 天内记录）；
4. `pnpm -C web build`、`cd server && go vet ./... && go test ./...` 通过；
5. `docs/spec/现状.md` 已同步。
