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
	// clone
	cloneRules []CloneRule // 项目 clone 规则
	// git info cache
	gitCache *gitcache.Cache // git 信息缓存（项目 branch/dirty/repoUrl 等）
	// 刷新时间戳（供前端展示数据新鲜度；零值 = 未刷新过）
	scanUpdatedAt time.Time // 项目列表最近一次重扫完成时间
	// 定时刷新（仅常驻 server 启用，CLI 不启用）
	stopCh chan struct{} // nil = 未启用；非 nil = 定时器在跑
}

func NewService(cfg config.ProjectConfig, cacheDir string) *Service {
	// scan 规则：展开 ~/ 为绝对路径，校验目录存在（不存在的规则降级跳过，不阻断）
	var scanRules []ScanRule
	for _, r := range cfg.Scan {
		absPath, err := pathkit.StaticAbsPath(r.Path)
		if err != nil {
			slog.Warn("scan 规则路径配置错误，跳过", "group", r.Group, "path", r.Path, "err", err)
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
	var cloneRules []CloneRule
	for _, r := range cfg.Clone {
		absLocalPath, err := pathkit.StaticAbsPath(r.LocalPath)
		if err != nil {
			slog.Warn("clone 本地路径配置错误", "localPath", r.LocalPath, "err", err)
			continue
		}
		cloneRules = append(cloneRules, CloneRule{
			RepoHost:   r.RepoHost,
			RepoPrefix: r.RepoPrefix,
			LocalPath:  absLocalPath,
		})
	}

	// 加载 git 信息缓存（降级优先：失败返回空缓存，不报错）
	gitCache, err := gitcache.Load(cacheDir)
	if err != nil {
		log.Printf("加载 git 缓存失败: %v", err)
	}

	s := &Service{
		scanRules: scanRules,
		scanCache: easycache.NewItem(func() []*Project {
			projects, err := scan(scanRules)
			if err != nil {
				slog.Error("扫描项目失败", "err", err)
				return nil
			}
			slog.Info("scan 项目完成", "count", len(projects))
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

func (s *Service) FindByName(name string) *Project {
	for _, proj := range s.Projects() {
		if proj.Name() == name {
			return proj
		}
	}
	return nil
}

func (s *Service) FindByPath(path string) *Project {
	// 仅接受绝对路径/~ 前缀（调用方是 web，server 进程的 cwd 对请求路径无意义）；
	// 相对路径视为未找到而非报错，与「查无此项目」语义一致
	absPath, err := pathkit.StaticAbsPath(path)
	if err != nil {
		return nil
	}

	for _, proj := range s.Projects() {
		if proj.Path() == absPath {
			return proj
		}
	}
	return nil
}

func (s *Service) SearchByName(query string) []*Project {
	return fuzzy.MatchBy(query, s.Projects(), (*Project).Name, nil)
}

// SearchByPath 通过路径搜索项目列表
// up 表示是否向上搜索。用于通过项目子目录标定当前目录时使用。
// 因为项目子目录不可能是另一个项目的目录或父目录，所以当向上匹配成功时不会出现其他项目
//
// path 只接受绝对路径或 ~ 前缀（经 StaticAbsPath 归一化，兼容 web 直接传 ~/xxx）；
// 相对路径的 cwd 解析是出口层职责（cmd 用 AbsPath），到这里说明调用方传错，按未找到处理。
func (s *Service) SearchByPath(path string, up bool) []*Project {
	absPath, err := pathkit.StaticAbsPath(path)
	if err != nil {
		return nil
	}

	var result []*Project
	for _, proj := range s.Projects() {
		// 判断 proj 是否在 realpath 目录及子目录中
		if pathkit.HasPrefix(proj.Path(), absPath) {
			result = append(result, proj)
			continue
		}
		// 若向上查找， 判断 proj.Path() 是否在 realpath 父目录
		if up && pathkit.HasPrefix(absPath, proj.Path()) {
			result = append(result, proj)
			continue
		}
	}
	return result
}

// --- scan 相关 ---

// MatchScanRule 判断 absPath（git init 后）能否被 scan 收录为新项目（供 init 命令预检）。
// 返回匹配的规则与项目名；不满足收录条件时 ok=false。
func (s *Service) MatchScanRule(absPath string) (rule ScanRule, name string, ok bool) {
	return MatchScanRule(absPath, s.scanRules)
}

// --- clone 相关 ---

func (s *Service) MatchCloneRule(repoUrl string) (rule CloneRule, localPath string, ok bool) {
	return MatchCloneRule(repoUrl, s.cloneRules)
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

// ScanUpdatedAt 返回项目列表最近一次重扫完成时间；未刷新过返回零值。
func (s *Service) ScanUpdatedAt() time.Time { return s.scanUpdatedAt }

// GitUpdatedAt 返回 git 缓存最近一次落盘时间；无缓存返回零值。
// 区别于单项目的 CollectedAt：这是整个 cache 文件的刷新时间。
func (s *Service) GitUpdatedAt() time.Time {
	if s.gitCache == nil {
		return time.Time{}
	}
	return s.gitCache.UpdatedAt()
}

// defaultRefreshInterval 默认定时刷新间隔。
// 主要受限于 git 采集（单次约数十秒级，取决于项目数）；扫描本身极快（毫秒级）。
// 过短会让后台频繁读几十个仓库；过长则缓存陈旧。
// StartRefreshTicker 传 interval <= 0 时用此默认值。
const defaultRefreshInterval = 5 * time.Minute

// StartRefreshTicker 启动后台定时刷新 project 视图（项目列表 + git info）的 goroutine（仅常驻 server 调用）。
//
// interval <= 0 时用 defaultRefreshInterval。重复调用安全：已在跑则直接返回。
// 启动后立即刷新一次（避免冷启动空窗），之后按 interval 定时刷新。
// 刷新动作：重扫项目列表（毫秒级）→ 记录 scanUpdatedAt → 用最新列表采集 git 信息（数十秒级）→ 记录 gitUpdatedAt。
// 两个时间戳分开记录：扫描极快、git 采集慢，前端需据此分别判断「项目列表新鲜度」和「git 状态新鲜度」。
//
// CLI 不调用此方法（CLI 短命，只读启动时 Load 的快照）。
func (s *Service) StartRefreshTicker(interval time.Duration) {
	if s.gitCache == nil || s.stopCh != nil {
		return // 无缓存或已在跑
	}
	if interval <= 0 {
		interval = defaultRefreshInterval
	}

	stopCh := make(chan struct{})
	s.stopCh = stopCh

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("project 视图刷新 ticker panic", "err", r)
			}
		}()

		s.refresh() // 启动即刷一次，避免冷启动空窗

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.refresh()
			case <-stopCh:
				return
			}
		}
	}()
}

// StopRefreshTicker 停止定时刷新 goroutine（server shutdown 时调）。
// 未启用时调用安全（空操作）。
func (s *Service) StopRefreshTicker() {
	if s.stopCh == nil {
		return
	}
	close(s.stopCh)
	s.stopCh = nil
}

// refresh 刷新 project 视图：先重扫项目列表（纳入新增/剔除已删），再用最新列表采集 git 信息。
// 采集异常不抛出（降级优先）：失败只 slog 记录，不影响 server 进程。
func (s *Service) refresh() {
	// 强制重扫项目列表
	projects := s.scanCache.Reload()
	s.scanUpdatedAt = time.Now() // 记录项目列表刷新时间

	// 刷新 git 信息
	paths := slicekit.Map(projects, (*Project).Path)
	if err := s.gitCache.Refresh(paths); err != nil {
		slog.Warn("刷新 git 缓存失败", "err", err, "projects", len(paths))
		return
	}

	slog.Debug("project 视图刷新完成", "projects", len(paths))
}
