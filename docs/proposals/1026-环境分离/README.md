# 环境分离：dev / prod 二进制身份 + 配置目录分流

> **状态**：🚀 提案（2026-08-25，方向已与 owner 对齐，**优先级高**——落地前 dev/prod 互相干扰持续存在）
>
> **关联**：[1016-opener改造](../1016-opener改造/README.md)、[1025-settings配置页](../1025-settings配置页/README.md)（settings.json 的切割规则由本提案的边界定义衍生）。

## 背景与动机

一边开发 cube（air 起的 dev server、跑测试）一边使用 cube（常驻 server），目前共用 `~/.config/cube/` 一个配置目录和重叠的端口段，产生过一连串实际问题：

1. **端口干扰**：`cube server` 默认端口 8080 容易和测试端口撞；dev（air，6001）和正式实例没有明确分段。
2. **默认端口失配**：常驻 server 用 `-p 6101` 起，而 `cube md .` 等依赖 server 的命令按默认 8080 找，直接失败——允许多端口起多实例被证明是个错误决定，依赖方无从知道「当前常驻的是哪个端口」。
3. **孤儿实例**：`-p` 传了自定义端口后忘记，实例在后台被遗忘；server.json 实例注册表方案已放弃（复杂度不值）。

已手工铺好的现状（本提案将其制度化）：

- launchd LaunchAgent `~/Library/LaunchAgents/com.heyuuu.cube.plist` 已运行，`-p 6101`，KeepAlive + RunAtLoad，日志 `~/.config/cube/launchd.log`；
- 端口段约定：**dev 6001–6009，prod 6101–6109**（已验证机器上无冲突）。

## 已收敛的设计决策

### 1. env 由 version 推导，二进制自识别

不引入独立的 `Env` 注入——`version.IsDev()` 以「version 是否为默认值 `dev`」判定环境：ldflags 注入了正式 version 的二进制（`make build` / `make install`，产物本就带正式 tag/commit）即 prod；无注入的（源码直跑、air、run.sh）即 dev。env 跟着二进制走，单条命令即可自证身份，Makefile 无需任何改动。

### 2. 默认配置目录按 env 分流

- dev（默认）→ `~/.config/cube-dev/`
- prod → `~/.config/cube/`
- `-c` 全局 flag 保持最高优先级，覆盖一切。

`make build` 与 `make install` 产物均为 prod（都注入了 version）；air / run.sh 不动——无注入即 dev，自动落 dev 目录。

### 3. env 可见（对付孤儿/错环境的直接手段）

- `cube version` 输出 `VersionInfo()`：dev 显示 `dev`，正式显示 `tag (commit time)`——version 本身即环境标识（env 由它推导），无需单独的 env 字段；
- `/api/system/whoami` 返回 version，同理可判环境；
- server 启动时打印 `cube version: ...`。

### 4. 端口单一事实源 = config.json 的 `server.port`

- **不再允许多端口多实例**作为常规形态；每个环境的常驻端口写死在自己的 config.json：dev `6001`、prod `6101`。
- `DefaultPort` 8080 → 6101；`-p` 降级为开发期逃生口（临时换端口调试），不承担常规职责。
- `cube md`、`cube server start/stop/status` 等依赖 server 的命令一律读 config 端口（修复 `cube md .` 找 8080 失败）。
- 落地后清理：`.air.toml` 的 `args_bin` 去掉 `-p 6001`；launchd plist 去掉 `-p 6101`（**注意 kickstart -k 不会重读 plist**，需 `bootout` + `bootstrap` 重新注册）。

### 5. 跨环境数据策略（保持简单，不做工具）

| 数据 | 策略 |
|---|---|
| config.json | 人工 diff 合并（文本，低频） |
| settings.json（1016 起） | 人工 diff 合并（这正是 opener 落库方案改为 settings.json 的原因：sqlite 无法人工合并） |
| history sqlite（data.db） | **不迁移**，各环境独立积累——使用痕迹价值低，重建成本低 |

不做 sync/export 命令，不做 schema 比对，不做 migration。

## 实施顺序

1. `version` 包收口为未导出 var + getter（`IsDev()`/`Version()`/`VersionInfo()` 等），ldflags 注入小写字段名；✅ 已完成
2. config 默认目录按 env 分流（dev `~/.config/cube-dev/`，prod `~/.config/cube/`；`-c` 不变）；✅ 已完成
3. env 可见性：`cube version` / whoami / 启动日志；✅ 已完成
4. `server.port` 成为端口事实源：DefaultPort 6101、md 及 server 子命令读 config 端口；
5. 环境清理：air args_bin 去 `-p`、launchd plist 去 `-p` 并 bootout+bootstrap、给 `~/.config/cube-dev/` 初始化一份 config（含 port 6001）、`~/.config/cube/config.json` 补 port 6101；
6. 更新 `docs/spec/现状.md`（配置目录、端口、env 章节）。

## 验收标准

1. 源码直跑（air / run.sh / go run）产物为 dev：配置目录 `~/.config/cube-dev/`；`make build` / `make install` 产物为 prod：`~/.config/cube/`；`-c` 可覆盖。
2. `cube version` 与 whoami 能看出 env（version 为 `dev` 或正式 tag）；server 启动打印 version。
3. prod 常驻（launchd）在 6101，dev（air）在 6001，互不可见对方数据；`cube md .` 在 prod 环境下直接可用（不传端口）。
4. `cd server && goimports -w . && go vet ./... && go test ./...` 通过。
