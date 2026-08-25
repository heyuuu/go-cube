# desktop 壳与 wails 评估

> **状态**：⏸️ 挂起（结论为「不做」，且属结构性不匹配而非时机未到；解挂条件见文末）

## 背景

讨论「给 cube 加一个 desktop app 壳（wails）的可能性和成本」。cube 现状：CLI + 本地 web server（huma HTTP API）双出口，前端 `web/` 深度消费 HTTP 契约（openapi-fetch + OpenAPI schema + `ApiOutput` envelope），workbench 终端走 WebSocket（`/api/workbench/pty`）。

## 评估过程的关键事实

1. **wails 的心智模型是「前端项目为主体，Go 是前端的后端」**：绑定（bindings）生成 JS 调用函数的私有 RPC 形态，与 cube「显式 HTTP API + 通用消费者」的形态相反。
2. **cube 的存量资产全在 Go 领域层**，前端只是展示层——「前端为主、后端语言随意」的前提对 cube 不成立。
3. wails 也能挂 `http.Handler` 承载现有 web UI（v2 `AssetServer.Handler` / v3 `Route`），但此时 bindings 卖点完全用不上，wails 退化为「带窗口的浏览器」，相比 Tauri/Electron/零框架土办法无优势。
4. Tauri/Electron 无法承载 Go 后端，只能走「本地 server + 壳」，与土办法等价。

## 结论（三段推理）

1. **cube 没有添加 desktop 壳的强需求**——开 web 已可用，壳的增量（窗口/托盘/常驻）均为锦上添花。
2. **其他项目没有对 Go 后端的强绑定**——通用桌面壳场景 Electron 生态更丰富、Tauri 更轻量且生态好于 wails（wails 的唯一护城河是「后端必须是 Go」）。
3. **纯简单工具求低占用**——应走 Swift/SwiftUI 原生路线（WKWebView 方案常驻 100MB+，SwiftUI 工具十几 MB）。

**推论**：没有投入学习 wails 的必要。wails 的知识资产可迁移性差（胶水框架的私有绑定模型 + alpha API）；同等时间投入 Swift/SwiftUI 是平台级技能，与个人 Mac 工具线直接匹配。

## 为什么是「结构性不匹配」而非「暂时不做」

「bindings 有不可替代提升」的场景逐一推演后均不成立：

- **延迟**：bindings IPC vs `127.0.0.1` HTTP 是微秒级差，本地 server 场景无感知；
- **流式**：绑定是请求-响应，长连接反而靠 WebSocket/SSE（cube pty 已这么干且工作正常）；
- **调用语义**：`(value, error)` 绑定能表达的，`fetch` + envelope + openapi-fetch 类型推导全能表达，后者还白送 curl/浏览器/agent/第三方工具等通用消费者——这正是 cube「可被编排、不绑 AI」定位的根基，bindings 反而把这些消费者全挡在外面。

即：**cube 的产品形态（本地 server + 通用 HTTP API）与 bindings 模型（单 app 私有 RPC）是互斥的两种哲学**，前者越做越好用，后者越没有空间。

## 解挂条件（极低概率）

- cube 出现某个 desktop 需求，重交互到 HTTP/WebSocket 无法承载、且只在 desktop 形态下有意义——目前无法构象此类需求，视为不存在；
- 若仅想要「窗口 + 托盘」，走零框架土办法（Go 侧 `webview`/`systray` 或系统命令开浏览器）即可，不构成 wails 立项理由。

## 相关方向

- 个人 Mac 工具线的投入方向定为 **Swift/SwiftUI**（低占用小工具）。
- 若未来要做「重前端的新产品」（cube 只是数据源之一）：独立 Tauri 项目 + 直接消费 cube 现有 HTTP API，与 cube 各自演进，cube 本身零改动。
