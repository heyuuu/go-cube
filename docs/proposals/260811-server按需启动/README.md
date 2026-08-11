# server 按需启动（lazy 拉起）

> **状态**：📋 待办（独立提案，md / workspace 工作台依赖它）
> **来源**：md 渲染需求讨论中牵引出的 server 生命周期议题

## 背景

现状 `cube server` 是**前台阻塞命令**：用户显式启动、Ctrl+C 退出、不用时进程完全不存在。这个形态保持了 cube 作为"轻量工具"的质感——不往系统塞后台进程、不抢端口、不写守护配置。

但未来的需求（md 渲染、workspace 工作台）依赖 server 在跑。如果要求用户每次先手动 `cube server`，体验割裂；而引入"server 常驻化"的全套方案（开机启动 / 保活 / supervisor / 系统集成）又过度设计——见 [`alternatives.md`](./alternatives.md) 的逐项否决。

本提案采纳 **lazy 拉起**：把"保证 server 存活"的责任从"一个全局常驻基础设施"下沉到"每个需要它的命令自己兜底"。用时自动起、不用时不存在。

## 核心机制：ensureServer()

一个内部函数，所有依赖 server 的命令入口调用它：

```
ensureServer():
  1. 探测 server 是否在跑（读 pid 文件 + 进程探活，或 TCP 探端口）
  2. 在  → 直接返回
  3. 不在 → fork 起一个脱离终端的后台子进程
  4. 轮询等待端口就绪
  5. 返回
```

调用方（如 `cube md xxx`）调完 `ensureServer()` 后，拿到可用的 server 地址，继续自己的业务逻辑。

### fork 脱离能力（内部）

后台启动通过 **re-exec 子进程** 实现：进程用 `os/exec` 重新执行自己（带隐藏 flag 如 `--serve-child`），子进程 `setsid` 脱离控制终端、stdio 重定向到日志文件、父进程正常退出。

**这是内部能力，不向用户暴露 `-d` flag。** 用户不需要理解"后台启动"这个概念——前台 `cube server start` 自己跑，后台交给 lazy 拉起自动管。理由见 [`alternatives.md`](./alternatives.md) "start -d 独立命令"。

### 单实例与 pid 文件

全局最多一个 cube server。通过 pid 文件 + 探活保证：

- pid 文件记录 pid + 端口 + 启动时间戳
- 探活时读 pid → 信号 0 探进程存在 → 校验该 pid 确实是 cube server（防 pid 复用给别的进程，用启动时间戳或端口交叉验证）
- 进程崩溃后 pid 文件可能残留，光看文件存在不能证明存活，必须探活

## 命令族（只分组 server，其余扁平不动）

```
cube server              # = cube server start（兼容现状）
cube server start        # 前台启动（开发/调试用，现状不变）
cube server stop         # 停后台实例（读 pid → 发信号）
cube server status       # 诊断：查在不在 / pid / 端口
```

设计要点：

- **只把 server 相关命令收进 `cube server` 分组**，解决未来 `status` 等动词与 project 命令的歧义。其余命令（project / open / git）维持扁平不动（全量分组记为技术债，见 alternatives）。
- **`cube server`（不带子命令）= `cube server start`**，兼容现有习惯，平滑迁移。
- **不做 `restart`**：用户可 `stop` + `start`；且 lazy 拉起机制下，server 崩了下次命令自动恢复，手动 restart 退化为低频调试操作，不值得单独命令。
- **不做 `reload`**：无长连接（没有 WebSocket 用户），配置/二进制变了重起即可，partial reload 的工程复杂度不值。

## 生命周期约定

回答"server 起完之后怎么办"：

| 问题 | 约定 | 理由 |
|---|---|---|
| 起完后何时停 | **不主动停**，留到用户 `stop` 或关机 | 最简，符合"用一次留一次"的直觉 |
| 空闲超时自杀 | **不做** | 避免误杀正用着的 server；多写一套空闲检测不值 |
| 崩溃恢复 | **靠下次命令 lazy 拉起兜底** | 按需恢复比 supervisor 一直盯着省资源 |
| 保活/supervisor | **不做** | lazy 拉起天然兜底，不需要监督进程（见 alternatives） |

## 双模式

- **开发测试**：`cube server start`（前台），air 热重载照常，日志走 stdout，Ctrl+C 退出。与现状完全一致。
- **日常使用**：用户敲 `cube md xxx` 等命令时，`ensureServer()` 自动后台拉起，命令本身执行完即退出，server 留后台供后续复用。

## 平台范围

**仅 macOS。** Linux / Windows 不在本次范围，提案里仅留扩展口。

## 与其他提案的关系

- **md 渲染（待立项）**：依赖本提案。`cube md xxx` 调 `ensureServer()` → 开浏览器指向 `http://localhost:<port>/md/...` → cube 命令退出（server 留后台）。
- **workspace 工作台（`260811-workspace工作台`）**：强依赖。工作台天然需要共享的、长存的 server（多 tab 共用一个 server 实例），lazy 拉起机制是其基础设施。

## 工作量评估

- `ensureServer()` + pid 文件 + 探活：中等（核心逻辑，~200 行）
- re-exec daemon 实现：中等（Go re-exec + setsid + stdio 重定向，macOS 单平台可控）
- 命令分组改造（`cube server` 作为分组 + start/stop/status 子命令）：小
- `status` 命令：小（pid 文件 + 探活，几乎免费）

建议作为独立工作单元排期，不夹在 md 或工作台里零碎做。
