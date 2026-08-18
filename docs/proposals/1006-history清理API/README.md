# history 清理 API

> **状态**：⏸️ 暂不做（等触发条件再评估）

## 背景

history 当前只有写入和读取，没有任何清理 API——数据只增不减。

## 为什么暂不做

1. **写入面极窄**：当前 history 写入只在 alfred 流程（`cmd/alfred/project_search.go` 写 select、`cmd/alfred/project_open.go` 写 open），CLI 的 `open` / web 的 `/api/project/open` 路径都没接 history。
2. **写入量极小**：单用户、单 alfred 交互，估算一年 ~7300 行/表，sqlite 十年都不会有压力。
3. **策略无数据支撑**：清理策略（按时间 vs 按条数、阈值多少、触发时机）现在没有实际数据支撑，提前定是凭空猜。
4. **频率信号价值**：history 核心用途是「最近用排序」（`Order("max(id) desc")`），长期记录有频率信号价值，过早清理反而损信号。

## 触发条件

当 history 写入面铺开到 CLI/web 全入口后，重新评估数据增长。

## 未来形态（届时参考）

- 倾向按时间清理：`PurgeBefore(90天前)`。
- 触发点：挂 `app.New()` 启动时跑一次（低频安全）。
- 参考 cube-next `server/history/store.go` 的 `PurgeBefore` 实现。
