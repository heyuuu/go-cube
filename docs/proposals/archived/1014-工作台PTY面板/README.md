# 工作台 PTY 面板：WebSocket 终端

> **状态**：✅ 已完成并验收归档（2026-08-23）；同日增补多终端 tab / 分屏 / 偏好设置（实现差异见文末「实现更新」）
>
> **所属**：[`1008-workspace工作台` 总纲](./1008-workspace工作台/README.md)（先读总纲「已收敛的全局决策」）。
> **依赖**：[`1010-workbench基座`](./1010-workbench基座/README.md)（仅路由与面板挂载点；**本提案可任意插队实施**，不依赖 1011-1013）。

## 背景与目标

工作台里跑命令行的 PTY 终端。技术栈已定：后端 WebSocket + `creack/pty` 起子进程，前端 **xterm.js**（注意不是 term.js，那是弃用前身）。链路成熟，复杂度不在协议而在**会话生命周期管理**——这些是本提案要定的产品决策。PTY 是完全独立的面板，与其他面板无数据耦合（不读 TreeSource，不进 URL 选中态）。

**本提案不做**：会话持久化到磁盘、远程访问鉴权。（原后置的「多终端 tab 管理」已于 2026-08-23 实现，见文末。）

## 方案

### 1. 后端：WebSocket endpoint

- **依赖**：`github.com/creack/pty`（起子进程）；WebSocket 用 `github.com/coder/websocket`（原 nhooyr/websocket，维护活跃；若实施时项目已有其他 ws 依赖则复用）。
- **endpoint**：`GET /api/workbench/pty?path=<工作目录>&cols=&rows=`，`http upgrade` 升级为 WebSocket（huma 对 upgrade 路由支持不便，实际在 handler 层用原生 `http.Handler` 挂到 server 的 mux 上——`WorkbenchHandler.RegisterRaw`，路径仍守 `/api/` 前缀约定）。
- **协议**（简单 JSON 帧）：
  - client → server：`{type:"input", data}`（键盘输入）、`{type:"resize", cols, rows}`。
  - server → client：`{type:"output", data}`（PTY 输出）、`{type:"exit", code}`（进程退出）。
- **子进程**：`$SHELL`（默认回退 `/bin/zsh`），cwd 为 `path` 参数（必须是存在的目录），环境继承。PTY 初始尺寸用连接参数。
- **会话生命周期**：
  - **一条 WebSocket = 一个会话**：连接建立即起子进程；断开（页面关闭 / 前端主动关闭）= 杀子进程（defer 兜底 SIGKILL + wait 回收，不留孤儿）。多终端 = 前端开多条连接，后端天然支持，无会话数上限（量级 = 浏览器内实例数，极小）。
  - **不做断线重连保留**：断开后进程即终止，前端重连是新会话。（session 池化/重连恢复明确后置，避免背状态管理。）
- **优雅退出**：server shutdown 时（`server/web/shutdown_token.go` 停机机制）经 `Service.StopPtySessions` 统一 cancel 所有活跃会话——不留孤儿进程是硬要求。
- 安全注脚同总纲：localhost 无鉴权下这等于开放本机 shell，MVP 接受。

### 2. 前端：面板组件（`web/src/pages/workbench/panels/terminal-panel.tsx` + `terminal-session.tsx`）

- **依赖**：`@xterm/xterm` + `@xterm/addon-fit`（自适应尺寸；引入其 css）。
- **形态**：1010 布局骨架的**底部抽屉**，默认收起；点击 header 空白处（或折叠按钮）切换收展。
- **交互**：
  - 输入 → `input` 帧；`ResizeObserver` + fit addon → `resize` 帧（隐藏状态跳过，避免量出 0 尺寸）。
  - 收到 `exit` → 该终端实例自动从面板移除（级联见「实现更新」）；连接失败 → 提示 + 「重新开始」按钮（不自动移除，保留重试入口）。
  - cwd 由 URL `path` 参数决定（跟随工作台入口目录）；面板内不提供 cd 到别处的快捷切换（终端里自己 cd 即可）。
  - 字体（预设回退栈）/ 字号（10–20）/ 抽屉高度（160–600px，顶边拖拽）三项偏好走 localStorage（`workbench.terminal.*`），字体字号变化经 `term.options` 热更新不重建会话。
- **不进 URL 状态**：终端面板不消费/不写 source 系列参数，抽屉展开状态与 tab 结构也不进 URL（刷新即重来）。

## 验收标准

1. 后端：连接后 `echo hello` 输出可见；`exit` 后收到 exit 帧；杀 server 进程后无孤儿 shell（`pgrep` 验证）；resize 帧生效（`stty size` 验证）。
2. web 层：`web/api_workbench_test.go` 的 `TestWorkbenchPty` 用真实 `httptest.Server` + ws client 覆盖「连接-输入-收输出-退出」链路。
3. 前端抽屉展开即用：终端可交互、随窗口 resize、关页面后服务端进程被回收。
4. `cd server && go vet ./... && go test ./...`、`pnpm -C web build` 通过。

## 实施注意

- PTY 子进程 stdio 接管后所有输出走 ws 帧，日志走 `slog`（debug 级），不用 fmt.Println。
- goroutine 生命周期：read pump / write pump / wait 子进程，三方任一结束都要触发整体清理（channel 或 context 取消），退出路径注释在 `servePtySession` 头部。
- 错误消息中文（如 `"path 目录不可用: path=..."`）。

## 实现更新（2026-08-23：多终端 tab / 分屏 / 偏好）

原 MVP 为「一个展开周期 = 一个会话、收起即断开」。现已升级为多终端模型，**生命周期决策相应修订**：

- **面板结构**：终端 tab 列表 → tab 内横向分屏实例（不限个数，flex 均分 + 拖拽调比例，每侧最少 10%）。默认面板无任何终端；展开时若无 tab 自动新建「终端 1」。
- **会话生命周期**：**折叠面板不再断开会话**（内容常驻渲染、仅隐藏）；关闭实例 / 关闭 tab / 实例 pty 退出才断开。切 tab 不杀会话（隐藏保留现场）。
- **级联移除**：实例关闭（cmd+W / pane 按钮 / pty exit 帧）→ 从 tab 实例列表移除 → tab 空则移除 tab → tab 清空则面板自动折叠。
- **终端内快捷键**：`cmd+D` 在当前实例（焦点所在）后插入新分屏；`cmd+W` 关闭当前实例。只拦 cmd 组合，ctrl+W 等留给 shell。
- **偏好设置**：字体预设（回退栈，兜底特殊字符渲染）、字号步进、抽屉高度拖拽，均 localStorage 持久化（`use-terminal-prefs.ts`）。
- 组件拆分：`terminal-panel.tsx`（tab/分屏/偏好容器）+ `terminal-session.tsx`（单会话：xterm + ws + 状态机 + 快捷键）；后端零改动。
