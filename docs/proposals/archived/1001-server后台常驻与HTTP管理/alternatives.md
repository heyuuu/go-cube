# 否决方案存档

讨论中考虑过、最终未采纳的方案。记录其机制、成本与否决理由，供未来重新评估时参考。

> 注：本文档随方案演变有过调整。原始版本是在「lazy 拉起 + pid 文件」方案下写的，部分结论（如「否决 `-d` flag」）在转向 HTTP 显式管理后已被推翻——这些条目标注了「🔄 已反转」。其余否决（launchd / supervisor / reload / 开机启动 / 全量分组 / 自动找端口）与具体实现无关，仍然成立。

---

## 1. launchd 常驻（开机启动 + KeepAlive 保活）

**机制**：写一份 `~/Library/LaunchAgents/com.cube.daemon.plist`，配置 `RunAtLoad=true`（开机自动起）+ `KeepAlive=true`（崩溃自动拉起）。

**成本**：中等。要写 plist 生成 + `launchctl load/unload` + `autostart enable/disable` 子命令。launchd 的语义（KeepAlive / RunAtLoad / ProgramArguments）得摸对，调试不直观。

**否决理由**：
- 显式 `cube server start -d` 已经覆盖了「按需可用」诉求，用户需要时自己起。
- 开机启动本身不是刚需——用户不一定每次开机都用 cube，强制开机常驻违背「用时存在、不用时消失」的定位。
- 引入 launchd = 引入系统集成，cube 从「工具」偏向「服务」，定位偏移大。
- 若未来真需要开机启动，可作为可选增强单独补，不阻塞主线。

---

## 2. 自 fork supervisor 保活

**机制**：自己 fork 一个 supervisor 进程盯着 server，server 崩了 supervisor 拉起。

**成本**：高。多一层进程，又得保证 supervisor 自己不死（无限回归）。

**否决理由**：
- 无限回归：supervisor 谁来保活？
- launchd 的 KeepAlive 是更好的 supervisor（系统级、免费、可靠）。
- 这个方案在任何维度都被 launchd 包含，没有独立存在的价值。

---

## 3. 🔄 已反转：`-d` flag 作为面向用户的后台启动

> **更新**：本条在「lazy 拉起」方案下被否决（理由是「后台启动是 lazy 拉起的内部能力，不暴露给用户」）。**转向 HTTP 显式管理方案后，lazy 拉起取消，`-d/--detach` 反而成了后台启动的主入口**——因为不再有「自动起」机制，用户需要一个显式的后台启动命令。本条保留作历史记录。

**原否决理由（已不适用）**：
- 后台启动曾是 lazy 拉起要调用的内部能力，不是面向用户的动词。
- 暴露 `-d` 会让用户多理解一个概念，且 lazy 拉起下没有「手动后台起一个」的真实需求。

**新方案下的立场**：
- HTTP 显式管理抛弃了 lazy 拉起，`-d` 成为必要的用户入口（类比 nginx 的 daemon 模式）。
- 用户要么临时调试（前台 `start`），要么日常常驻（`start -d`），语义清晰。

---

## 4. reload（热重载配置）

**机制**：守护进程收到 reload 信号 → 重新 `config.Load` → 重建受影响的 service（openers / scan rules 等热换）。

**成本**：高。partial reload 容易做不干净——哪些 service 能热重建、哪些不能（db 连接 / paths / 正在处理的请求）要逐一厘清；http.Server 运行期换 mux 不 trivial。

**否决理由**：
- cube 是个人工具，没有「不能中断的长连接」（无 WebSocket 用户），restart 的代价 ≈ 0。
- partial reload 的工程复杂度和 bug 面积与收益不成比例。
- 统一用 stop + start 覆盖「配置变了 / 二进制变了」所有场景，简单可靠。

---

## 5. 开机启动（独立于 launchd 全案）

**机制**：单独做开机启动，不叠加 KeepAlive 保活。

**成本**：仍需 launchd（macOS 上开机启动的事实标准），所以和方案 1 的实现重叠，只是 plist 少一行 KeepAlive。

**否决理由**：
- 第一次用 cube 时显式 `start` 即可，省掉整套自启逻辑。
- 开机启动单独做意义不大——如果接受 launchd 的复杂度，不如直接开 KeepAlive 顺带拿保活；如果不接受 launchd，那连开机启动也别做。没有「只做开机启动不做保活」的中间态价值。

---

## 6. 全量命令按领域分组

**机制**：把现有所有扁平命令按领域重新分组：`cube project list/info/open/...`、`cube git push/...`、`cube open ...`。

**成本**：大。改动面广，且破坏现有命令习惯（`cube projects` → `cube project list` 等）。

**否决理由**：
- 本次分组 server 的动机是解决 `status` 等动词的歧义，其余命令（project / git / open）当前没有这个痛点。
- 全量重构是独立的技术债清理，不该夹在 server 需求里做。
- **记为技术债**：未来若命令数量继续增长导致 `cube --help` 难读，再单独立项全量分组。

---

## 7. 端口冲突时自动找空闲端口

**机制**：默认 8080 被占时，自动扫描空闲端口，把实际端口告知调用方。

**成本**：中等。端口扫描 + 调用方动态拿端口。

**否决理由**：
- 允许多实例后，用户用 `-p` 显式指定端口即可，不同端口就是不同 server。
- 自动找端口会让「server 监听哪个端口」变得不可预测，调试和文档都更麻烦。
- 真冲突（同端口 bind）时 `ListenAndServe` 直接报错退出，用户改 `-p` 即可，清晰可追溯。
