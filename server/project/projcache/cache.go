// Package projcache 提供项目状态快照的本地缓存（git 信息 + 打开目标元数据）。
//
// 对几十个 git 项目的全量采集是 IO 密集型操作，每次都跑会明显卡顿。
// 本包把采集结果缓存到 ~/.config/cube/cache/git.json：
//   - CLI（短命进程）启动时 Load 一次快照到内存，只读不刷新；
//   - 常驻 server 进程内 goroutine 定时 Refresh 采集，写内存 + flush git.json。
//
// git.json 的角色是「跨重启持久化缓存」：server 重启后秒恢复，CLI 读最近一次落盘。
// 原名 gitcache，1030 起快照含 workspace 信息（非纯 git 信息）更名 projcache。
package projcache

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cube/project/workspace"
	"cube/util/git"
	"cube/util/store"
)

// 当前缓存文件格式版本；结构变更时递增，用于后续做兼容迁移。
// v2：worktree 归并为项目打开目标（1032）——移除 WorktreeMain（worktree 不再是独立项目），
// 新增 Worktrees（主项目附带枚举 linked worktree）。
// v3：包更名 projcache + workspace 字段（1030）——Entry/WorktreeInfo 各增 Workspaces。
const cacheVersion = 3

// 缓存文件名（位于缓存目录 dir 下）。
const cacheFileName = "git.json"

// WorktreeInfo 主项目快照里的单个 linked worktree（提案 1032：worktree 归并为项目打开目标，
// 不再是独立项目，枚举结果挂在主项目条目下）。
type WorktreeInfo struct {
	Path       string                `json:"path"`       // worktree 绝对路径（git 输出经符号链接规范化）
	Branch     string                `json:"branch"`     // 检出分支短名；detached 为空
	Detached   bool                  `json:"detached"`   // HEAD 游离（展示名回退目录名的信号）
	Workspaces []workspace.Workspace `json:"workspaces"` // 该 worktree 根的 workspace 成员（1030，Path 相对 worktree 根）
}

// Entry 单个项目的 git 信息快照。
type Entry struct {
	RepoUrl       string                `json:"repoUrl"`       // origin remote URL
	CurrentBranch string                `json:"currentBranch"` // HEAD 指向分支短名，detached 为空
	DefaultBranch string                `json:"defaultBranch"` // 默认主分支名（master/main/...）
	Branches      []string              `json:"branches"`      // 本地+远程分支短名列表
	Ahead         int                   `json:"ahead"`         // 默认分支相对 origin 的领先 commit 数
	Behind        int                   `json:"behind"`        // 落后的 commit 数
	Dirty         bool                  `json:"dirty"`         // 工作区是否有改动
	Worktrees     []WorktreeInfo        `json:"worktrees"`     // 主项目的 linked worktree 列表（无则空数组；主目录本身不含在内）
	Workspaces    []workspace.Workspace `json:"workspaces"`    // 主项目根的 workspace 成员（1030，Path 相对项目根；无则空数组）
	CollectedAt   time.Time             `json:"collectedAt"`   // 本次采集时间
}

// cacheFile 缓存文件的磁盘序列化结构。
// key = 项目绝对路径。
type cacheFile struct {
	Version   int               `json:"version"`
	UpdatedAt time.Time         `json:"updatedAt"`
	Entries   map[string]*Entry `json:"entries"`
}

// Cache 内存态缓存。读多写少，用 RWMutex 保护 entries map。
//
// 并发模型（单写者）：
//   - 常驻 server 是 git.json 的唯一写方，CLI 只读不写，无跨进程并发写问题。
//   - 进程内：RWMutex 保护 map 读写；Refresh 每次整表重建（全新 map 覆盖 c.entries）。
//   - 落盘靠 Save() 的原子写（tmp + rename）保证，无需 flock。
type Cache struct {
	dir       string // 缓存目录（~/.config/cube/cache/）
	mu        sync.RWMutex
	entries   map[string]*Entry // key = 项目绝对路径
	updatedAt time.Time         // 最近一次采集落盘时间（缓存整体刷新时间）
}

// UpdatedAt 返回缓存最近一次落盘时间（整体刷新时间，区别于单项目的 CollectedAt）。
func (c *Cache) UpdatedAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.updatedAt
}

