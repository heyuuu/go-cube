# server 后台常驻与 HTTP 管理

> **状态**：✅ 已实现（commit `99a0f19`，2026-08）
> **历史**：本目录原为提案 `260811-server按需启动`（lazy 拉起方案），实施过程中改为 HTTP 显式管理方案，详见下文「方案演变」。

## 背景

cube 的 `cube server` 原本是**前台阻塞命令**：用户显式启动、Ctrl+C 退出、不用时进程不存在。这对「轻量工具」的质感是好的，但未来的需求（workspace 工作台、md 渲染）依赖 server 长期在跑。

需求本质：**让 server 能以后台常驻形态运行，并提供管理（启停、探活）能力**。实施时没有引入 launchd / supervisor 这类系统级守护，而是走 **HTTP 应用层管理**——server 自己暴露管理 API，CLI 通过 HTTP 来探活/关停，不依赖 pid 文件、信号、系统守护。

## 命令族

```
cube server              # = cube server start（兼容裸跑）
cube server start        # 前台启动（开发/调试用，Ctrl+C 退）
cube server start -d     # 后台启动（fork 脱终端，不占 stdout）
cube server stop         # 触发后台 server 平滑关闭
cube server status       # 探活，输出状态/端口/版本/访问地址
```

设计要点：

- **`cube server`（裸跑）= `cube server start`**，兼容现有习惯。
- **`-d/--detach` 是后台启动的主入口**：fork 一个子进程跑前台模式的 `cube server start`，setsid 脱离终端，父进程立即退出。
- **不做 `restart` / `reload`**：cube 是个人工具，无长连接（无 WebSocket 用户），配置/二进制变了 stop + start 即可，partial reload 的工程复杂度不值。
- **砍掉自动开浏览器**（`--open` flag / `cube ui` 命令）：server 只负责打印访问地址（`http://localhost:<port>/`），用户在终端里自己点。现代终端都识别 URL。
- **允许多实例**：不同端口 = 不同 server，不做全局单实例约束。`-p` 指定端口（默认 8080），不进 config（端口不是唯一事实源）。

## HTTP 管理 API

进程管理全部走 HTTP，不靠 pid 文件或信号：

| 端点 | 鉴权 | 作用 |
|------|------|------|
| `GET /api/system/whoami` | 无 | 返回 `{app:"cube", version}`，探活方验 `app=="cube"` 确认身份 |
| `POST /api/system/shutdown` | HMAC 时间戳 | 触发本进程自发 SIGTERM，走 graceful shutdown |

### shutdown 的 HMAC 时间戳鉴权

防盲目 CSRF 调用和重放攻击：

- 请求方（CLI）用写死常量 token 对当前时间戳做 HMAC-SHA256 签名，放 `X-Shutdown-Token: <unix秒>.<sig>` header。
- server 端重算 HMAC 比对 + 校验时间戳在 **5 秒窗口**内（本机调用毫秒级往返，窗口够宽又防重放）。
- token 编译进二进制（server 与 CLI 共享）。防的是「浏览器/远程的盲目 CSRF」和「抓包重放」，不防「本机能反编译拿 token 的进程」（那个威胁等级下能直接 kill，token 已非瓶颈）。

### 关停路径

shutdown handler 鉴权通过后 → 先回 200 → 异步 `syscall.Kill(os.Getpid(), SIGTERM)` 给本进程发信号 → `web.Server.Start` 里已注册的 `signal.Notify` 接收 → `http.Server.Shutdown` graceful 退出。

**和用户按 Ctrl+C 走完全相同的路径**，没有两条分支。handler 不持有 Server 引用，依赖图干净。

## 生命周期约定

| 问题 | 约定 |
|------|------|
| 起完后何时停 | 不主动停，留到 `stop` 或关机 |
| 空闲超时自杀 | 不做（避免误杀；空闲检测工程不值） |
| 崩溃恢复 | 不做 supervisor，用户重新 `start` 即可 |
| 保活/supervisor | 不做（见 alternatives） |

## 平台范围

仅 macOS。Linux / Windows 不在本次范围。

## 方案演变（从 lazy 拉起到 HTTP 显式管理）

本需求实施过程中**大幅调整过方向**，记录关键转变以备未来回溯：

### 阶段一：lazy 拉起 + pid 文件（初版方案，已废弃）

最初设计为「lazy 拉起」：依赖 server 的命令（如 `cube md xxx`、`cube ui`）自动 fork 后台 server，靠 **pid 文件 + 信号 0 探活 + 端口交叉验证** 保证单实例。

实施中发现这套方案的复杂度失控：
- pid 文件探活无法可靠解决「pid 复用」——系统回收 pid 后，新的无关进程可能恰好占了同 pid，光靠信号 0 探活会误判。
- 端口冲突处理（已有 server 跑在别的端口时）需要一套三态判定（alive / portConflict / dead），每个入口都要复用，逻辑蔓延。
- `__serve` 隐藏子命令、re-exec 的 stdio 重定向、孤儿进程防护……每一层都在引入新的边界条件。

### 阶段二：HTTP 显式管理（最终方案）

转向 nginx 模式：**server 只能由用户显式 `start` 启动，依赖 server 的命令只探测、不自动起**。身份认证和关停触发全部走 HTTP API：

- 身份 = 谁能响应 `/api/system/whoami` 返回 cube 标记
- 控制 = 谁能响应 `/api/system/shutdown` 实际关掉服务

彻底绕开了 pid 复用、孤儿进程、端口冲突三态这类 OS 层副作用问题——别的进程即便占了同 pid 或同端口，它不会响应 cube 的 API，天然被识破。

代码量和边界情况都大幅减少。详见 [`alternatives.md`](./alternatives.md) 对各方案的否决理由。

## 与其他需求的关系

- **workspace 工作台**：依赖本机制。工作台需要长存的 server，由 `cube server start -d` 拉起后，前端连 `ws://localhost:<port>/...`。
- **md 渲染**：未来实现时，`cube md xxx` 先调 `serve.Status(port)` 探活，在跑则开浏览器指向 `/md/...`，不在则提示用户先 `start`。
