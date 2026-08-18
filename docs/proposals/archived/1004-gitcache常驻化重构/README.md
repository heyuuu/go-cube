# gitcache 常驻化重构

> **状态**：✅ 已实现（2026-08）
> **历史**：原依赖提案 `260811-server按需启动`（lazy 拉起方案），该提案实施时改为 nginx 模式（server 由用户显式 `start`，不 lazy 拉起）。本重构照常进行——server 在跑时定时刷新，没跑时 CLI 读上次快照（可接受，见 alternatives.md 方案 2/3 论证）。
>
> **实施偏差**（相对原计划）：
> - 定时器接入点定为 `cmd/server/start.go` 的 `startServer()`（原计划同），由 `project.Service.StartRefreshTicker`/`StopRefreshTicker` 提供能力，不挂 web 层（alternatives.md 方案 4 否决）。
> - 定时器间隔写死 5 分钟（`defaultRefreshInterval`），未走 config。
> - CLI 不加刷新兜底，纯读快照。

## 背景

### 现状机制

`project/gitcache/` 当前是「**fork 子进程异步采集 + 文件落盘 + flock 跨进程互斥**」的设计。核心动机是 cube 传统上是**一次性 CLI 进程**（用完即退），约束链条：

- git 采集是 IO 密集（几十个仓库），前台命令不能阻塞等
- 父进程用完即退，goroutine 随进程死，没法把采集留在父进程里
- 又需要跨多次 CLI 调用共享采集结果

唯一出路就是 fork 独立子进程采集 + 结果落盘（`git.json` 作为跨进程通信信道）。flock 是为「多个独立 CLI 进程同时 fork 子进程」的去重兜底。

### 关键观察：web server 长驻却没产生简化收益

核实代码发现，web server 当前虽然长驻，但 gitcache 仍然走 fork 子进程那套（`web/api_project.go:71-84`）：

```go
func (h *ProjectHandler) projectList(...) {
    h.service.ReloadGitCacheIfStale()   // mtime 检测磁盘是否被子进程更新
    h.service.TriggerAsyncRefresh()     // fork 子进程采集
    ...
}
```

长驻父进程 fork 一次性子进程 + 文件中转 + mtime 检测搬回内存——是个别扭的组合。`ReloadGitCacheIfStale` 本身就是为了补「子进程写盘后父进程不自动感知」的窟窿。**web server 当前的长驻，对 gitcache 没有产生任何简化收益。**

### 为什么重构

`260811-server按需启动` 落地后，server 真正成为常驻进程（lazy 拉起 + 后台留存）。这颠覆了 gitcache 的核心前提——**「没有常驻进程」不再成立**。整套 fork + 文件中转 + flock + mtime 检测都是为这个前提打的补丁，常驻后大部分变得冗余。

## 目标

把 gitcache 的刷新机制从「fork 子进程 + 文件中转」改为「**常驻进程内 goroutine 定时刷新**」，删掉所有为「无常驻进程」打补丁的代码。

## 架构终态

### 核心原则：读路径共用，差异只在刷新触发

**gitcache 包的 API 不分 CLI/Server**：
- `Load` / `Get` / `Save` / `Refresh` 都是通用能力
- CLI 和 Server 启动时都走 `project.NewService` → 都初始化 `*gitcache.Cache` → 都 `Load` git.json 到内存
- **读路径完全共用**（这就是"两个逻辑共用"的含义）

**差异只在于"谁触发刷新"**：

| 进程 | 读 git.json | 触发刷新 |
|---|---|---|
| **Server**（常驻） | 启动时 Load | 进程内 `time.Ticker` 定时调 `Refresh`（goroutine 并发采集）→ 写内存 → flush git.json |
| **CLI**（短命） | 启动时 Load | **不触发任何刷新**，只读启动时 Load 进来的内存快照 |

### git.json 的角色降级

从「跨进程通信信道」降级为「**跨重启持久化缓存**」：
- Server 启动时 Load 一次（避免冷启动全量采集几十个仓库的卡顿）
- Server 运行期以内存缓存为主，定时 flush 到 git.json
- Server 重启后从 git.json 秒恢复
- CLI 读 git.json，接受快照可能略旧（新鲜度依赖 Server 的 flush 频率）

## 要做的事

### 1. 删除：调度层整套 fork 机制

删掉 `project/gitcache/refresh.go`（整个文件）：
- `TryAsyncRefresh`（父进程侧 fork 子进程）
- `RefreshSync`（子进程侧 flock 抢锁 + 采集）
- `shouldRefresh`（TTL 判断，纯只读 git.lock）
- `writeLockState`（写 git.lock）
- flock 跨进程互斥逻辑

