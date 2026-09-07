# 1044 forge 模型简化：account-only（推翻 namespace）

## 状态

**已定案，实施中**。推翻 1041 的 namespace 设计；1042 forge 页随之调整数据源。

## 背景

1041/1042 落地后真机验证发现致命的平台语义问题：**「按命名空间列仓库」在主流平台没有统一可靠的 API**：

- github `GET /users/{user}/repos` 官方明示只列**公开**仓库（文档原文 "Lists public repositories for the specified user"），带有效 token 也不含私有库；
- gitee 同语义（实测 `/users/heyuuu/repos` 带 token 只回 1 个公开库），私有库必须走 `GET /user/repos?affiliation=owner`（实测 19 个含 18 私有）；
- org 端点（`/orgs/{org}/repos`）虽按 token 可见性返回私有库，但实测 gitee 会混入**访问不了的幽灵条目**（列表有、详情 404，疑似已删除/已转移残留或仓库级 ACL 未过滤），列表不可信。

即：模拟「namespace 列表」需要按 personal/org、有无 token 分叉到不同端点，且部分端点输出不可信——`ListNamespaceRepos` 语义混乱的根源就在这里。

## 结论（讨论收敛）

**只以 account 为准，只以认证账号端点为准。** 三平台的 `GET /user/repos` 都是「列表即真相」（返回的每一条都可访问）：

- github：affiliation 默认含 `owner,collaborator,organization_member`——个人私有库 + 所属组织仓库一次拉全；
- gitea：`/api/v1/user/repos` 同语义（按请求者可见性）；
- gitee：`/api/v5/user/repos`（实测 21 个跨 4 个 namespace，每条自带 `namespace.path` 归属）。

namespace 想解决的「组织仓库怎么拉」被 account 端点天然覆盖（组织成员身份即可见），无需单独建模。

## 变更

- **模型**：`Namespace` 删除（`type`、`accountUsername` 挂载、`forgeNamespaces` 节、detect 探测全部消失）。保留 `Forge{host,kind,icon}` + `Account{forgeHost,username,token}`。
- **gitapi**：`ListNamespaceRepos` / `DetectNamespace` 换成 `ListAccountRepos`（三平台统一 `/user/repos` 分页拉全量），`NamespaceType` 删除。
- **拉取**：挂 account 上（`POST /api/forge/account/fetch`），按 account 缓存（`cache/forge-repos.json` 键改为 `host/username`，含 fetchedAt）。
- **对账**：`RepoKey` 全局匹配；孤儿 = 本地 remote host 命中任一已配 account 的 forge、且不在该 forge 全部账号仓库并集里。
- **forge 页**：namespace 摘要条 → account 摘要条；owner（`full_name` 前缀）降为纯展示分组维度，不再是配置实体。
- **出口**：删 `forge/namespace/*` 5 端点；settings 页删命名空间子表。

## 不做 / 损失

- 「围观」无成员关系的组织的公开仓库列表——低频场景，未来有需求再加可选的只读 namespace 配置，与 account-only 不冲突。
- token 为空 = 匿名，匿名无 `/user/repos` 可用 → 拉取报错提示需配 token（公开 namespace 浏览场景放弃）。
