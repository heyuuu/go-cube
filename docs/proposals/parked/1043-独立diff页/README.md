# 独立 diff 页

> **状态**：⏸️ 挂起（方向初步认可，设计未想透——是否独立成单独工具、多源边界、监听语义均待定；解挂条件见文末）

## 背景：与面板页 diff 的定位差异

现状 workbench 的 diff 能力（`content` 面板 base→current 模式、`/api/workbench/diff` + `/api/workbench/file-diff`）是 **repo 中心**的：两侧源必须落在同一仓库的 TreeSource 体系内，且目录对比是全量 `DiffTrees`。

设想中的独立 diff 页对标 Beyond Compare，是**路径中心**的通用对比工具，差异在四点：

1. **更多类文件系统的源**：本地目录、sftp 目录、git repo 的任意 commit + 任意子目录（现有 `util/git/tree.go` 的 `ListFiles` / `FileShasAtRef` / `ReadFileAtRef` 已覆盖 git 侧原语）。
2. **渐进式获取**：因对目录规模的预期不同（可能指向远端/超大目录），不做全量 listing 比对，改为按目录懒对比——展开节点时对该目录做一次两侧 listing + sha 比对，文件级 diff 点开才算。需要一个统一 `Source` 接口（`ListDir` / `ReadFile` / `Stat`），对比逻辑为纯函数。
3. **文件对比**：现有 file-diff 返回结构化 hunks，可泛化为「两侧各一个任意 Source 路径」。
4. **文件监听**：文件变动实时刷新。按源区别对待——本地 fs 用 fsnotify + WebSocket 推送（PTY 已有 WS 先例）；git 源实为 ref/HEAD 变化，轮询 HEAD sha 即可；sftp 无可靠事件机制，降级手动刷新。

## 待定问题（解挂前必须想清）

- **是否做成独立工具**（独立仓库 + 独立 server/前端）还是 cube 内新 domain + 独立路由 `/diff`。当前倾向后者：70% 地基已有（TreeSource 抽象、git 原语、hunks 渲染、WS 基建），独立工具等于复制全部基建，且会断掉 opener 的 `diff-dir`/`diff-file` 入口与项目上下文；但「通用对比工具」与「cube 项目管理」的生态位张力未充分评估。
- **sftp 源是否进 v1**：引入 `golang.org/x/crypto/ssh` 依赖与远端配置管理（挂哪、凭证存哪），可先 fs + git 两源验证接口设计。
- **与 workbench 面板 diff 的入口分工**：workbench 内对比留面板，diff 页承接跨项目/跨目录/任意源场景——入口从项目列表 ⋯ 菜单与 opener `diff-dir` 加动作串接，具体交互未设计。
- **落地形态预判**（若留 cube）：五处加法——新 domain `diff`（Source 接口 + fs/git 实现 + 懒对比纯函数）、可选 `cmd/diff-page`、`handlers/diff_handler.go`、前端 `/diff?left=&right=` 独立路由（类 `/md`，不进主 Layout）、app 装配。

## 解挂条件

- 明确使用场景与频率（自用对比的真实痛点是否足够支撑一个页面/工具的维护成本）；
- 上述「独立工具 vs cube domain」与 sftp 边界两个决策有结论；
- 届时重估与 workbench diff 面板的能力重叠，避免两套 diff 交互并行演化。
