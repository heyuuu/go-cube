# Cube · v3 设计

> 个人开发者的本地开发工具集合平台。项目管理是首个 domain，未来平级加入 sqlite 管理等。
> 配套调研见 [research.md](./research.md)；前端设计见 [v3-frontend.md](./v3-frontend.md)。

## 一、定位

本地优先的开发者**工具集合平台**。多个 domain（project / db / ...）平级共存，共享平台基础设施（config / 存储 / CLI/Web 出口）。

- 不做云服务，不绑 AI（cube 可被 AI 编排，但自身不集成 AI）。
- 三个出口：CLI（人直接用，含 alfred）、Web（local，可视化管理 + 批量操作）、MCP（后续）。
- cube 可作为 MCP server 被外部 AI 平台编排，自身独立可用。

## 二、数据模型：project 与扫描规则

**两个核心概念**：

- **Project**：一等实体。主键 `path`（绝对路径，全局唯一），展示名 `name = group:subpath`。持有 `gitInfo`（来自 gitcache 快照）、`tags`（git / godot ...）。
- **ScanRule `{Group, Path, MaxDepth}`**：扫描规则。一条规则扫一个根目录，命中的项目归属该 `group`。`group` 是字符串字段（如 `github`、`personal`），承担"物理目录约定 + 分组聚合 + 批量作用域"三重价值。

> 历史注记：v2 曾有 `Workspace` 一等实体，v3 已简化为 `ScanRule` + `group` 字段——`group` 保留了 workspace 的全部实用价值（分组、批量作用域），但不再需要独立实体和 service。

**CloneRule `{RepoHost, RepoPrefix, LocalPath}`**：clone 路由规则。按 host + path 前缀匹配，算出本地落地路径（兼容 ghq 的 `host/path` 镜像约定）。

## 三、分层架构

```
基础设施  config / db / logger / version           所有层共享
能力      opener / util(git / gogit / fuzzy / easycache / pathkit / slicekit)  通用动作, 不含业务实体
领域      project (含 gitcache / scan / clone)       业务 domain, 含实体和规则
出口      cmd / web                                  把领域包成 CLI/Web
装配      app / main                                 接线
```

**能力 vs 领域**：`opener`（打开路径）、`util/git`（repo URL 解析）、`util/gogit`（调 git 子命令）、`util/fuzzy`（搜索）、`util/easycache`（缓存）是通用动作 → 能力层。`project`（有 Project 实体、扫描/clone 规则、gitcache）→ 领域层。

**依赖纪律**：基础设施不依赖上层；能力层只依赖基础设施；领域层依赖能力+基础设施；cmd 与 web 不互调；app 是唯一接线点。

## 四、git 信息缓存（gitcache）

cube 是 CLI 模式执行，每次 `project list --status` 要为每个 git 项目采集 git 信息，全量采集是 IO 密集型，反复跑会卡。`project/gitcache` 解决：

- **前台读命令**（list/info/Web API）从缓存读快照，几乎零开销。
- **后台子进程**异步采集并回写缓存（`TriggerAsyncRefresh` → fork 子进程，TTL 1 分钟内不重复触发）。
- 缓存文件 `~/.config/cube/cache/git.json`，跨进程 flock 保证同一时刻只有一个采集进程。
- `Entry{RepoUrl, CurrentBranch, DefaultBranch, Branches, Ahead, Behind, Dirty, CollectedAt}`。注意这些字段属于 `gitcache.Entry`：`Project` 通过 `GitInfo()` 返回 `*Entry` 访问（`Project` 自身只有 `RepoUrl()` 一个直通方法，其余 git 字段都经 Entry）。

这套机制是**前端实时性的基础**：Web 前端轮询拉 list 快照即可拿到被后台子进程刷新的 git 状态，无需 SSE（详见 [v3-frontend.md](./v3-frontend.md) 第六节）。

## 五、Web 端

`cube server` 起本地 HTTP 后端（huma 已落地，`/docs` Scalar、`/openapi.json` 自动生成）。**前端尚未接入**：`web/dist/` 与 `go:embed` 属 M4 待实现，当前 server 只暴露 API + docs。前端设计独立成 [v3-frontend.md](./v3-frontend.md)。

