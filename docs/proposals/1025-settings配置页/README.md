# settings 配置页

> **状态**：📝 提案（2026-08-24，方向已与 owner 对齐）
>
> **依赖**：[`1016-opener改造`](../1016-opener改造/README.md)第 1-2 步（opener 迁 settings.json + save/delete API）——settings 首个分区「Opener」的编辑能力由它提供；骨架可先行，编辑能力等 1016。
> **关联**：[`1024-projects筛选URL化`](../1024-projects筛选URL化/README.md)（无硬依赖，但其完成后「业务页 ↔ settings 往返无损」才成立）。

## 背景与目标

现在的 `/config` 是 config.json 的只读展示。随着 1016 把 opener 迁入 settings.json 并提供 Web 增删改，以及未来 scan 规则等更多配置进 settings.json，需要一个**真实可编辑的配置管理页**，且语义与「config.json 只读事实」分开：

- **settings** = 用户可管理的数据（openers、扫描规则、……，统一存 settings.json——无需启动时加载、可在运行中变更），真实增删改；
- **config** = config.json 的只读事实（log/dataDir 等，启动期读一次），后续收缩为 settings 的一个分区。

形态讨论已收敛：**整页路由 + ⌘, 快捷键 + 新 tab 打开**。曾评估宽 Sheet 覆盖层（保上下文）与路由页（保宽度）之争，结论：表格/表单编辑需要完整视口宽度，上下文保持由「业务状态 URL 化（1024）+ 新 tab」解决。

## 心智模型（架构基准）

- **settings 页是出口层聚合，不是新 domain**：opener 分区调 opener 域的 `opener/save`/`opener/delete`，scan 规则将来调 project 域的 API。后端不出现包揽一切的 settings service，各域按「五处加法」自己长配置管理能力，settings 页只是新的前端消费方。
- **命名切割**：`config` 一词保留给 config.json 语义（Go `config` 包不动）；用户可编辑数据一律叫 settings。

## 方案

### 1. 页面骨架：`/settings` 整页路由 + 页内分区导航

```
┌──┬───────────────┬──────────────────────────────┐
│⚙ │ 设置           │  [当前分区：Opener]           │
│  │               │                              │
│  │ ▸ Opener      │  name  cmd        roles icon │
│  │   项目·扫描    │  …    …          …     …    │
│  │   基础配置     │  [+ 新增] [编辑] [删除]       │
│  │               │                              │
└──┴───────────────┴──────────────────────────────┘
   rail 48px   页内左侧分区导航     编辑区（完整宽度）
```

- 挂 Layout 下（收敛型 main），分区用 URL 记忆（`/settings?section=opener`），可刷新/直达；默认第一个分区。
- rail 底部的 Config 图标换成 Settings 入口（图标 Settings 不变即可，目标改 `/settings`）。`/config` 暂时保留，等「基础配置」分区就位后内容收编、路由重定向到 `/settings?section=basic`。
- 分区按域逐个到货：Opener（随 1016）→ 项目·扫描（project 域后续提案）→ 基础配置（收编现 `/config` 只读内容，log 等）。

### 2. 入口交互：⌘, 新 tab 打开

- 全局快捷键 **⌘,（Ctrl+,）** 打开 settings——`window.open('/settings', '_blank')`，聚焦新 tab。
- 选新 tab 而非同 tab 跳转的理由：settings 是全宽编辑界面，业务 tab 原封不动（连 1024 都不依赖），改完关 tab 即回，无「恢复现场」心智负担；个人工具多 tab 是开发者常态。
- rail 的 Settings 图标同样新 tab 打开（与 ⌘, 行为一致）；`/settings` 路由本身仍可直接访问。

### 3. 分区编辑交互模板（各分区复用）

- 列表 → 单条编辑进一层视图（页内二级态，顶部返回），不嵌 Modal；
- 保存 = `xxx/save`、删除 = `xxx/delete`（POST，规则 15；1016 已定 opener 的一对），删除前 ConfirmDialog；
- Opener 分区含 icon 选择器（lucide 图名 / .app 提取 / 上传图片，见 1016）。

## 不做的事

- 不做 monolithic settings domain / settings service；
- 不做 settings 内嵌 Modal 编辑（统一页内二级视图）；
- 不在本提案做 scan 规则编辑（project 域另立提案）。

## 实施顺序

1. 骨架：`/settings` 路由 + 分区导航 + 空 Opener 分区（列表只读）；
2. 1016 第 1-2 步合并后：Opener 分区升级为完整增删改 + icon 选择器；
3. 后续：scan 规则分区、基础配置分区收编 `/config`（届时移除 `/config` 路由）。

## 验收标准

1. ⌘, / rail 图标在新 tab 打开 `/settings`，业务 tab 不受任何影响；
2. 分区切换写 URL，刷新恢复当前分区；
3. Opener 分区（依赖 1016）完成增删改后，`cube project list` 等读路径立即反映修改（无缓存不一致）；
4. `pnpm -C web build`、`cd server && go vet ./... && go test ./...` 通过。