// Load 从 dir 加载缓存。
// 行为约定（降级优先，绝不因缓存问题阻塞 CLI）：
//   - dir 不存在：创建并返回空缓存。
//   - git.json 不存在：返回空缓存。
//   - 解析失败：备份损坏文件到 git.json.corrupt-{ts}，返回空缓存。
func Load(dir string) (*Cache, error) {
	c := &Cache{dir: dir, entries: make(map[string]*Entry)}

	// 确保目录存在
	if err := os.MkdirAll(dir, 0755); err != nil {
		return c, fmt.Errorf("创建 cache 目录失败: %w", err)
	}

	// 文件不存在：空缓存
	path := c.path()
	data, err := os.ReadFile(path)
	if err != nil {
		return c, nil // 文件不存在或其他读错误，降级返回空缓存
	}

	// 解析失败：备份损坏文件，返回空缓存（降级优先）
	// 版本不符：结构已变，老数据按新结构 lenient unmarshal 会得到缺字段全零值的
	// 部分数据（比没有数据更糟），直接当文件不存在丢弃，靠下轮采集重建
	var file cacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		slog.Warn("git 缓存文件损坏，备份后从空重建", "path", path, "err", err)
		backupCorrupt(path, data)
		return c, nil
	}
	if file.Version != cacheVersion {
		slog.Warn("git 缓存版本不符，丢弃旧文件从空重建", "path", path, "version", file.Version)
		return c, nil
	}
	c.mu.Lock()
	if file.Entries != nil {
		c.entries = file.Entries
	}
	c.updatedAt = file.UpdatedAt
	c.mu.Unlock()

	return c, nil
}

// path 返回缓存文件完整路径（包内自用）。
func (c *Cache) path() string { return filepath.Join(c.dir, cacheFileName) }

// Get 读取单个项目的缓存条目；未命中返回 (nil, false)。
func (c *Cache) Get(path string) (*Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[path]
	return e, ok
}

// Size 返回缓存条目数（最近一次采集成功的项目数）。
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Save 原子写入 git.json（store.WriteFileAtomic 的 tmp + rename）。
func (c *Cache) Save() error {
	c.mu.Lock()
	now := time.Now()
	c.updatedAt = now
	file := cacheFile{
		Version:   cacheVersion,
		UpdatedAt: now,
		Entries:   c.entries,
	}
	c.mu.Unlock()

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 cache 失败: %w", err)
	}

	if err := store.WriteFileAtomic(c.path(), data, 0644); err != nil {
		return fmt.Errorf("写入 cache 文件失败: %w", err)
	}
	return nil
}

// Refresh 用给定的项目路径列表并发采集 git 信息，整表重建后写回内存 + 落盘。
//
// 整表重建语义：以本次采集结果为准，旧 entries 被完全覆盖——
//   - 采集成功的项目写入新 entry；
//   - 采集失败（collectEntry 返回 error）或已不在 paths 中的项目，其 entry 不进入新表，自然丢弃。
//
// 刻意不保留失败项目的旧 entry：避免某个项目长期异常、旧快照却一直存在而不被发现。
//
// 入参用 []string（项目绝对路径）而非 []*project.Project，刻意解耦对 project 包的依赖，
// 避免 project → projcache → project 的循环 import。
//
// 并发模型：固定 defaultWorkers 个 worker 从 tasks channel 抢活——worker 数即并发上限
// （采集是子进程 + IO 密集型，过高并发会与系统其他 IO 抢资源），无需额外信号量。
// 单项目采集失败（collectEntry 返回 error）会 Warn 记录后跳过。
func (c *Cache) Refresh(paths []string) error {
	start := time.Now()
	entries := collectEntries(paths)
	c.mu.Lock()
	c.entries = entries
	c.mu.Unlock()
	if err := c.Save(); err != nil {
		return err
	}
	// 采集汇总日志：排查「缓存为何没更新 / 采集耗时异常」时直接查这一条。
	// 失败数 = 项目数 - 成功数（失败项目另有逐条 Warn 日志）。
	// 耗时用 String() 而非 time.Duration 原值：JSON handler 会把 Duration 序列化成纳秒整数，不可读。
	slog.Info("git 缓存采集完成",
		"项目数", len(paths),
		"成功", len(entries),
		"失败", len(paths)-len(entries),
		"开始", start.Format("15:04:05.000"),
		"结束", time.Now().Format("15:04:05.000"),
		"耗时", time.Since(start).Round(time.Millisecond).String(),
	)
	return nil
}

// RefreshOne 定向刷新单个项目的快照条目并落盘（提案 1031：worktree/分支写操作
// 成功后即时可见，不等 TTL 整表重建）。与 Refresh 的整表重建语义不同：只覆盖
// 该路径的 entry，其余项目保持原样。采集失败不落盘、保留旧 entry，返回错误
// 由调用方决定是否上抛（写操作本身已成功，通常 Warn 后依赖 TTL 自愈即可）。
func (c *Cache) RefreshOne(path string) error {
	entry, err := collectEntry(path)
	if err != nil {
		return fmt.Errorf("定向刷新 git 缓存失败: %w", err)
	}
	c.mu.Lock()
	c.entries[path] = entry
	c.mu.Unlock()
	return c.Save()
}

// defaultWorkers 默认并发数。
// 偏保守：采集是子进程 + IO 密集型，过高并发会与系统其他 IO 抢资源。
const defaultWorkers = 8

