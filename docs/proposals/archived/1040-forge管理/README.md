# 1040 forge 管理

## 状态

**已实现并归档**（2026-09-05）。落地形态见下「最终设计」与 现状.md 3.11 / 6.5。

## 背景

cube 的项目全是 git 项目，每个项目的 remote URL 都指向某个 git 托管平台实例——GitHub / Gitea / Gitee / 自建 Gitea。目前 cube 对这些平台一无所知：项目列表里看不出某仓库托管在哪，也无法按托管平台筛选。后续的 account 拉取（1041）与 forge 页（1042）都需要一个统一的「平台实例」概念作为挂靠点。

术语约定（系列三提案共用）：

- **Forge**：git 托管平台实例，host 级一条配置（术语来自 git forge 生态，Gitea/Forgejo 同源）。限定 git 仓库的托管平台，不覆盖 SVN 等非 git 托管。
- **Account**（1041）：某 forge 上的身份 + 凭证。
- **Owner**：仓库归属的命名空间（个人 / org）。1041 明确暂不做 owner 区分，但数据模型设计时不得与 account 概念混淆。

## 最终设计

- **数据**：`Forge{Host, Kind, Icon}` 存 settings.json `forges` 节。host 归一化（去空白 + 小写 + 去尾部点号）后是唯一键，写侧校验纯域名[:端口]（拦住粘贴完整 URL 的手误）；编辑 host = 删旧存新（同 scanRule path 语义）。
- **kind 四值**：`github` / `gitea` / `gitee` / `generic`。kind 决定 1041 account 拉取的 API 方言；generic = 无 API 仅展示。
- **五处加法落地**：`forge` 领域包（`types.go` 纯函数 + `service.go` 直读不缓存 / 写侧校验，模式同 scan/clone 规则）、`cmd/forge.go`（裸跑 = `forge list` 表格输出）、`handlers/forge_handler.go`（list / save / delete 三端点）、`app.go` 装配。无 config.json 节（无端口类配置需求）。
- **repo→forge 匹配放前端**：前端从 settings API 拿 forge 列表，按项目快照 `gitInfo.repoUrl` 解析 host 查表（后端 `forge.RepoHost` 纯函数，前端 `lib/forge.ts` 的 `repoHostOf` 同语义复刻，均有表驱动测试）；projects 列表与树两模式在项目名称旁展示 forge icon（悬停 title=host）。**不把 forge 信息写进 git.json 快照**，读路径零改动；未匹配 host / forge 未配 icon 不展示（无兜底图标）。
- **前端**：settings 页新增 Forge 分区（Sheet 抽屉表单 + 删除确认，模板沿用扫描规则分区；kind 用 chip 单选；无顺序语义不做拖拽）。
- **IconField 提取入口泛化**（随本提案补全）：原「.app 提取」升级为「路径 / URL 提取」——`POST /api/icon/extract`（中性 icon 路由，替代 opener/extract-icon）：本地图片文件、.app 目录（icns）、http(s) URL（favicon.ico / png / jpg / gif）三形态统一 64px PNG base64；ICO 容器由 iconkit 手解（favicon 主流形态标准库不支持）。

## 已知取舍

- 不做 favicon 自动抓取兜底、不做 kind 自动识别 / 连通性探测（用户手选 kind）——保持 forge 配置零外呼。
- 未配 icon 的 forge 在项目列表无任何视觉呈现（匹配只服务于 icon 展示）；1042 forge 页落地时再考虑按 forge 筛选。