**后端：huma + 标准 ServeMux**：
- `web.NewServer(handlers ...Handler)` 持有 `*http.ServeMux` + `huma.API`，每个 domain 是一个 `Handler`，`Register(api huma.API)` 注册自己的路由。
- `apiRegister` 辅助函数：强制路径 `/api/` 前缀，从路径解析 group（作 tag）+ operationId，自动生成 summary。
- `apiGet/apiPost` + `jsonHandler`：统一 `ApiOutput{ok,message,data}` envelope 包裹所有响应。
- huma 自动生成 OpenAPI 3.1 spec（`OpenAPIJSON()` / `/openapi.json`），`/docs` 用 Scalar 渲染器。前端用 spec codegen 出 typed SDK。

**当前 API（动词式，全 GET 只读）**：
```
GET /api/project/list          # 项目列表（含 gitInfo 快照）
GET /api/project/info?name=    # 项目详情
GET /api/project/scan-rules
GET /api/project/clone-rules
GET /api/opener/list
GET /api/opener/info?name=
GET /api/config
```

**待补 API**（前端 F2/F3 阶段依赖）：
- `POST /api/project/open`（带 path + app）
- `POST /api/project/pull`（单项目 / group 批量）
- `GET /api/project/diff?path=`
- worktree 系列

**Web 出口结构**：
```
web/
  server.go            # NewServer + Start；持有 mux + huma.API
  api.go               # apiRegister/apiGet/apiPost/jsonHandler + ApiOutput envelope
  api_project.go       # ProjectHandler: Register(api) 注册 /api/project/*
  api_opener.go        # OpenerHandler
  api_config.go        # ConfigHandler
```
加 domain = `web.NewServer(..., web.NewDbHandler(...))` 多传一个 Handler，Server 零改动。

## 六、多 domain 平台骨架

**CLI**（`easycobra` 封装 cobra，子命令分组）：
```
cube project list / info / open / clone / scan-rules / clone-rules / check / init / tree / refresh-git-cache
cube opener list
cube alfred ...
cube server               # Web HTTP server
cube server openapi       # 生成 openapi.json (别名 api)
cube config / version
# 未来: cube db list / query
```

**配置**（`config/config.go`）按 domain/功能分节：
```json
{
  "log": {...},
  "project": { "scan": [...], "clone": [...] },
  "openers": [...]
}
```
未来加 domain 顶层加一节（`"db": {...}`），互不干扰。默认配置目录 `~/.config/cube/`。

**共享基础设施**：config 加载、db（gorm+sqlite）、logger（slog）、CLI/Web 出口框架被各 domain 共享。

**扩展模型**：加一个新 domain = ①领域包 ②CLI 子命令组 ③Web `Handler` ④config 加节 ⑤`app/init.go` 装配清单加构造。五处都是加法，不碰现有 domain。前端侧见 [v3-frontend.md](./v3-frontend.md) 第八节。

## 七、存储

- **配置态**（scan/clone/openers 定义）：config.json，读多写少。
- **事件态**（打开日志、frecency）：sqlite（`history` 包，gorm + AutoMigrate）。
- **git 缓存态**：`~/.config/cube/cache/git.json`（gitcache）。

## 八、里程碑

- **✅ 第 0 步**：包结构重组、去 wire、HTTP 表层合并
- **✅ M1 — 平台骨架**：config 分节、CLI 分组、Web huma 化、easycobra 封装
- **✅ git 信息缓存**：gitcache 异步采集机制
- **⏳ M2 — git 操作 + 批量**：`util/gogit` 完整操作；project 暴露 pull/fetch；group 维度批量
- **⏳ M3 — worktree**：创建/列出/删除
- **⏳ M4 — Web 端**：前端工程（[v3-frontend.md](./v3-frontend.md)）。F1 骨架当前可开工
- **⏳ M5 — MCP 出口**：官方 go-sdk，按 domain 分 toolset
- **⏳ M6+ — 新 domain**：sqlite 管理（`cube db`）、本地端口查看等

## 九、待细化

- M2 git 操作 API 形态（动词式 `/api/project/pull` vs RESTful）+ 批量并发/失败策略
- worktree 物理位置约定 + 是否纳入扫描
- diff API 返回结构（patch 文本 vs 结构化 hunks）
- history 包是否拆"机制(能力层) + project 应用"
- remote/clone 包归属（project 内部 / 独立）