删掉 `cmd/refresh_git_cache.go`（`cube project refresh-git-cache` 子命令，fork 的入口）。常驻后刷新由 Server 定时器自动跑，这个手动触发命令不再需要。

### 2. 删除：文件中转的感知补丁

删掉 `Cache.IsStale()` 和 `Cache.Reload()` 的 mtime 检测路径，以及 `project.Service.ReloadGitCacheIfStale()`：
- 这些是为了「子进程写盘后父进程不自动感知」而加的补丁
- 常驻进程内 goroutine 直接写内存，没有「写盘 → mtime 检测 → reload 回内存」这个迂回

删掉 `web/api_project.go` 里 `projectList` 的 `ReloadGitCacheIfStale()` + `TriggerAsyncRefresh()` 调用——刷新改由 Server 定时器自动驱动，读 API 不再顺带触发。

### 3. 删除：git.lock 文件

`git.lock` 承担双重职责（flock 载体 + TTL 状态 `lastRefreshAt`），两者都随 fork 机制消失。整个文件不再需要。

### 4. 保留：数据层基本不动

`project/gitcache/cache.go` 的核心 API 保留：
- `Entry` / `cacheFile` / `Cache` 结构定义
- `Load(dir)` / `Reload()`（Reload 改为纯内部用，不再靠 mtime）
- `Get(path)`（读内存，RWMutex 读锁）
- `Save()`（原子写 git.json：tmp + rename）——**写安全靠这个保证**，Server 是唯一写方，无并发写问题
- `Refresh(paths)`（8 并发 goroutine 采集 → 合并内存 → Save）
- `collectEntry(path)`（调 gogit 采集单项目）

### 5. 新增：Server 进程的定时刷新

在 `project.Service` 层增加定时器能力（领域自治，web 层不插手）：

- `NewService(...)` 保持既有签名（CLI/Server 共用，不强制起定时器）
- 新增 `StartRefreshTicker(interval time.Duration)`：启动一个 goroutine 跑 `time.Ticker`，定时调 `Cache.Refresh`
- 新增 `StopRefreshTicker()`：停止定时器（Server shutdown 时调，资源清理）
- 装配点：`app/app.go` 里 Server 装配时显式调 `service.StartRefreshTicker(...)`；CLI 装配不调（CLI 短命，不刷新）

定时器间隔暂定 5 分钟（可后续做配置化，初版写死或走 config）。

### 6. 保留：降级优先原则

gitcache 任何故障都不阻塞业务（Load 失败返回空缓存、采集异常 recover 吞掉、损坏文件备份后重置）。这个原则在重构后必须保留——定时器 goroutine 里的 panic 要 recover，不能让采集异常拖垮 Server 进程。

## 写安全说明

Server 是 git.json 的唯一写方（CLI 只读不写）。并发安全从「flock 多写者互斥」简化为「**单写者原子写**」：

- `Save()` 走 tmp 文件 + `os.Rename`（同目录保证同文件系统原子性）
- 无需 flock，无需 `sync.Mutex` 跨进程
- 进程内 `Refresh` 已有 goroutine 并发采集，但写回内存有 `sync.RWMutex` 保护（既有逻辑，不动）

## 改动文件清单

| 文件 | 改动 |
|---|---|
| `server/project/gitcache/refresh.go` | **删除整个文件** |
| `server/project/gitcache/cache.go` | 删 `IsStale`；`Reload` 调整为内部用；其余保留 |
| `server/cmd/refresh_git_cache.go` | **删除整个文件** |
| `server/cmd/root.go` | 删 `newRefreshGitCacheCmd` 的 `AddCommand` |
| `server/project/service.go` | 删 `ReloadGitCacheIfStale`；`TriggerAsyncRefresh` 改名/重写为进程内 goroutine；新增 `StartRefreshTicker` / `StopRefreshTicker` |
| `server/web/api_project.go` | 删 `projectList` 里的 `ReloadGitCacheIfStale()` + `TriggerAsyncRefresh()` 调用 |
| `server/app/app.go` | Server 装配时调 `StartRefreshTicker`；shutdown 时调 `StopRefreshTicker` |
| `~/.config/cube/cache/git.lock` | 运行期不再生成（已存在的残留可忽略） |

## 工作量评估

- 删除调度层 + 补丁（refresh.go / refresh_git_cache.go / IsStale / ReloadGitCacheIfStale）：中等，主要是理清依赖
- 新增定时器（StartRefreshTicker / StopRefreshTicker）：小
- 装配点改造（app.go 起停定时器）：小
- web 读 API 简化（删两个调用）：小
- 测试调整（cache_test.go 里涉及 refresh 的部分）：中等

**前置依赖**：`260811-server按需启动` 必须先落地（server 常驻是本重构的前提）。
