package gitcache

// 本文件是 git 缓存的「调度层」，与 cache.go（数据层）相对。
//
// 调度问题：CLI 模式下，每次读命令（project list/info）都触发一次采集是不现实的，
// 但又不能让前台命令阻塞等待采集。解法：
//
//   - 前台命令（父进程）：返回前调用 TryAsyncRefresh，仅做「是否需要刷新」的判断；
//     通过则 fork 一个子进程做真正的采集，父进程 cmd.Start() 后立即返回，绝不阻塞。
//   - 后台子进程：被命令 `cube project refresh-git-cache` 触发，做真正的采集。
//
// 防雪崩与防并发（两层保护）：
//  1. 父进程侧：读 git.lock 里的 lastRefreshAt 做 TTL 判断，TTL 内直接不 fork。
//  2. 子进程侧：flock(git.lock, LOCK_EX|LOCK_NB)，拿不到锁立即退出（说明已有进程在跑）。
//     两层叠加，即便高频 CLI 调用（如 alfred workflow 每次按键）也不会 fork 雪崩。
//
// git.lock 的双重职责（刻意合并，减少文件数与状态源）：
//   - 作为 flock 载体：跨进程互斥。
//   - 作为 TTL 状态载体：文件内容是 {"lastRefreshAt": "..."}。
//   子进程抢到锁的第一件事就是原子写 lastRefreshAt=now，立即占住 TTL 窗口；
//   即便采集要 20s，并发父进程读到的也是「刚刚刷新过」的状态。
//
// 关于并发窗口的说明：
//   advisory flock 不影响 os.ReadFile（读永远成功），所以「采集进行中」这个窗口里，
//   父进程读到的 lastRefreshAt 已经是新值（子进程拿锁后立即写），不会被误判。
//   唯一的窗口是「子进程刚 fork 但还没 flock」的极短时间（毫秒级），此时若有别的
//   父进程通过 TTL 检查 fork 出子进程，新子进程会被 flock 兜底立即退出 —— 安全。
//
// 为什么不用 TTL 过期读：
//   - 读命令永远命中当前缓存（哪怕 stale），保证前台响应极快；
//   - TTL 仅用于判断「是否触发一次新的后台采集」，不参与读路径。

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// git.lock 文件名：既是 flock 锁载体，又承载 lastRefreshAt（TTL 判断依据）。
const lockFileName = "git.lock"

// lockState 是 git.lock 文件内容的序列化结构。
type lockState struct {
	LastRefreshAt time.Time `json:"lastRefreshAt"`
}

// TryAsyncRefresh 在父进程里触发一次异步刷新。
//
// 决策顺序（短路）：
//  1. 读 git.lock 的 lastRefreshAt，TTL 未到 → 直接返回。
//  2. fork 自身二进制为子进程，参数 `project refresh-git-cache`。
//  3. cmd.Start() 后立即返回（不 Wait）。
//
// 不在父进程做 flock 探测：fork 后到子进程真正拿锁之间，可能多个父进程都通过
// TTL 检查，靠子进程的 flock 兜底去重即可。这种「无效 fork」代价极小（子进程
// flock 失败立即退出，毫秒级），换来父进程的简单性，值得。
func TryAsyncRefresh(cacheDir string, ttl time.Duration) {
	if ttl <= 0 {
		ttl = defaultTTL
	}

	// 1. TTL 判断（纯只读，不持锁）
	if !shouldRefresh(cacheDir, ttl) {
		slog.Debug("git cache: skip async refresh (within TTL)")
		return
	}

	// 2. fork 子进程
	exe, err := os.Executable()
	if err != nil {
		slog.Warn("git cache: resolve self executable failed", "err", err)
		return
	}

	// 3. 子进程完全脱钩：新会话（setsid）+ 丢弃 stdio。
	//   - Setsid: true  让子进程脱离父进程的会话/控制终端；父进程退出时不会向
	//     子进程传播 SIGHUP，也不会因共享终端 fd 把子进程拖死。
	//   - Stdin/Stdout/Stderr = nil：exec.Command 默认接 /dev/null，子进程不继承
	//     父进程任何 fd，彻底解耦生命周期。
	//   - 不调 cmd.Wait：父进程立即返回；子进程退出后由 init（PID 1）回收，
	//     不会变僵尸（setsid 后已 reparent 到 init）。
	cmd := exec.Command(exe, "project", "refresh-git-cache")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		slog.Warn("git cache: fork refresh subprocess failed", "err", err)
		return
	}

	// Release 通知 Go runtime 放弃对该 pid 的跟踪；真正回收由 init 完成。
	_ = cmd.Process.Release()

	slog.Debug("git cache: async refresh forked", "pid", cmd.Process.Pid)
}

