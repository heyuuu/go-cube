# 前端栈迁移

> **状态**：📋 后续待办（单独大需求）
> **来源**：cube-next 吸收讨论 + v3-frontend.md 历史设计

## 目标

前端从当前 vanilla JS（Alpine.js）整体迁移到 Vite + React + TypeScript + React Query + Tailwind + shadcn 风格组件。

当前前端是 `server/web/ui/` 下的原生 HTML/JS（app.js + views/ + vendor/），无构建链。迁移是整体替换，不是渐进改造。

## 关键技术决策

### 1. 前后端契约不走 HTTP

用 `cube openapi` 命令本地生成 `openapi.json` 文件，再用该文件通过 [openapi-typescript](https://github.com/drizzle-team/openapi-typescript) 生成 TS 类型。

**不采用**依赖 server 在线的方案（那会导致「构建前端必须先起 server」的依赖）。cube 已有 `cube openapi` 命令，天然支持本地生成。

### 2. React Query 替代手写 fetch

cube 有「projects 列表轮询 git 状态」场景（gitcache 定期刷新），React Query 的 `refetchInterval` 正好契合。

### 3. UI 组件用 shadcn 风格

cva + clsx + tailwind-merge + lucide-react（cube-next 已验证）。

## 数据流与实时性设计（来自 v3-frontend.md，机制不变）

核心策略：**复用后端 gitcache，前端只做轻轮询**。

- 后端有 git 信息缓存机制（`project/gitcache`）：常驻 server 进程内 goroutine 定时采集（默认 5 分钟）回写 git.json；读 API 读内存快照（不阻塞）。
- 前端用 React Query 的 `refetchInterval`（如 30s）重新 `GET /api/project/list` 即可拿到**已被 server 定时刷新**的新快照。
- **前端不自己采集 git 信息，也不需要 SSE**。后端定时采集 + 前端轮询拉快照，是已验证的成熟链路。
- 预留 SSE hook 位（空实现），仅当未来「批量 pull 进度」等场景真需要实时推送时再填。

详细的数据流设计（Query keys / Mutations 失效策略等）见历史文档 `docs/design/v3-frontend.md` 第六节（该文档即将删除，如需参考请从 git 历史查阅）。

## 页面规划（260815 讨论定稿）

> 本节是页面规划讨论后的定稿结论；下方「页面与路由设计思路」是 v3-frontend 历史参考，冲突处以本节为准。

### 已定决策

| 决策点 | 结论 |
|---|---|
| 路由模式 | history 路由 + Go 侧 `web/static.go` 加 SPA fallback（未匹配的 GET 回 index.html）。放弃旧 UI 特意选的 hash 模式（当时的理由是零后端改动，本次接受这处小改动） |
| Config 页 | 降级为**只读**。后端本就没有 config 写 API（旧 UI 的增删调用的是从未实现的端点）；写能力作为后续独立提案（命令式 CRUD + 变更生效机制），届时页面从只读升级 |
| 导航占位 | sidebar 只放当前可用页面，不放「规划中」死链接；workspace 深度档位定了再加入口 |
| 项目详情形态 | 终态**双形态**：抽屉（列表页快览，不占路由）+ 独立详情页（承载 info -v Web 化后的深度信息——分支差距宽表/未提交文件，随详情增强需求落地；双形态共享组件）。本次仅做抽屉，只含现有信息量 |
| 本次范围 | **仅基线**：工程脚手架 + 现有三页 1:1 迁移 + history 路由。详情增强 / md 页 / 批量 pull-push / clone 发起均不在本次 |

### 终态页面地图

```
布局：左 sidebar（域导航）+ 右内容区，无全局 header
导航：Projects / Config（+ 底部 API Docs 外链），页面落地逐个加入口

/projects            项目列表：表格/树双模式、搜索、group/git/tag 筛选、
                     scan/git 采集时间新鲜度、行内打开方式菜单、复制路径
                     （远期增强，均需后端新 API：批量 pull/push、
                       最近使用排序——依赖 history 写入面铺开）
/projects/:name      独立详情页（远期，随 info -v Web 化落地；name 的编码
                     细则实现时定）——与抽屉双形态共享组件
  └ /…/diff          diff 页（远期，v3-frontend 遗留设想，待后端 diff API）
详情抽屉             列表页点行滑出，不占路由：基本信息 / git 快照 /
                     打开方式 / 复制路径
/md?path=            markdown 渲染页（md 渲染提案，依赖 /api/md/content）。
                     path 走 query 参数——绝对路径 encode 进 path 段太丑。
                     注意：md 提案中 cube md 的 ensureServer() 与现行
                     「不做 lazy 拉起」（现状.md 3.6）冲突，启动该提案时先澄清
/config              只读配置：scan/clone/opener 规则展示 + 运行信息
                     （dataDir / 版本）；写能力 = 后续提案
/workspace           工作台（workspace 提案，远期）：代码阅读 / Git 可视化 /
                     终端，三方向深度档位未定前不做页面设计，只保留分支位
```

### 明确不进 Web 地图的

- `alfred`（外部 workflow 集成）、`server start/stop/status`（CLI/HTTP 管理域，页面本身就跑在 server 上）、`init`（交互式终端场景）、`version` / `openapi`（开发命令）、MCP（另一出口）
- clone 发起、check 视图、create（模板引擎）向导、全局任务中心——讨论定稿**不进终态地图**，保持 CLI 或后续两说

### 本次执行清单（基线）

1. **工程脚手架**：Vite + React + TS + React Query + Tailwind + shadcn 风格组件（cva / clsx / tailwind-merge / lucide-react）。源码仍在仓库根 `ui/`，构建产物走既有 `make build` 嵌入链路（vite build → `server/web/ui` → go:embed）
2. **契约链路**：`cube openapi` 本地生成 openapi.json → openapi-typescript 生成 TS 类型 → `api/client.ts` 统一 unwrap envelope（业务代码只见纯数据）
3. **Go SPA fallback**（`web/static.go`）：未匹配的 GET 返回 index.html，支持 history 路由刷新/直链
4. **`/projects` 列表 1:1**：表格/树双模式、搜索、group 多选 / git / tag 筛选、React Query 30s 轮询、行内 opener 菜单、复制路径、prettyPath；批量选择条按 1:1 保留占位（实现时可精简为仅多选计数）
5. **详情抽屉 1:1**：现有信息量（基本信息 / git 快照 / 打开方式 / 复制路径）；抽屉的刷新/直链定位形态（query param or 纯不占路由）实现时定
6. **`/config` 只读版**：三个规则区块只读展示 + 运行信息

## 页面与路由设计思路（来自 v3-frontend.md，供参考；路由细节以上方定稿为准）

- **布局**：B 端看板，左 sidebar + 右正文，无全局 header。
- **路由**：`/project`（列表）、`/project/info`（详情抽屉，不占路由）、未来 `/project/groups`（批量）、`/project/p/:path/diff`（diff，待后端 API）。
- **列表页**：扁平表格，搜索/group 筛选/git 筛选在前端做（量级几十个，无压力），后端 list 不带 query。
- **详情**：右侧 Drawer（Sheet），点行滑出，上下文不丢。

## envelope 处理

后端统一 `ApiOutput{ok, message, data}` 包裹。前端 `api/client.ts` 在 generated SDK 之上包一层，统一 unwrap：`ok` 为 false 时 throw（让 React Query 走 error 分支），否则返回 `data`。业务代码只见纯数据。

## 备注

本次讨论不做任何前端代码改动，现有 Alpine.js 前端维持运行。

## 落地结果（260817，基线清单全部完成）

1. 工程脚手架：源码定在仓库根 **`web/`**（非提案所写的 `ui/`），栈为 Vite 8 + React 19 + React Compiler + tsgo + Tailwind 4 + **Base UI 风味 shadcn**（`rsc: false`）+ React Query；oxfmt/oxlint 管格式与 lint，`verify:shadcn` 守基件无漂移。
2. 契约链路：`cube openapi` → openapi-typescript → `api/client.ts` 泛型分发（`apiGet`/`apiPost` 扁平传参，必填 query 编译期强制，契约由 `client.type-test.ts` 锁定）。
3. Go SPA fallback 已实现（`/api`、`/docs`、`/openapi.json` 不回退）。
4. `/projects`：列表 + 树双模式（`?view=tree`）；**树改为前端自计算，后端 `/api/project/tree` 已移除**（连带删除 `project/tree.go`）；筛选谓词化、徽标点击联动、行点击开抽屉。
5. 详情抽屉（Sheet）：基本信息 / git 状态 / 打开动作 / 复制路径；未采集降级提示。
6. `/config` 只读页：基本信息 / scan / clone / openers。

顺带修复：`web/jsonfmt.go` 深拷贝清零所有 `time.Time`（响应时间字段全为 0001-01-01）的序列化 bug。旧 `ui/` 目录已删除。
