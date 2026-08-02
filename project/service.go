package project

import (
	"log"
	"time"

	"github.com/heyuuu/cube/config"
	"github.com/heyuuu/cube/project/gitcache"
	"github.com/heyuuu/cube/util/easycache"
	"github.com/heyuuu/cube/util/fuzzy"
	"github.com/heyuuu/cube/util/slicekit"
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
	scanRules := slicekit.Map(conf.Scan, func(r config.ScanRuleConfig) ScanRule {
		return ScanRule{
			Group:    r.Group,
			Path:     r.Path,
			MaxDepth: r.MaxDepth,
		}
	})
	cloneRules := slicekit.Map(conf.Clone, func(r config.CloneRuleConfig) CloneRule {
		return CloneRule{
			RepoHost:   r.RepoHost,
			RepoPrefix: r.RepoPrefix,
			LocalPath:  r.LocalPath,
		}
	})

	// 加载 git 信息缓存（降级优先：失败返回空缓存，不报错）
	gitCache, err := gitcache.Load(cacheDir)
	if err != nil {
		log.Printf("load git cache failed: %v", err)
	}

	// 构建 service 实体
	s := &Service{
		scanRules:  scanRules,
		gitCache:   gitCache,
		cloneRules: cloneRules,
	}
	s.scanCache = easycache.NewItem(s.loadProjects)
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
	var result []*Project
	for _, rule := range s.scanRules {
		err := scanProjects(rule, func(path string, tags []string) {
			gitInfo, _ := s.gitCache.Get(path)
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
	return MatchCloneRule(repoUrl, s.cloneRules)
}