// RefreshSync 在子进程里执行真正的采集。
//
// 流程：
//  1. flock(LOCK_EX|LOCK_NB)，拿不到立即退出（防并发刷新 + 防雪崩）。
//  2. 拿到锁后原子写 lastRefreshAt=now 到 git.lock（占住 TTL 窗口，即便采集要很久）。
//  3. Load 缓存 → Refresh 采集 → 落盘。
//  4. 进程退出，flock 自动释放。
//
// paths 由调用方（cmd/cache）传入项目绝对路径列表。
func RefreshSync(cacheDir string, paths []string) error {
	// 确保目录存在（父进程的 TryAsyncRefresh 不一定先建目录）
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("创建 cache 目录失败: %w", err)
	}

	// 1. flock 非阻塞抢锁
	lockPath := filepath.Join(cacheDir, lockFileName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("打开 lock 文件失败: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		slog.Debug("git cache: another refresh in progress, exit")
		return nil // 已有进程在跑，正常退出
	}

	// 2. 拿到锁，立即原子写 lastRefreshAt（即便采集还没开始，先占住 TTL 窗口）。
	//    写到同一个 git.lock 文件，让父进程的 shouldRefresh 立即看到新值。
	if err := writeLockState(lockPath, lockState{LastRefreshAt: time.Now()}); err != nil {
		slog.Warn("git cache: write lock state failed", "err", err)
	}

	// 3. 采集
	cache, err := Load(cacheDir)
	if err != nil {
		return fmt.Errorf("加载 cache 失败: %w", err)
	}
	if err := cache.Refresh(paths); err != nil {
		return fmt.Errorf("刷新 cache 失败: %w", err)
	}

	slog.Debug("git cache: refresh done", "projects", len(paths))
	return nil
}

// shouldRefresh 读 git.lock 的 lastRefreshAt 判断是否已经过了 TTL 窗口。
//
// 判定规则（均不需要持锁，纯只读）：
//   - 文件不存在 / 读失败 / 解析失败 → true（需要刷新，冷启动或状态丢失）
//   - lastRefreshAt 距今 >= ttl      → true（TTL 已过）
//   - 否则                           → false（TTL 内，跳过）
//
// 注意：advisory flock 不影响 os.ReadFile，所以即便子进程正在持锁采集，
// 父进程也能读到 lastRefreshAt（且此时已是新值，正确跳过）。
func shouldRefresh(cacheDir string, ttl time.Duration) bool {
	path := filepath.Join(cacheDir, lockFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return true // 文件不存在：需要刷新（冷启动）
	}
	var st lockState
	if err := json.Unmarshal(data, &st); err != nil {
		return true // 损坏：需要刷新
	}
	return time.Since(st.LastRefreshAt) >= ttl
}

// writeLockState 原子写入 git.lock 的 lastRefreshAt（tmp + rename）。
//
// 为什么用 tmp+rename 而非直接覆盖写：
//
//	直接 Write 可能让并发读进程读到「写了一半的 JSON」。rename 是原子操作，
//	读进程要么看到旧文件、要么看到新文件，不会半截。
//
// 调用方已持 flock（RefreshSync 内），此处不再加锁；原子写只保证「读不读半截」。
func writeLockState(lockPath string, st lockState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := lockPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, lockPath)
}

// defaultTTL 默认 TTL：1 分钟。
// 用于 TryAsyncRefresh 判断「距离上次刷新是否够久」。
// 太短会让高频 CLI 调用频繁 fork；太长会让缓存陈旧。
const defaultTTL = time.Minute
