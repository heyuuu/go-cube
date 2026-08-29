# server 进程管理定调：移除 start -d + daemon 设计决策

> **状态**：✅ 已实现（`-d` 移除部分，2026-08）；⏸️ `server reload` 搁置（解挂条件见文末）
> **前史**：1001 引入 `-d` 后台启动时定位为「后台启动的主入口」；本提案将其移除，并把触发本次讨论的一串 daemon 设计问题一次性定调（知识部分不限 cube 自身，适用未来同类程序）。

## 背景

`cube server stop` 在 launchctl 常驻环境下误判：旧实例 shutdown 后 KeepAlive 立刻拉新进程占回端口，`stop` 轮询 whoami 只看「端口上还有没有 cube」，把新实例当成没停掉甚至超时报错。由此引出三串讨论：stop 判定修复、`-d` 的去留、daemon 的正统设计。

## 已落地的修复：whoami 实例标识（instance）

`newSystemHandler()` 进程启动时随机生成 8 字节 hex，随 `GET /api/system/whoami` 返回；`serve.Stop` 记下发起时的实例，轮询至「探不到 cube **或** instance 换人」均判旧实例下线，并返回 `replaced` 标记端口被新实例接管（cmd 层输出「旧实例已停止（端口已被新实例接管）」）。旧版 server 无此字段读作空串，被新实例接管同样可判下线（滚动升级兼容）。`status` 表加「实例」列。

（此部分随 workspace scanRule 的 c01a510 一并入库，提交消息未提及，特此记录。）

## 决策一：移除 `start -d`（自 daemon 模式）

### 为什么删

- **无真实消费者**：prod 是 launchd plist 跑**前台** `cube server start` + KeepAlive，dev 是 air——`-d` 是第三套只有自己用的启动机制。
- **结构性 bug，修不如删**：`serve.Fork()` 用写死的 argv re-exec 自身（`exec.Command(exe, "server", "start")`），不透传 `--config`/`-D`，fork 出的子进程回落默认配置；stdio 归零后启动失败（如端口被占）完全无声。两个病根都长在「re-exec 自身」这个设计上。
- **历史裁决的延续**：1001 曾「允许多实例 + `-p`」，后来删 `-p` 杜绝孤儿实例（现状.md 3.6）；`--config + -d` 等于把孤儿从后门放回来。
- **launchd 是更好的 supervisor**：setsid、stdio、崩溃重启、KeepAlive 全由系统做，还多送生命周期管理。自 fork daemon 无论写多对都是「二号 supervisor」。

### 移除范围

`serve/fork.go` 整删、`cmd/server/start.go` 去 `--detach`、`cmd.go`/`serve/doc.go` 注释、现状.md 两处。`stop`/`status`（HTTP 进程管理）不受影响——那是 1001 真正的承重部分。`--config` 保留，定位降级为**前台实验的逃生门**（见决策三）。

## 决策二：`server reload` 搁置

`stop` 在 launchd 环境下事实上是「重启」，但把它当 reload 用语义不对——而正经 reload 的前提是「cube 自己保活、自己重新启动程序」，这与现行架构（系统保活，launchd 拉起）矛盾。instance 修复后 `stop` 已能正确识别「旧实例关闭、新实例接管」，语义缺口只剩命名层面，不值得为它引入自保活。

**解挂条件**：出现 cube 自己管理进程生命周期的形态（如未来某种 app 内嵌 helper），或用户对「stop = 重启」的表述产生实际困惑时再议。

## 决策三：多 daemon 边界（`--config + -d` 为什么不允许）

多 daemon 本身不是罪（dev 6001 / prod 6101 本就合法共存），判据是**可管理性三条件**：

1. 能被**零 flag** 发现（管理命令裸跑就能管到）；
2. 有 supervisor（launchd / air / 前台终端，谁拉起谁负责）；
3. 有独立数据目录（cube 的 `DataDir` 缺省跟随配置文件所在目录，隔离是默认行为）。

dev/prod 三条全占；`--config x -d` 起的实例三条全不占（标准 stop/status 读默认 config 看不见它、无人监督、客户端也连不上）——正是删 `-p` 时要杜绝的孤儿。切分方式按生命周期：**`--config` + 前台 = 允许**（可见、随会话死）；**后台化自定义配置 = 功能不存在即拦截**。

## 知识沉淀：daemon 设计定调（适用未来程序）

### 配置发现：零参数找到配置，flag 只是临时覆盖

daemon 应当不带任何参数找到自己的配置（身份不活在调用者的 argv 里）。四种主流模式：

| 模式 | 例子 | 说明 |
|---|---|---|
| 编译期烙默认路径 | nginx `--conf-path` | 构建时定，unit 裸跑 |
| 构建身份推导 | **cube dev/prod**（ldflags version → IsDev → 配置目录） | 一条事实源推多层 |
| supervisor 注入 env | systemd `Environment=` | 登记处写 unit，二进制零参数 |
| 运行时实例名 | pg_ctlcluster、systemd 模板单元 `%i` | 一个选择器推导整组目录，适合任意 N 实例 |

### 如果非做自 daemon 不可（正确形态，备忘）

1. **argv 原样透传** `os.Args[1:]`——绝不重新拼装（cube 的 bug 就是反面教材）；
2. **防递归用 env 哨兵**（子进程见 `_CHILD=1` 忽略 `-d`），不做 argv 字符串手术；
3. **readiness 探活**——stdio 归零后子进程启动失败无声，父进程必须探到 whoami 才报「已启动」。
4. 注入身份时注入**身份根**（可推导 config+data+port+socket 全家），不是裸 config 路径。

### 桌面 app 的 helper 进程（未来参考）

- **app 死、helper 活、重连**：daemon-first + 固定 domain socket（**bind 即单实例锁**，免 pid 文件）+ app 懒拉起（connect ECONNREFUSED 就 spawn）；重连协议带状态版本做对账（Syncthing/mpd 模式）。若需「app 不在也崩溃自愈」，仍需 launchd/SMAppService——自拉起给不了保活。
- **app 死、helper 陪葬**：**pipe 生命线**——app 握管道写端、helper 阻塞读端，app 无论怎么死（含 SIGKILL）内核关 fd → helper 读到 EOF 自杀。唯一「通用 + 即时 + 抗 SIGKILL」三全；进程组同杀、Linux PDEATHSIG、轮询 getppid 各有短板。

### 终态：不自 daemon + 外部 supervisor

现代主流（systemd 推荐 `Type=simple`、Docker/K8s 要求前台）。程序不做 daemon 化 ≠ 什么都不做，要做 **supervisor-ready** 六条：① 前台唯一形态；② stdout/stderr 日志主通道（cube 现写自己的 app.log 属妥协形态）；③ SIGTERM = graceful shutdown（cube 已具备，最要命的一条）；④ 绑定即单实例锁；⑤ 启动失败 loud（stderr + 非 0 退出码）；⑥ 退出快、退出码准。

未来若要更好的服务化体验，方向是 **`cube service install`** 类命令（帮写 plist、包 launchctl 的 start/stop/status，Docker Desktop helper / Syncthing macOS 包装同款），不是恢复 `-d`。
