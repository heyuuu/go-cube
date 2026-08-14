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

## 页面与路由设计思路（来自 v3-frontend.md，供参考）

- **布局**：B 端看板，左 sidebar + 右正文，无全局 header。
- **路由**：`/project`（列表）、`/project/info`（详情抽屉，不占路由）、未来 `/project/groups`（批量）、`/project/p/:path/diff`（diff，待后端 API）。
- **列表页**：扁平表格，搜索/group 筛选/git 筛选在前端做（量级几十个，无压力），后端 list 不带 query。
- **详情**：右侧 Drawer（Sheet），点行滑出，上下文不丢。

## envelope 处理

后端统一 `ApiOutput{ok, message, data}` 包裹。前端 `api/client.ts` 在 generated SDK 之上包一层，统一 unwrap：`ok` 为 false 时 throw（让 React Query 走 error 分支），否则返回 `data`。业务代码只见纯数据。

## 备注

本次讨论不做任何前端代码改动，现有 Alpine.js 前端维持运行。
