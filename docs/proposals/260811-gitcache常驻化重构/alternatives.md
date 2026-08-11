# 否决方案存档

讨论中考虑过、最终未采纳的方案。记录其机制与否决理由，供未来重新评估时参考。

---

## 1. 完全删掉 git.json，纯内存缓存

**机制**：Server 运行期只维护内存缓存，不落盘。每次 Server 启动都冷启动全量采集一次。

**否决理由**：
- Server 重启后的第一次全量采集会卡顿（几十个仓库的 IO 密集操作），违背"读路径不阻塞"的设计目标。
- git.json 降级为"跨重启持久化"后，代价几乎为零（既有 `Save` 机制保留），收益是重启秒恢复。成本收益不对称。
- CLI 读路径依赖 git.json（CLI 是短命进程，没法自己采集），纯内存会让 CLI 完全拿不到 git 状态。

---

## 2. CLI 砍掉 git 状态显示

**机制**：CLI 的 `cube projects` / `cube info` 不再显示 git 信息（分支/ahead-behind/dirty 等），git 状态只在 web 看。CLI 只管项目结构信息。

**否决理由**：
- CLI 优先是 cube 的定位原则之一，砍掉 CLI 的 git 状态能力是明显的功能倒退。
- CLI 读 git.json 旧快照的成本极低（Load 一次到内存），新鲜度虽依赖 Server flush 频率，但"略旧"对 CLI 浏览场景可接受——用户要新鲜数据时起 Server 即可。
- 砍掉会逼用户为了看 git 状态必须起 Server，违背"CLI 优先"。

---

## 3. CLI 保留自己 fork 采集

**机制**：只把 web 的刷新改成常驻进程内定时器，CLI 仍保留旧的 fork 子进程采集机制（`refresh-git-cache` 子命令 + flock）。

**否决理由**：
- fork 机制删不干净，`refresh.go` / `refresh_git_cache.go` / flock / git.lock 都得保留，违背"简化"初衷。
- 代码库会同时存在两套刷新机制（Server 进程内 + CLI fork），维护负担倍增。
- CLI 读 git.json 旧快照已经够用（见方案 2 的否决理由），没必要为 CLI 单独维护 fork 能力。

---

## 4. 定时器挂在 web 层（web.Server）

**机制**：在 `web.Server.Start` 里 `go startGitCacheTicker(...)`，由 web 出口层驱动 project 域的刷新。

**否决理由**：
- web 是出口层，职责是"把领域包成 HTTP"，不该插手领域内部的刷新调度。跨层调用违背分层纪律。
- 领域逻辑应自治：`project.Service` 自己管自己的定时器，web 只负责 HTTP 包装。
- 采用方案：定时器挂在 `project.Service`（领域层），`StartRefreshTicker` / `StopRefreshTicker` 成对暴露，Server 装配时显式启动、shutdown 时停止。CLI 装配不启动。这样 CLI/Server 共用 `NewService`，差异只在装配点的一次调用。

---

## 5. 保留 flock 做并发写保护

**机制**：即便常驻后只有一个写方（Server），仍保留 flock 防御"万一多个 Server 实例同时写"。

**否决理由**：
- `260811-server按需启动` 已定全局单实例（pid 文件 + 探活），不会有多个 Server 同时跑。
- `Save()` 的原子写（tmp + rename）对单写者已经足够安全。
- flock 是为"多个独立进程"设计的，单进程内用 `sync.RWMutex`（既有）即可，flock 多此一举。

---

## 6. 刷新触发改为"读请求驱动"（保留现状思路）

**机制**：不引入定时器，仍靠 web 读 API（`/api/project/list`）触发刷新，但把 fork 改成进程内 goroutine。

**否决理由**：
- 读请求驱动的问题是"没人读就不刷新"——Server 常驻着但用户没开浏览器时，git.json 永远是旧的，下次 CLI 读到的快照可能非常旧。
- 定时器驱动让 Server 成为"主动采集者"，刷新节奏稳定可预期，不依赖外部请求。
- 定时器是常驻进程才可能的能力（短命 CLI 没法定时），不用白不用。
