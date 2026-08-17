# web 层测试补全

> **状态**：✅ 已实现（260817 归档，newTestEnv 基建 + 全部 handler 用例，见 `server/web/*_test.go`）
> **来源**：cube-next 吸收讨论（借鉴测试范式，非直接搬代码）

## 背景

cube 的 `server/web/` 有 6 个生产 .go 文件（server / api / api_project / api_opener / api_config / static），**零测试**。web 层是 huma 路由 + DTO 转换 + ApiOutput envelope，回归风险高（历史上有大改，如 commit 313b7d8 一次砍掉 api_config 168 行，无测试兜底）。

## 目标

借鉴 cube-next 的 web 测试范式，从零搭建 cube 的 web 测试基建，给每个 handler 补测试。

## 可借鉴的范式（来自 cube-next）

- **newTestServer**：`httptest.NewServer(s.Handler())` 拉起真实 server，打真实 HTTP 请求验证整条链路（路由 / DTO 转换 / envelope / nil 序列化等）。
- **newSvcWithTree**：建临时目录树伪造 .git 项目（复用 cube 的 `internal/testfixture`），构造 project.Service 喂给 handler。
- **fake 模式**：web 层依赖注入测试（fake config provider 等）。

详见 cube-next `server/web/server_test.go` / `projects_test.go` / `config_test.go`（路径 `/Users/heyu/Code/heyuuu/cube-next/server/web/`）。

## 为什么不直接搬

cube 和 cube-next 的 web handler 注册方式不同：
- cube 用 `Handler` 接口 + `NewServer(handlers...)` + `apiRegister` 统一入口。
- cube-next 的测试基建依赖它自己的 handler 构造方式。

所以是「借鉴范式，从零写」，不是复制文件。需要理解 cube 每个 handler 的业务语义才能写好测试。

## 工作量评估

- 搭建测试基建（newTestServer / fixture 构造）：中等。
- 给 6 个 handler 逐个补测试：较大，需理解每个 handler 的业务语义。
- 建议作为独立的改进项排期，不夹在其他任务里零碎做。

## 触发时机建议

- 下次大改 web 层时（如前端栈迁移会带动 API 调整），顺手补测试。
- 或独立排一个「web 测试补全」的工作单元。
