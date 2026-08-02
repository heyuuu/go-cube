// Package gitcache 提供 git 信息的本地缓存。
//
// cube 是 CLI 模式执行，每次 `project list --status` 都要为每个 git 项目采集 git 信息。
// 对几十个项目的全量采集仍是 IO 密集型操作，每次 CLI 调用都跑一遍会明显卡顿。
//
// 本包把这些信息缓存到 ~/.config/cube/cache/git.json：
//   - 前台读命令（project list/info）从缓存读，几乎零开销；
//   - 后台子进程（由 refresh.go 触发）异步采集并回写缓存。
//
// 本文件 (cache.go) 只负责「数据层」：结构定义、Load/Get/Save/Refresh。
// 进程调度（flock/fork/TTL）见 refresh.go。
package gitcache

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/heyuuu/cube/util/gogit"
)

// 当前缓存文件格式版本；结构变更时递增，用于后续做兼容迁移。
const cacheVersion = 1

// 缓存文件名（位于缓存目录 dir 下）。
const cacheFileName = "git.json"

// Entry 单个项目的 git 信息快照。
type Entry struct {
	RepoUrl       string    `json:"repoUrl"`       // origin remote URL
	CurrentBranch string    `json:"currentBranch"` // HEAD 指向分支短名，detached 为空
	DefaultBranch string    `json:"defaultBranch"` // 默认主分支名（master/main/...）
	Branches      []string  `json:"branches"`      // 本地+远程分支短名列表
	Ahead         int       `json:"ahead"`         // 默认分支相对 origin 的领先 commit 数
	Behind        int       `json:"behind"`        // 落后的 commit 数
	Dirty         bool      `json:"dirty"`         // 工作区是否有改动
	CollectedAt   time.Time `json:"collectedAt"`   // 本次采集时间
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
// 并发模型：
//   - 同进程内：RWMutex 保护 map 读写；Refresh 整条 entry 替换（非字段级原地改）。
//   - 跨进程：由 refresh.go 的 flock 保证同一时刻只有一个采集进程在写文件；
//     读进程加载快照到内存后只读不改，不存在真并发修改。
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

// IsStale 判断磁盘缓存文件是否比内存新（即后台子进程已刷新落盘，需 Reload）。
// 用磁盘文件的 mod-time 对比内存记录的 UpdatedAt；磁盘更新则视为 stale。
func (c *Cache) IsStale() bool {
	info, err := os.Stat(c.path())
	if err != nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	// 磁盘 mtime 比内存 updatedAt 晚至少 1 秒才判 stale（避免同秒内抖动）
	return info.ModTime().After(c.updatedAt.Add(time.Second))
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
		if os.IsNotExist(err) {
			return c, nil // 正常的冷启动
		}
		return c, nil // 其他读错误也降级
	}

	c.loadFromBytes(path, data)
	return c, nil
}

// loadFromBytes 解析缓存文件字节并写回内存（entries + updatedAt）。供 Load / Reload 复用。
func (c *Cache) loadFromBytes(path string, data []byte) {
	var file cacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		slog.Warn("git cache file corrupted, backing up and starting fresh",
			"path", path, "err", err)
		backupCorrupt(path, data)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if file.Entries != nil {
		c.entries = file.Entries
	}
	c.updatedAt = file.UpdatedAt
}

// Reload 重新从磁盘读取缓存文件，刷新内存里的 entries + updatedAt。
// 供长驻进程（web server）感知后台子进程的刷新结果：子进程 fork 采集落盘后，
// 父进程调 Reload 即可拿到最新数据，无需重启。
func (c *Cache) Reload() {
	path := c.path()
	data, err := os.ReadFile(path)
	if err != nil {
		return // 文件不存在/读失败：保持旧内存数据（降级）
	}
	c.loadFromBytes(path, data)
}

// path 返回缓存文件完整路径（包内自用，对外只暴露 Dir）。
func (c *Cache) path() string { return filepath.Join(c.dir, cacheFileName) }

