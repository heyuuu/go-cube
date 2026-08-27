# util/store 文件存储通用包

> **状态**：📝 待评审
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

## 不做的事

- 不做跨进程文件锁（flock）——当前所有写方都是「O_APPEND 追加」或「单写者整文件重写」，均不需要；
- 不做通用 KV / 带索引的存储，只做无状态的原语函数。

## 实施顺序

1. `util/store` 建包 + 单测（原子写的中断恢复语义、坏行跳过、泛型序列化，用 testfixture 工作区）；
2. gitcache `Save` 改调 `store.WriteFileAtomic`，`go test ./project/...` 回归。

## 验收标准

1. `cd server && go vet ./... && go test ./...` 通过；
2. gitcache 行为不变（现有测试全绿，无新增配置/文件布局变化）。
