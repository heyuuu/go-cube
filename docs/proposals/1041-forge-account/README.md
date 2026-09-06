# 1041 forge account / namespace 与远端仓库拉取

## 状态

**已实施**（后端 + 前端 + 测试全绿；依赖的 1040 已归档）。

## 背景

只配 forge 还不知道「我在这个平台上的仓库有哪些」。个人开发者的真实痛点是对账：远端各命名空间下有多少仓库、哪些还没 clone 到本地、本地哪些库远端已删（孤儿）。这需要两层新配置 + 一个平台 API 拉取能力。

## 概念模型（讨论收敛定稿）

三层配置，各自职责单一，相互正交：

```
Forge      {host, kind, icon}                     // 1040 已落地，不动。host 级平台实例
Account    {forgeHost, username, token}           // forge 1—N account。纯 API 凭证
Namespace  {forgeHost, path, type, accountUsername?} // forge 1—N namespace。仓库归属的命名空间
```

- **Account 是纯 API 凭证**：token 唯一用途是调平台 API 拉仓库列表（私有库可见性）。cube 的 git 操作（clone/push）走系统 git 用本机凭证（ssh key / credential helper），token 从不参与 git 传输。account 语义上不对应任何 namespace。
- **Namespace 是仓库归属的命名空间**：即 URL 的 path 前缀（`github.com/heyuuu/xxx` 的 `heyuuu`），个人空间或 org 空间。与 clone 规则的 `repoPrefix` 天然对应，但数据上不耦合（独立配置，不从 cloneRules 推导）。
- **account 与 namespace 正交**：namespace 可选挂载一个 account（`accountUsername`）用于拉取其下私有仓库；不挂也能拉（公开 namespace 匿名可拉，限流内）。不设 forge 级默认 account（namespace 配置量小、显式声明一眼见底，避免两级回落歧义）。
- **type ∈ {personal, org}**：决定拉取端点（`/users/{path}` vs `/orgs/{path}`），从 git url 看不出来，显式配置、默认 personal。配置表单提供自动探测辅助回填（见设计要点），可手动覆盖。

## 需求

1. **Account 配置管理**：`Account { forgeHost, username, token }`，唯一键 `forgeHost+username`。校验 forgeHost 必须命中已配置 forge 且 kind ≠ generic。
2. **Namespace 配置管理**：`Namespace { forgeHost, path, type: personal|org, accountUsername? }`，唯一键 `forgeHost+path`。path 保留原样大小写、trim 空白与首尾 `/`，匹配时小写比较；accountUsername 须命中该 forge 下已配置 account（或留空）。
3. **namespace 自动探测**：按 kind 调探测端点（GitHub `GET /users|/orgs/{path}`、gitea `/api/v1/users|orgs/{path}`、gitee `/api/v5/users|orgs/{path}`），命中即返回 `personal | org`，两者皆未命中返回「未找到」。探测失败不阻塞配置（字段可手选）。
4. **拉取 namespace 下仓库列表**（github / gitea / gitee 三方言）：
   - github：`GET /users|/orgs/{path}/repos`
   - gitea：`GET /api/v1/users|orgs/{path}/repos`（自建 Gitea 主要场景）
   - gitee：Gitee OpenAPI `GET /api/v5/users|orgs/{path}/repos`
   - generic：无 API，不支持（配置页禁用拉取入口）。
   - 带 token 时可见私有库，分页拉全量。结果字段以对账需求为准：远端仓库名、clone URL（归一化）、默认分支、更新时间等。
5. **与本地项目对账**：按 clone URL 归一化匹配（复用 `git.ParseRepoUrl` 归一到 `host + namespace/path + repo` 小写形式再比对，两边同走一个函数），产出三类——**未 clone 的远端库 / 本地孤儿（远端已无）/ 已 clone（含本地状态 dirty/ahead，取自 projcache 快照）**。对账是纯函数，表驱动测试。

## 不做

- 不做远端写操作（建仓 / 删仓 / push）。
- 不做 token 有效期的定时探测（拉取失败自然暴露）。
- gitee 企业（enterprise）类型暂不支持：API 独立（`/api/v5/enterprises/{name}/repos`），拉取时明确报错，模型不预留字段。
- 不做 forge 级默认 account。

## 设计要点（讨论收敛）

- **API 客户端封装为能力层 util 包 `util/gitapi`**（与 `util/git` 成对：`git` 管本机 git 子进程，`gitapi` 管平台 REST API）：
  - 统一 interface + 按 kind 多实现（`Client` 接口，`github.go` / `gitea.go` / `gitee.go`），构造按 kind 分发，方便后续加新的 git forge。
  - 对外暴露统一类型：`RemoteRepo{ name, cloneUrl, defaultBranch, updatedAt }` 等，不含 cube 业务实体；调用方（forge 领域包）做业务映射。
  - HTTP client 可注入 fake，测试不打真实外站。
  - 包内函数保持 util 纪律：只依赖入参做网络调用，不读 cube 进程状态/配置。
- **拉取是出站 API 调用，不进 projcache 常驻异步采集**：手动/低频触发（前端按钮或 API 调用），handler 内同步执行、HTTP client 设短超时（~10s），结果经 easycache 缓存；失败降级只影响该 namespace 的展示，slog 记录后继续。与「不做云服务」定位不冲突——cube 是纯客户端外呼。
- **token 明文存 settings.json**：与现有形态一致（本地个人工具，settings 里已存 exec 命令串）；list 端点返回时 token 打码，前端提交掩码值视为「未修改」沿用旧值。不做加密存储。
- **探测独立成轻端点**：`POST /api/forge/namespace/detect`，配置表单填 path 后自动/手动触发回填 type。
- **clone URL 归一化规则收敛在 forge 领域包**（复用 `git.ParseRepoUrl`），对账两端同走一个函数，表驱动测试覆盖 ssh/https/带端口/尾缀 `.git` 各形态。

## 领域与出口落地形态

- **领域包 `forge`**：新增 `Account` / `Namespace` 类型 + settings 节 `forgeAccounts` / `forgeNamespaces` 的读写（直读不缓存、写侧校验，模式同 forges 节）；拉取与对账的编排（调 `util/gitapi` + easycache 缓存 + 对账纯函数）。
- **出口 `handlers`**：`forge_handler.go` 扩展，API 沿用 GET（查）/ POST（动作）双 method 惯例：
  - `GET /api/forge/account/list`、`POST /api/forge/account/save`、`POST /api/forge/account/delete`
  - `GET /api/forge/namespace/list`、`POST /api/forge/namespace/save`、`POST /api/forge/namespace/delete`、`POST /api/forge/namespace/fetch`、`POST /api/forge/namespace/detect`
- **前端 settings 页 Forge 分区**：forge 条目下挂 account 与 namespace 的增删改（token 掩码展示；kind=generic 时禁用拉取相关入口）；namespace 表单带 type 选择与自动探测；对账结果做最小可用展示（Sheet/抽屉按 namespace 列出三类），不做独立页面。
