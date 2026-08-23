# 工作台 PTY 面板：WebSocket 终端

> **状态**：✅ 已实现（2026-08-18）
>
> **所属**：[`1008-workspace工作台` 总纲](../1008-workspace工作台/README.md)（先读总纲「已收敛的全局决策」）。
> **依赖**：[`1010-workbench基座`](../archived/1010-workbench基座/README.md)（仅路由与面板挂载点；**本提案可任意插队实施**，不依赖 1011-1013）。

## 背景与目标

工作台里跑命令行的 PTY 终端。技术栈已定：后端 WebSocket + `creack/pty` 起子进程，前端 **xterm.js**（注意不是 term.js，那是弃用前身）。链路成熟，复杂度不在协议而在**会话生命周期管理**——这些是本提案要定的产品决策。PTY 是完全独立的面板，与其他面板无数据耦合（不读 TreeSource，不进 URL 选中态）。

**本提案不做**：多终端 tab 管理（先单会话，tab 后置）、会话持久化到磁盘、远程访问鉴权。

## 方案

### 1. 后端：WebSocket endpoint

- **依赖**：`github.com/creack/pty`（起子进程）；WebSocket 用 `github.com/coder/websocket`（原 nhooyr/websocket，维护活跃；若实施时项目已有其他 ws 依赖则复用）。
- **endpoint**：`GET /api/workbench/pty?path=<工作目录>&cols=&rows=`，`http upgrade` 升级为 WebSocket（huma 对 upgrade 路由支持不便时，可在 handler 层用原生 `http.Handler` 挂到 server 的 mux 上——参考 `server/web/server.go` 的组装方式，路径仍守 `/api/` 前缀约定）。
- **协议**（双向 JSON 消息或裸流，推荐简单 JSON 帧）：
  - client → server：`{type:"input", data}`（键盘输入）、`{type:"resize", cols, rows}`。
  - server → client：`{type:"output", data}`（PTY 输出）、`{type:"exit", code}`（进程退出）。
- **子进程**：`$SHELL`（默认 `/bin/zsh` 回退 `/bin/bash`），cwd 为 `path` 参数（必须是存在的目录），环境继承。PTY 初始尺寸用连接参数。
- **会话生命周期（MVP 决策，实施即按此）**：
  - **一个前端页面对应一个会话**：前端面板挂载时连接、卸载/页面关闭时断开；断开 = 杀子进程（`pty.Kill` + wait 回收，不留孤儿）。
  - **不做断线重连保留**：断开后进程即终止，前端重连是新会话。（session 池化/重连恢复明确后置，避免 MVP 背状态管理。）
  - 并发限制：同一 server 允许多个连接（不同浏览器 tab 各自独立），但单连接内单进程。
- **优雅退出**：server shutdown 时（`server/web/shutdown_token.go` 已有停机机制）向所有活跃 PTY 发 SIGHUP 并回收——不留孤儿进程是硬要求。
- 安全注脚同总纲：localhost 无鉴权下这等于开放本机 shell，MVP 接受。

### 2. 前端：面板组件（`web/src/pages/workbench/panels/terminal/`）

- **依赖**：`@xterm/xterm` + `@xterm/addon-fit`（自适应尺寸；Web 字体渲染注意引入其 css）。
- **形态**：1010 布局骨架的**底部抽屉**，默认收起；展开时若未连接则建立 WebSocket。
- **交互**：
  - 输入 → `input` 帧；`ResizeObserver` + fit addon → `resize` 帧。
  - 收到 `exit` → 显示退出码 + 「重新开始」按钮。
  - 连接断开（非主动关闭）→ 提示断开 + 重连按钮（= 新会话）。
  - cwd 由 URL `path` 参数决定（跟随工作台入口目录）；面板内不提供 cd 到别处的快捷切换（终端里自己 cd 即可）。
- **不进 URL 状态**：终端面板不消费/不写 source 系列参数，抽屉展开状态也不进 URL（刷新即收起）。

## 验收标准

1. 后端：连接后 `echo hello` 输出可见；`exit` 后收到 exit 帧；杀 server 进程后无孤儿 shell（`pgrep` 验证）；resize 帧生效（`stty size` 验证）。
2. web 层：PTY 不走 httptest 的常规 JSON 断言，用 `coder/websocket` 的测试工具（或真实 `httptest.Server` + ws client）补一条「连接-输入-收输出-退出」链路用例；若 ws 测试成本过高，允许以集成手测为主并在 PR 说明（对齐 AGENTS.md 测试策略中「进程编排类靠手动验证」的口径）。
3. 前端抽屉展开即用：终端可交互、随窗口 resize、关页面后服务端进程被回收。
4. `cd server && go vet ./... && go test ./...`、`pnpm -C web build` 通过。

## 实施注意

- PTY 子进程 stdio 接管后所有输出走 ws 帧，日志走 `slog`（debug 级），不用 fmt.Println。
- goroutine 生命周期：read pump / write pump / wait 子进程，三方任一结束都要触发整体清理（channel 或 context 取消），写清注释里的退出路径。
- 错误消息中文（如 `"path 目录不存在: path=..."`）。