// Dir 返回缓存目录路径。供 refresh.go 的 TTL/flock 等调度逻辑使用。
func (c *Cache) Dir() string { return c.dir }

// Get 读取单个项目的缓存条目；未命中返回 (nil, false)。
func (c *Cache) Get(path string) (*Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[path]
	return e, ok
}

// Save 原子写入 git.json。
// 流程：序列化 → 写 git.json.tmp → rename 覆盖 git.json。
// rename 保证原子性（同文件系统下）；tmp 与目标同目录以满足这一前提。
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

	path := c.path()
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("写入 cache 临时文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("重命名 cache 临时文件失败: %w", err)
	}
	return nil
}

// Refresh 用给定的项目路径列表并发采集 git 信息，写回内存 + 落盘。
//
// 入参用 []string（项目绝对路径）而非 []*project.Project，刻意解耦对 project 包的依赖，
// 避免 project → gitcache → project 的循环 import。
//
// 并发上限由包内 defaultWorkers 固定为 8（go-git 状态读取是 IO 密集型，过高并发
// 会与系统其他 IO 抢资源）。
// 单项目采集 panic 会被 recover 吞掉（该项目保留旧 entry）。
func (c *Cache) Refresh(paths []string) error {
	if len(paths) == 0 {
		return c.Save() // 无项目也刷一次 UpdatedAt
	}

	// 并发采集
	type result struct {
		path  string
		entry *Entry
	}
	sem := make(chan struct{}, defaultWorkers)
	results := make(chan result, len(paths))
	var wg sync.WaitGroup

	for _, path := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Warn("collect git entry panicked",
						"path", path, "panic", r)
				}
			}()

			// 限制并发
			sem <- struct{}{}
			defer func() { <-sem }()

			entry := collectEntry(path)
			results <- result{path: path, entry: entry}
		}(path)
	}
	wg.Wait()
	close(results)

	// 合并结果到内存（整条 entry 替换）
	c.mu.Lock()
	for r := range results {
		if r.entry != nil {
			c.entries[r.path] = r.entry
		} // nil 表示采集失败，保留旧 entry
	}
	c.mu.Unlock()

	return c.Save()
}

// defaultWorkers 默认并发数。
// 偏保守：go-git 状态读取是 IO 密集型，过高并发会与系统其他 IO 抢资源。
const defaultWorkers = 8

// collectEntry 采集单个项目的 git 信息。
// 依赖 util/git 包的错误约定：业务空值场景返回零值+nil，所以这里基本不会拿到 error。
func collectEntry(path string) *Entry {
	repoUrl, _ := gogit.RemoteUrl(path)
	branches, currBranch, _ := gogit.Branches(path)
	defaultBranch, _ := gogit.DefaultBranch(path)
	// ahead/behind 用仓库的默认分支（master/main/...）做本地 vs 远程比较
	var ahead, behind int
	if defaultBranch != "" {
		ahead, behind, _ = gogit.AheadBehind(path, defaultBranch, "origin/"+defaultBranch)
	}
	dirty, _ := gogit.IsDirty(path)
	return &Entry{
		RepoUrl:       repoUrl,
		CurrentBranch: currBranch,
		DefaultBranch: defaultBranch,
		Branches:      branches,
		Ahead:         ahead,
		Behind:        behind,
		Dirty:         dirty,
		CollectedAt:   time.Now(),
	}
}

// backupCorrupt 把损坏文件备份到 git.json.corrupt-{timestamp}，便于事后排查。
// 备份失败不影响主流程（最多 Warn 一次）。
func backupCorrupt(path string, data []byte) {
	bk := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
	if err := os.WriteFile(bk, data, 0644); err != nil {
		slog.Warn("backup corrupt cache file failed", "src", path, "backup", bk, "err", err)
		return
	}
	slog.Info("corrupt cache file backed up", "src", path, "backup", bk)
}
