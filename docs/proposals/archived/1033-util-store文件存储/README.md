# util/store 文件存储通用包

> **状态**：✅ 已交付（2026-08-27 验收归档；实施偏差与增强备忘见文末）
>
> **关联**：[`1034-最近使用排序与usage统一`](../1034-最近使用排序与usage统一/README.md)（本提案是其前置需求，usage 的 JSONL 存储依赖本包）。

## 背景与目标

cube 的文件存储原语目前散落在各领域包里就地手写：gitcache 的落盘是自己实现的 tmp+rename 原子写（`project/gitcache/cache.go` 的 `Save`）。接下来的 usage 记录（见关联提案）需要 JSONL 追加写 + 全量读 + 原子重写，同类需求只会越来越多。

目标：建 `util/store` 包，收敛文件存储的通用原语，一次沉淀多方复用。

## 方案

### API 面（只收「现在就有消费方」的）

```go
// 原子写：tmp 文件 + rename（tmp 落同目录，保证与目标同文件系统）。gitcache.Save 现有实现迁入。
WriteFileAtomic(path string, data []byte, perm fs.FileMode) error

// JSON 整文件读写
LoadJson[T any](path string) (T, error)   // 文件缺失返回可区分的哨兵错误，调用方降级
SaveJson(path string, v any) error         // marshal + WriteFileAtomic

// JSONL 行式读写
AppendJsonl[T any](path string, v T) error // O_APPEND 打开追加一行（小块 append 写 POSIX 原子，无需跨进程锁）
LoadJsonl[T any](path string) ([]T, error) // 全量读，坏行跳过并 slog 记录（降级优先）
```

- 只依赖标准库，符合能力层纪律（无基础设施上层依赖、无环境副作用——路径由调用方给）；
- head/tail 等更多能力**先不做**：util 包加函数是加法，等真有消费方再补，不预埋无消费方的 API。

### gitcache 重构

`Cache.Save` 的 tmp+rename 逻辑替换为 `store.WriteFileAtomic`，行为不变（含备份文件的写法若涉及原子写一并复用）。这是本包的第一个存量受益方，也是原子写正确性的现成回归验证。

## 实施偏差与增强备忘（归档时补记）

- **存量迁移超出提案范围**：除 gitcache 外，`config.Save/Load` 与 `settings.readDoc/saveDoc` 的手写原子写/JSON 读写也一并收敛——至此全仓手写原子写清零（提案只列了 gitcache，实施中确认另两处同模式，用户要求合并）。
- **`SaveJson` 定为缩进 + 尾换行**（提案未指定序列化形态）：JSON 存储文件常被人工翻看（config.json / settings.json / git.json），可读性优先；config 迁移时保持原缩进行为。第一个消费方即 config。
- **`WriteFileAtomic` 自动递归建目录**（`MkdirAll` 0755）：原实现要求目录已存在，实施中改为自动创建，调用方（config.Save）的 MkdirAll 随之移除。
- **tmp 文件名用 `os.CreateTemp` 随机后缀**而非原 gitcache 的固定 `.tmp` 后缀：避免中断残留同名 tmp 冲突；语义仍是同目录 + rename。
- **超额新增 JSONL 流式迭代器** `IterJsonl` / `IterJsonlReverse`（`iter.Seq2[物理行号, T]`）：逐行推进、随时 break 停止，逆向为 64KB 分块从文件尾扫描（不整读文件），为大文件取最近记录（1034 usage 场景）预置；两者属 JSONL 主题新增 API，非预埋无消费方（1034 已排期）。
- **包内布局按主题分文件**：`store.go`（包 doc + `ErrFileMissing`）/ `atomic.go` / `json.go` / `jsonl.go`，主题间无相互依赖（json 单向依赖 atomic），为后续按文件类型扩展留位。

## 不做的事

- 不做跨进程文件锁（flock）——当前所有写方都是「O_APPEND 追加」或「单写者整文件重写」，均不需要；
- 不做通用 KV / 带索引的存储，只做无状态的原语函数。

## 实施顺序

1. `util/store` 建包 + 单测（原子写的中断恢复语义、坏行跳过、泛型序列化，用 testfixture 工作区）；
2. gitcache `Save` 改调 `store.WriteFileAtomic`，`go test ./project/...` 回归。

## 验收标准

1. `cd server && go vet ./... && go test ./...` 通过；
2. gitcache 行为不变（现有测试全绿，无新增配置/文件布局变化）。
