# 否决方案存档

讨论中考虑过、最终未采纳的方案。记录其机制、成本与否决理由，供未来重新评估时参考。

---

## 1. launchd 常驻（开机启动 + KeepAlive 保活）

**机制**：写一份 `~/Library/LaunchAgents/com.cube.daemon.plist`，配置 `RunAtLoad=true`（开机自动起）+ `KeepAlive=true`（崩溃自动拉起）。一份 plist 同时覆盖开机启动和崩溃恢复。

**成本**：中等。要写 plist 生成 + `launchctl load/unload` + `autostart enable/disable` 子命令。launchd 的语义（KeepAlive / RunAtLoad / ProgramArguments）得摸对，调试不直观。

**否决理由**：
- lazy 拉起已经覆盖了"崩溃恢复"（下次命令自动起）和"按需可用"（第一次用时起）这两个诉求。
- 开机启动本身不是刚需——用户不一定每次开机都用 cube，强制开机常驻违背"用时存在、不用时消失"的定位。
- 引入 launchd = 引入系统集成，cube 从"工具"偏向"服务"，定位偏移大。
- 若未来真需要开机启动，可作为本提案的**可选增强**单独补，不阻塞主线。

---

## 2. 自 fork supervisor 保活

**机制**：自己 fork 一个 supervisor 进程盯着 server，server 崩了 supervisor 拉起。

**成本**：高。多一层进程，又得保证 supervisor 自己不死（无限回归）。要么 supervisor 再套 supervisor，要么依赖系统进程管理器——后者就是方案 1 的 launchd。

**否决理由**：
- 无限回归：supervisor 谁来保活？
- launchd 的 KeepAlive 是更好的 supervisor（系统级、免费、可靠），而 lazy 拉起让保活本身不必要。
- 这个方案在任何维度都被 lazy 拉起或 launchd 包含，没有独立存在的价值。

---

## 3. `start -d` 作为面向用户的独立命令

**机制**：暴露 `cube server start -d`，用户可手动后台起一个 server，脱离终端但不登记开机启动。

**成本**：中等（re-exec daemon 实现成本不变，只是多暴露一个 flag）。

**否决理由**：
- 后台启动是 lazy 拉起机制要调用的**内部能力**，不是面向用户的动词。
- 暴露 `-d` 会让用户多理解一个概念（"前台 start 和后台 start -d 有什么区别"），且实际场景几乎不存在——用户要么临时调试（用前台 start），要么日常用（lazy 拉起自动管），没有"我现在就想手动后台起一个备用"的真实需求。
- 保持"前台 `start` / 后台交给 lazy 拉起"两种清晰姿势，代码也只维护一套后台逻辑（被 lazy 拉起调用）。

---

## 4. reload（热重载配置）

**机制**：守护进程收到 reload 信号 → 重新 `config.Load` → 重建受影响的 service（openers / scan rules 等热换）。

**成本**：高。partial reload 容易做不干净——哪些 service 能热重建、哪些不能（db 连接 / paths / 正在处理的请求）要逐一厘清；http.Server 运行期换 mux 不 trivial。

**否决理由**：
- cube 是个人工具，没有"不能中断的长连接"（无 WebSocket 用户），restart 的代价 ≈ 0。
- partial reload 的工程复杂度和 bug 面积与收益不成比例。
- 统一用 restart（或 stop + start）覆盖"配置变了 / 二进制变了"所有场景，简单可靠。

---

## 5. 开机启动（独立于 launchd 全案）

**机制**：单独做开机启动，不叠加 KeepAlive 保活。

**成本**：仍需 launchd（macOS 上开机启动的事实标准），所以和方案 1 的实现重叠，只是 plist 少一行 KeepAlive。

**否决理由**：
- 第一次用 cube 时 lazy 拉起即可，省掉整套自启逻辑。
- 开机启动单独做意义不大——如果接受 launchd 的复杂度，不如直接开 KeepAlive 顺带拿保活；如果不接受 launchd，那连开机启动也别做。没有"只做开机启动不做保活"的中间态价值。

---

## 6. 全量命令按领域分组

**机制**：把现有所有扁平命令按领域重新分组：`cube project list/info/open/...`、`cube git push/...`、`cube open ...`。

**成本**：大。改动面广，且破坏现有命令习惯（`cube projects` → `cube project list` 等）。

**否决理由**：
- 本次分组 server 的动机是解决 `status` 等动词的歧义，其余命令（project / git / open）当前没有这个痛点。
- 全量重构是独立的技术债清理，不该夹在 server 提案里做。
- **记为技术债**：未来若命令数量继续增长导致 `cube --help` 难读，再单独立项全量分组。

---

## 7. 端口冲突时自动找空闲端口

**机制**：默认 8080 被占时，自动扫描空闲端口，把实际端口写入 pid 文件供调用方发现。

**成本**：中等。端口扫描 + 调用方读 pid 文件拿端口（不能写死 8080）。

**否决理由**：
- lazy 拉起场景下，server 由 cube 自己起，正常不会和别人抢端口（除非用户同时跑了别的 8080 服务）。
- 真冲突时报错提示用户用 `-p` 指定端口即可，简单直接。
- 自动找端口会让"server 监听哪个端口"变得不可预测，调试和文档都更麻烦。
- 作为后续增强保留可能性，初版不做。
