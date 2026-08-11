# md 渲染（以 Web 方式打开 Markdown）

> **状态**：📋 待办
> **依赖**：
> - [`260811-server按需启动`](../260811-server按需启动/)（CLI 触发时确保 server 在跑）
> - [`260811-前端栈迁移`](../260811-前端栈迁移/)（渲染页面归属前端工程）

## 背景

cube 需要一种"以 Web 方式打开 Markdown"的能力：本地 `.md` 文件渲染成 HTML 在浏览器里查看。

讨论过几种形态（Web 内渲染浏览 / 调用浏览器打开 / 托管文档站点），结合 cube 的定位（本地工具、CLI 优先 + 本地 Web）和触发场景（CLI 命令触发），结论是 **Web 内渲染浏览**。

### 关键设计原则：渲染完全归前端工程

**后端不做任何模板渲染**——不引入 goldmark、不写 HTML 模板、不内联 CSS。理由：

- cube 已规划前端栈迁移（`260811-前端栈迁移`，Vite + React + TypeScript），未来所有 Web 页面都走前端工程
- 后端渲染的 Web 能力和迭代便利度远低于前端工程，现在投后端模板代码等于给未来添障碍
- md 渲染页面是未来 SPA 的其中一个页面，具体路由（`/md`、`/md/:path` 等）等前端工程起来后定

所以本提案**依赖前端栈迁移先落地**。后端只提供"读本地 md 文件内容"的 API，渲染、样式、交互全部前端管。

## 目标

`cube md <path>` —— 读本地 markdown 文件，浏览器打开前端 SPA 的 md 渲染页面。

```
$ cube md /abs/path/to/README.md
# 1. ensureServer() —— 确保 server 在跑（依赖 server 按需启动提案）
# 2. 开浏览器指向 http://localhost:<port>/<md 页面路由>?path=<abs>
# 3. cube 命令退出（server 留后台）
# 浏览器里：前端页面调 /api/md/content 拿 markdown 原文 → 前端渲染
```

## 要做的事

### 1. 后端：读文件 API

**`GET /api/md/content`** —— 走 huma，套 ApiOutput envelope
- query 参数 `path` = 本地 markdown 文件的绝对路径
- 返回该文件的** markdown 原文**（纯文本，不做任何渲染）
- 前端拿到原文后自行渲染（react-markdown / marked 等）

这是本提案后端**唯一**要做的事。不做 `/md/` 页面路由、不做后端 HTML 模板、不引入 markdown 渲染库。

### 2. 前端：md 渲染页面（SPA 的一个页面）

归属前端工程（Vite + React），作为 SPA 的其中一个页面/路由：
- 具体路由（如 `/md`、`/md/:path`）等前端工程起来后定
- 页面从 URL 拿 `path` 参数 → 调 `GET /api/md/content` 拿 markdown 原文 → 前端渲染
- 渲染库、主题、样式、交互全部前端决定
- 这部分的实现细节随前端栈迁移一并落地，本提案只定义需求和接口契约

### 3. CLI 子命令：`cube md`

- 输入：一个 `.md` 文件路径（相对路径基于 cwd 解析，`~` 展开，转绝对路径）
- 行为：调 `ensureServer()` → 拼出前端 md 页面 URL（含 `path` 参数）→ 调系统浏览器打开 → 命令退出
- 落在 `server/cmd/md.go`，挂在 root cmd 下（扁平，不分组）
- 复用 `server/cmd/server.go` 里的 `openBrowser()`（已封装 macOS `open <url>`）

## 架构落位

**不新建 domain 包**。理由：md 读文件本质是"把本地文件内容通过 HTTP 返回"的出口层动作，没有实体/规则/持久化状态，不符合"加新 domain 走五处加法"的前提。

落地文件（后端部分）：
- `server/cmd/md.go` —— CLI 子命令
- `server/web/api_md.go` —— `/api/md/content` handler（走 huma，和 `api_project.go` / `api_opener.go` 同构）
- 不动 `app/app.go`（md 读文件不需要 service 装配，handler 自包含）
- 不动 config（无配置项）

前端部分归属 `server/web/ui/`（前端栈迁移后的工程），具体文件结构随前端工程定。

## 路径范围

**宽范围，本地绝对路径。** `cube md` 接受任意本地 `.md` 文件路径，不限定在 cube 的 docs/ 或某个 project 下。md 作为通用工具动作，限定目录反而别扭。

未来可扩展为 URL / Git 地址等远端来源，本提案不涉及，留作后续议题。

## 安全考量

cube 是本地工具但仍跑 HTTP server，读本地文件成 API 响应有风险点：

| 风险 | 对策 |
|---|---|
| **路径穿越 / 任意文件读取** | `path` 参数必须校验：`filepath.Clean` + 强制绝对路径 + **扩展名白名单**（仅 `.md` / `.markdown`）。绝不能直接把 query 参数喂给 `os.ReadFile` |
| **URL 编码绕过** | `path` 参数解析后校验，不信任原始 query string |
| **XSS**（markdown 内容里的恶意脚本） | 后端只返回原文，渲染在前端——XSS 防线在前端渲染库（react-markdown 默认转义 HTML，不用 marked 的话需额外 sanitize） |
| **DNS rebinding** | 本提案不单独处理，作为 server 按需启动提案的安全增强项（Host 头校验） |

## 依赖与排序

```
260811-server按需启动   (无依赖)
        ↓
260811-前端栈迁移        (无强依赖，可与上一个并行)
        ↓
260811-md渲染            (依赖前两者都落地)
```

本提案必须在前端栈迁移落地后才能完整实现（前端页面部分）。后端 API + CLI 骨架可以先行，但端到端可演示要等前端工程。

## 工作量评估

- 后端 `/api/md/content` handler + 路径校验：小
- `cube md` 子命令 + `ensureServer` 调用：小（依赖 server 按需启动提案就绪）
- 前端 md 页面：随前端栈迁移一并做，工作量计入前端工程
- 不动 app / config / 现有 domain
