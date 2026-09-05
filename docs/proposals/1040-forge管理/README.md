# 1040 forge 管理

## 状态

**待实施**。本提案是 forge 系列三提案（1040 / 1041 / 1042）的地基，先行落地。

## 背景

cube 的项目全是 git 项目，每个项目的 remote URL 都指向某个 git 托管平台实例——GitHub / Gitea / Gitee / 自建 Gitea。目前 cube 对这些平台一无所知：项目列表里看不出某仓库托管在哪，也无法按托管平台筛选。后续的 account 拉取（1041）与 forge 页（1042）都需要一个统一的「平台实例」概念作为挂靠点。

术语约定（系列三提案共用）：

- **Forge**：git 托管平台实例，host 级一条配置（术语来自 git forge 生态，Gitea/Forgejo 同源）。限定 git 仓库的托管平台，不覆盖 SVN 等非 git 托管。
- **Account**（1041）：某 forge 上的身份 + 凭证。
- **Owner**：仓库归属的命名空间（个人 / org）。1041 明确暂不做 owner 区分，但数据模型设计时不得与 account 概念混淆。

## 需求

1. 支持 forge 配置管理，数据形态 `Forge { host, kind, icon }`：
   - `host`：如 `github.com`、`gitea.example.com`，唯一标识，一个 host 一条 forge。
   - `kind`：API 方言枚举，`github` / `gitea` / `gitee` / `generic`。kind 决定 1041 的 account 能否拉取（generic = 无 API，仅展示）；gitee API 与 GitHub/Gitea 均不兼容，第一版是否实现 gitee 拉取见 1041。
   - `icon`：复用 1029 已下沉的 `util/iconkit` + 前端 `IconField`。
2. 前台 settings 配置页支持增删改 forge（分区形态，参考 opener / scanRules 分区）。
3. 支持「从 repo 推导 forge」：由项目 remote URL 解析 host，匹配已配置 forge，用于前端展示 icon 与（1042）按 forge 筛选。

## 设计要点（讨论收敛）

- **存储：settings.json 新增 `forges` 节**，沿用 opener / scanRules 的节级 API 模式（Service 直读不缓存、写侧领域校验）。
- **repo→forge 匹配放前端**：前端从 settings API 拿 forge 列表，按项目快照里的 remote URL 自行解析 host 匹配；未匹配 host 走兜底展示（generic 图标）。不把 forge 信息写进 git.json 快照，读路径零改动。
- **新 domain 走「五处加法」**：`forge` 领域包（本提案先只有配置 CRUD 与 host 匹配纯函数）、`cmd/forge` 子命令组（CLI 兜底）、`handlers/forge_handler.go`、config/settings 节、`app/app.go` 装配。纯加法，不碰现有 domain。
- host 匹配（URL 解析 host、host 归一化如端口/大小写）是纯函数，表驱动测试。

## 不做

- 不做 forge 的连通性探测 / kind 自动识别（用户手选 kind）。
- 不做 favicon 自动抓取兜底（generic 用统一默认图标）。
