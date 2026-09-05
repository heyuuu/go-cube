# 1041 forge account 管理与远端仓库拉取

## 状态

**待实施**。依赖 1040（forge 配置与 kind 枚举）先行落地。

## 背景

只配 forge 还不知道「我在这个平台上的仓库有哪些」。个人开发者的真实痛点是对账：远端账号下有多少仓库、哪些还没 clone 到本地、本地哪些库远端已删（孤儿）。这需要 forge account——在某 forge 上的身份 + 凭证——通过平台官方 API 拉取账号下的仓库列表。

概念澄清（讨论已收敛）：

- **Account 是「身份 + 凭证」**：`{ forgeHost, username, token }`。token 挂在 account 上。
- **Owner 是「仓库归属的命名空间」**（个人空间 / org 空间），与 account 正交：一个 account 可管理多个 owner。**本提案暂不做 owner 区分**——拉取范围仅为账号自身的仓库（GitHub `/users/{me}/repos` 语义），org 仓库留待后续（数据模型不动，加拉取目标即可）。

## 需求

1. 支持 account 配置管理：`Account { forgeHost, username, token }`，settings 页在 forge 分区内配置（每条 forge 下挂账号）。
2. 分别支持 **github / gitea / gitee** 三种 kind 的「拉取账户下 git 仓库列表」：
   - github：`GET /user/repos`（或 `/users/{name}/repos`）。
   - gitea：`GET /api/v1/users/{name}/repos`（自建 Gitea 主要场景）。
   - gitee：Gitee OpenAPI `GET /api/v5/users/{username}/repos`（方言独立实现）。
   - generic：无 API，不支持拉取（配置 account 时前端禁用/提示）。
3. 拉取结果缓存并支持对账展示的基础数据：远端仓库名、clone URL（归一化）、默认分支、更新时间等（具体字段以对账需求为准）。
4. 与本地项目对账：按 clone URL 归一化匹配，产出三类——**未 clone 的远端库 / 本地孤儿（远端已无）/ 已 clone（含本地状态：dirty/ahead，取自 projcache 快照）**。对账是纯函数，表驱动测试。

## 设计要点（讨论收敛）

- **拉取是出站 API 调用，不进 projcache 常驻异步采集**：手动/低频触发（前端按钮或 API 调用），结果经 easycache 缓存；失败降级只影响该账号的展示，slog 记录后继续。与「不做云服务」定位不冲突——cube 是纯客户端外呼。
- **API 客户端按 kind 分文件内聚**（`forge/remote_github.go` / `remote_gitea.go` / `remote_gitee.go`），统一返回内部 `RemoteRepo` 类型；HTTP 层可注入 fake 测解析与对账逻辑，不打真实外站。
- **token 明文存 settings.json**：与现有形态一致（本地个人工具，settings 里已存 exec 命令串）；settings 页 token 字段做掩码展示。不做加密存储。
- gitee 方言是否随第一版实现，实施时按工作量决定，架构上已隔离在独立文件，后补无成本。

## 不做

- 暂不做 owner / org 空间拉取（模型预留，见背景）。
- 不做远端写操作（建仓 / 删仓 / push）。
- 不做 token 有效期的定时探测（拉取失败自然暴露）。
