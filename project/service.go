package project

import (
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/heyuuu/cube/config"
	"github.com/heyuuu/cube/project/gitcache"
	"github.com/heyuuu/cube/util/easycache"
	"github.com/heyuuu/cube/util/fuzzy"
	"github.com/heyuuu/cube/util/pathkit"
	"github.com/heyuuu/cube/util/slicekit"
)

type Service struct {
	mu sync.RWMutex
	// scan
	scanRules []ScanRule                  // 项目扫描规则
	scanCache *easycache.Item[[]*Project] // 项目扫描的缓存
	// git info cache
	gitCache *gitcache.Cache // git 信息缓存（项目 branch/dirty/repoUrl 等）
	// clone
	cloneRules []CloneRule // 项目 clone 规则
}

func NewService(conf config.ProjectConfig, cacheDir string) *Service {
	// 加载 git 信息缓存（降级优先：失败返回空缓存，不报错）
	gitCache, err := gitcache.Load(cacheDir)
	if err != nil {
		log.Printf("load git cache failed: %v", err)
	}

	s := &Service{gitCache: gitCache}
	s.scanCache = easycache.NewItem(s.loadProjects)
	s.applyConf(conf)
	return s
}

// applyConf 按配置重置 scan/clone 规则（路径展开 + 校验）。调用方负责持锁。
func (s *Service) applyConf(conf config.ProjectConfig) {
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

	s.scanRules = scanRules
	s.cloneRules = cloneRules
	// scanCache 清空：规则变了，旧的项目列表已失效，下次 Projects() 重新扫描
	s.scanCache.Clear()
}

// Reload 用新配置热更新 scan/clone 规则。gitCache 不动（缓存目录未变）。
// 供配置监听器在 config 变更后调用，让长驻进程无需重启即可应用新扫描配置。
func (s *Service) Reload(conf config.ProjectConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyConf(conf)
}

// -- getter --

func (s *Service) ScanRules() []ScanRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scanRules
}
func (s *Service) CloneRules() []CloneRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cloneRules
}

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

// --- scan 相关 ---

// 加载所有项目的实际逻辑
func (s *Service) loadProjects() []*Project {
	s.mu.RLock()
	scanRules := s.scanRules
	gitCache := s.gitCache
	s.mu.RUnlock()

	var result []*Project
	for _, rule := range scanRules {
		err := scanProjects(rule, func(path string, tags []string) {
			var gitInfo *GitInfo
			if gitCache != nil {
				gitInfo, _ = gitCache.Get(path)
			}
			project := newProject(rule, path, tags, gitInfo)
			result = append(result, project)
		})
		if err != nil {
			log.Println(err)
		}
	}
	return result
}

// --- clone 相关 ---

func (s *Service) MatchCloneRule(repoUrl string) (rule CloneRule, localPath string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return MatchCloneRule(repoUrl, s.cloneRules)
}