func collectEntries(paths []string) map[string]*Entry {
	if len(paths) == 0 {
		return make(map[string]*Entry)
	}

	type result struct {
		path  string
		entry *Entry
	}
	tasks := make(chan string)
	results := make(chan result, len(paths))

	// 固定数量 worker，从 tasks 取任务执行
	var wg sync.WaitGroup
	for range defaultWorkers {
		wg.Go(func() {
			for path := range tasks {
				entry, err := collectEntry(path)
				if err != nil {
					slog.Warn("采集 git entry 失败", "path", path, "err", err)
					continue // 采集失败跳过，该项目不进入本次结果（整表重建，不保留旧 entry）
				}
				results <- result{path: path, entry: entry}
			}
		})
	}

	// 派发任务（独立 goroutine，避免 tasks 写入与 worker 读取互锁）
	go func() {
		for _, p := range paths {
			tasks <- p
		}
		close(tasks)
	}()

	// worker 全部退出 = 所有结果已写入 results，可安全关闭
	go func() {
		wg.Wait()
		close(results)
	}()

	// 合并本次采集结果（只含采集成功的项目）
	entries := make(map[string]*Entry, len(paths))
	for r := range results {
		if r.entry != nil {
			entries[r.path] = r.entry
		}
	}
	return entries
}

// collectEntry 采集单个项目的 git 信息。
// 依赖 util/git 包的错误约定：业务空值场景返回零值+nil；但 git 子进程执行失败
// （如指向已删主仓库的 worktree 残骸）会返回 nil+err，必须上抛跳过，否则解引用 nil panic。
func collectEntry(path string) (*Entry, error) {
	repoUrl, _ := git.RemoteUrl(path)
	refs, err := git.Refs(path)
	if err != nil {
		return nil, err
	}
	branches := make([]string, 0, len(refs.Locals))
	for _, ref := range refs.Locals {
		branches = append(branches, ref.ShortName)
	}
	currentBranch := git.CurrentBranch(path)
	defaultBranch, _ := git.DefaultBranch(path)
	// ahead/behind 用仓库的默认分支（master/main/...）做本地 vs 远程比较——
	// 刻意不用 LoadRepoStatus 的 branch.ab：那是「当前检出分支 vs 其 upstream」，
	// 在 feature 分支上时与列表页要的口径不同
	var ahead, behind int
	if defaultBranch != "" {
		ahead, behind, _ = git.AheadBehindRemote(path, defaultBranch, "origin", defaultBranch)
	}
	st, err := git.LoadRepoStatus(path)
	if err != nil {
		return nil, err
	}
	return &Entry{
		RepoUrl:       repoUrl,
		CurrentBranch: currentBranch,
		DefaultBranch: defaultBranch,
		Branches:      branches,
		Ahead:         ahead,
		Behind:        behind,
		Dirty:         st.Dirty,
		Worktrees:     collectWorktrees(path),
		Workspaces:    workspace.Resolve(path),
		CollectedAt:   time.Now(),
	}, nil
}

// collectWorktrees 枚举主项目的 linked worktree 列表（1032：worktree 归并为项目打开目标）。
// WorktreeList 输出主目录在前，从第 2 项起才是 worktree；枚举失败降级为空列表
// （与非仓库目录 collectEntry 返回零值 entry 的降级基调一致，不让 worktree 问题拖垮整个条目）。
func collectWorktrees(path string) []WorktreeInfo {
	list, err := git.WorktreeList(path)
	if err != nil {
		slog.Debug("枚举 worktree 失败，降级为空列表", "path", path, "err", err)
		return []WorktreeInfo{}
	}
	worktrees := make([]WorktreeInfo, 0, len(list))
	for _, wt := range list[1:] { // 第 1 项是主目录自身
		// 目录已删但 git 元数据未 prune 的 worktree 仍会被列出，按存在性过滤，
		// 避免失联路径进快照成为打不开的幽灵目标
		if _, err := os.Stat(wt.Path); err != nil {
			continue
		}
		worktrees = append(worktrees, WorktreeInfo{Path: wt.Path, Branch: wt.Branch, Detached: wt.Detached, Workspaces: workspace.Resolve(wt.Path)})
	}
	return worktrees
}

// backupCorrupt 把损坏文件备份到 git.json.corrupt-{timestamp}，便于事后排查。
// 备份失败不影响主流程（最多 Warn 一次）。
func backupCorrupt(path string, data []byte) {
	bk := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
	if err := os.WriteFile(bk, data, 0644); err != nil {
		slog.Warn("备份损坏的 cache 文件失败", "src", path, "backup", bk, "err", err)
		return
	}
	slog.Info("损坏的 cache 文件已备份", "src", path, "backup", bk)
}
