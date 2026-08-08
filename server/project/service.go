package project

import (
	"log"
	"log/slog"
	"os"
	"time"

	"cube/config"
	"cube/project/gitcache"
	"cube/util/easycache"
	"cube/util/fuzzy"
	"cube/util/pathkit"
	"cube/util/slicekit"
)

type Service struct {
	// scan
	scanRules []ScanRule                  // 项目扫描规则
	scanCache *easycache.Item[[]*Project] // 项目扫描的缓存
	// git info cache
	gitCache *gitcache.Cache // git 信息缓存（项目 branch/dirty/repoUrl 等）
	// clone
	cloneRules []CloneRule // 项目 clone 规则
}

func NewService(conf config.ProjectConfig, cacheDir string) *Service {
	// scan 规则：展开 ~/ 为绝对路径，校验目录存在（不存在的规则降级跳过，不阻断）
	var scanRules []ScanRule
	for _, r := range conf.Scan {
		absPath := pathkit.RealPath(r.Path)
		if absPath == "" {
			slog.Warn("scan 规则路径为空，跳过", "group", r.Group, "path", r.Path)
			continue
		}
		if info, err := os.Stat(absPath); err != nil || !info.IsDir() {
			slog.Warn("scan 规则路径不存在或非目录，跳过", "group", r.Group, "path", r.Path, "abs", absPath, "err", err)
			continue
		}
		scanRules = append(scanRules, ScanRule{
			Group:    r.Group,
			Path:     absPath,
			MaxDepth: r.MaxDepth,
		})
	}

	// clone 规则：LocalPath 展开 ~/ 为绝对路径（不校验存在——clone 时会自动创建）
	cloneRules := slicekit.Map(conf.Clone, func(r config.CloneRuleConfig) CloneRule {
		return CloneRule{
			RepoHost:   r.RepoHost,
			RepoPrefix: r.RepoPrefix,
			LocalPath:  pathkit.RealPath(r.LocalPath),
		}
	})

	// 加载 git 信息缓存（降级优先：失败返回空缓存，不报错）
	gitCache, err := gitcache.Load(cacheDir)
	if err != nil {
		log.Printf("load git cache failed: %v", err)
	}

	s := &Service{
		scanRules: scanRules,
		scanCache: easycache.NewItem(func() []*Project {
			projects, err := scan(scanRules)
			if err != nil {
				slog.Error("scanWithGitCache failed: %v", "err", err)
				return nil
			}
			for _, p := range projects {
				p.gitInfo, _ = gitCache.Get(p.Path())
			}
			return projects
		}),
		gitCache:   gitCache,
		cloneRules: cloneRules,
	}
	return s
}

// -- getter --

func (s *Service) ScanRules() []ScanRule   { return s.scanRules }
func (s *Service) CloneRules() []CloneRule { return s.cloneRules }

// --- project 读操作 ---

func (s *Service) Projects() []*Project {
	return s.scanCache.Get()
}

func (s *Service) FindByPath(path string) *Project {
	for _, proj := range s.Projects() {
		if proj.Path() == path {
			return proj
		}
	}
	return nil
}

func (s *Service) FindByName(name string) *Project {
	for _, proj := range s.Projects() {
		if proj.Name() == name {
			return proj
		}
	}
	return nil
}

func (s *Service) Search(query string) []*Project {
	return fuzzy.MatchBy(query, s.Projects(), (*Project).Name, nil)
}

// --- git 缓存相关 ---

// GitInfo 读取项目的 git 信息缓存条目；未命中返回 (nil, false)。
// 不阻塞、不触发采集 —— 调用方读取的是当前缓存快照（可能 stale）。
func (s *Service) GitInfo(path string) (*gitcache.Entry, bool) {
	if s.gitCache == nil {
		return nil, false
	}
	return s.gitCache.Get(path)
}

// GitCacheUpdatedAt 返回 git 缓存整体最近一次落盘时间；无缓存返回零值。
// 区别于单项目的 CollectedAt：这是整个 cache 文件的刷新时间。
func (s *Service) GitCacheUpdatedAt() time.Time {
	if s.gitCache == nil {
		return time.Time{}
	}
	return s.gitCache.UpdatedAt()
}

// ReloadGitCacheIfStale 检测磁盘 git.json 是否比内存新，若是则重新加载。
// 供长驻 web server 感知后台 fork 子进程的刷新结果：web 进程内存里的 cache
// 是启动时的快照，子进程写盘后父进程不会自动感知，需主动 Reload。
// Reload 后同时清空 scanCache，让下次 Projects() 重新用新 gitInfo 构造项目。
func (s *Service) ReloadGitCacheIfStale() {
	if s.gitCache == nil {
		return
	}
	if !s.gitCache.IsStale() {
		return
	}
	s.gitCache.Reload()
	s.scanCache.Clear() // gitInfo 变了，项目数据需重建
}

// TriggerAsyncRefresh 触发一次异步刷新：TTL 内直接返回，否则 fork 子进程后台采集。
// 非阻塞，立即返回。供读命令（list/info）在返回前调用。
func (s *Service) TriggerAsyncRefresh() {
	if s.gitCache == nil {
		return
	}
	// TTL 固定用 gitcache 包的默认值（1 分钟）；如需调整再暴露参数。
	gitcache.TryAsyncRefresh(s.gitCache.Dir(), time.Minute)
}

// --- clone 相关 ---

func (s *Service) MatchCloneRule(repoUrl string) (rule CloneRule, localPath string, ok bool) {
	return MatchCloneRule(repoUrl, s.cloneRules)
}
