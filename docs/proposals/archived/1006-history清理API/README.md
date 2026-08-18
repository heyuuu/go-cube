# history 清理 API

> **状态**：✅ 已完成

## 背景

history 当前只有写入和读取，没有任何清理 API——数据只增不减。

## 实现形态

随着 service 生命周期钩子机制落地（见下），history 清理一并实现：

- **`Service.PurgeBefore(cutoff time.Time) (int64, error)`**：删除两表（`project_select_logs` / `project_open_logs`）中 `created_at` 早于 cutoff 的记录，`Unscoped` 硬删，返回删除总行数。
- **触发**：`history.Service.OnServerStart()` 异步清一次（不阻塞 server 启动），不做定时任务——保留期仅 30 天，server 重启频率足够覆盖。清理失败降级只记 slog，不抛出。
- **保留期**：包内常量 `retentionDays = 30`（个人项目，不进配置文件）。
- CLI 短命进程不清理（不走 server 钩子）。

## 顺带落地的 service 生命周期钩子

`app/hooks.go` 定义三个可选接口（app 层类型断言分发，未实现即跳过，新增 domain 无需改 app 的钩子代码）：

- `OnAppCreated() error` — App 构造完成、db 就绪后调用。**AutoMigrate 下沉到各 service 就近处理**（history 的建表在 `history.Service.OnAppCreated`），`app.New` 不再集中写 models。
- `OnServerStart()` / `OnServerStop()` — 常驻 server 启停后台任务。project 的 git 定时刷新、history 的定时清理挂在这两个钩子上；`app.StartBackgroundJobs/StopBackgroundJobs` 改为遍历 `services` 清单分发。
