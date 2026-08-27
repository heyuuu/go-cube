package project

import (
	"errors"
	"log"
	"log/slog"
	"path/filepath"
	"time"

	"cube/project/gitcache"
	"cube/util/easycache"
	"cube/util/fuzzy"
	"cube/util/git"
	"cube/util/pathkit"
	"cube/util/slicekit"
)

type Service struct {
	// settings
	settingsFile string // settings.json 路径，scan/clone 规则每次现读（直读不缓存，改完即生效）
	// scan
	scanCache *easycache.Item[[]*Project] // 项目扫描的缓存
	// git info cache
	gitCache *gitcache.Cache // git 信息缓存（项目 branch/dirty/repoUrl 等）
	// 刷新时间戳（供前端展示数据新鲜度；零值 = 未刷新过）
	scanUpdatedAt time.Time // 项目列表最近一次重扫完成时间
	// 定时刷新（仅常驻 server 启用，CLI 不启用）
	stopCh chan struct{} // nil = 未启用；非 nil = 定时器在跑
}

func NewService(settingsFile string, cacheDir string) *Service {
	// 加载 git 信息缓存（降级优先：失败返回空缓存，不报错）
	gitCache, err := gitcache.Load(cacheDir)
	if err != nil {
		log.Printf("加载 git 缓存失败: %v", err)
	}

	s := &Service{
		settingsFile: settingsFile,
		gitCache:     gitCache,
	}
	// 扫描时现读规则（规则直读 settings.json，外部修改后 Reload 即可反映）
	s.scanCache = easycache.NewItem(func() []*Project {
		projects, err := scan(s.ScanRules())
		if err != nil {
			slog.Error("扫描项目失败", "err", err)
			return nil
		}
		slog.Info("scan 项目完成", "count", len(projects))
		return projects
	})
	return s
}

// -- 规则（直读 settings.json，加载即转换校验） --

func (s *Service) ScanRules() []ScanRule {
	return loadScanRules(s.settingsFile)
}
func (s *Service) CloneRules() []CloneRule {
	return loadCloneRules(s.settingsFile)
}

// SaveScanRule 新增或按 path 替换一条 scan 规则（校验在写侧，坏数据返回中文错误）。
// 成功后立即重扫项目列表（毫秒级），保证 Web 保存后列表即时反映。
func (s *Service) SaveScanRule(rule ScanRule) error {
	if err := saveScanRule(s.settingsFile, rule); err != nil {
		return err
	}
	s.scanCache.Reload()
	return nil
}

func (s *Service) DeleteScanRule(path string) error {
	if err := deleteScanRule(s.settingsFile, path); err != nil {
		return err
	}
	s.scanCache.Reload()
	return nil
}

// ReorderScanRules 重排也影响项目列表展示序（scan 按规则序遍历），成功后同样立即重扫。
func (s *Service) ReorderScanRules(paths []string) error {
	if err := reorderScanRules(s.settingsFile, paths); err != nil {
		return err
	}
	s.scanCache.Reload()
	return nil
}

func (s *Service) SaveCloneRule(rule CloneRule) error {
	return saveCloneRule(s.settingsFile, rule)
}

func (s *Service) DeleteCloneRule(key CloneRuleKey) error {
	return deleteCloneRule(s.settingsFile, key)
}

func (s *Service) ReorderCloneRules(keys []CloneRuleKey) error {
	return reorderCloneRules(s.settingsFile, keys)
}

// --- project 读操作 ---

func (s *Service) Projects() []*Project {
	return s.scanCache.Get()
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
	// 兜底：符号链接口径二次比对。项目路径来自扫描（用户配置口径的字面路径），
	// 而调用方传入的可能是 git 规范化后的路径（macOS /var → /private/var，如 worktree
	// 归并链路），反之亦然。规范化两侧任一侧即可对齐；只有精确匹配落空才付这笔 syscall。
	realPath, realErr := filepath.EvalSymlinks(absPath)
	for _, proj := range s.Projects() {
		if proj.Path() == realPath {
			return proj
		}
		if realErr == nil {
			if projReal, err := filepath.EvalSymlinks(proj.Path()); err == nil && projReal == realPath {
				return proj
			}
		}
	}
	return nil
}

// SearchByName 按名称模糊搜索项目列表。
func (s *Service) SearchByName(query string) []*Project {
	return fuzzy.MatchBy(query, s.Projects(), (*Project).Name, nil)
}

// OpenTargets 返回项目的打开目标列表（1032：根目录在前 + worktrees）。
// 只读 gitcache 快照，不现场跑 git（遵守「读路径不得阻塞采集」）；项目未找到返回 nil。
func (s *Service) OpenTargets(path string) []OpenTarget {
	proj := s.FindByPath(path)
	if proj == nil {
		return nil
	}
	targets := []OpenTarget{{Path: proj.Path(), Label: "根目录"}}
	info, _ := s.GitInfo(proj.Path())
	return append(targets, worktreeTargets(info)...)
}

// ResolveProject 把目标目录归并到所属主项目：项目根本身直接命中，否则走 worktree
// 归并（ResolveMainProject）。所有「拿一个实际目录反查项目」的出口（打开目标 /
// usage 聚合 / 后续 workspace 子目录）统一走这里，不要在调用方手拼
// FindByPath + ResolveMainProject 的两段式。
// 项目普通子目录不在此列（那是 SearchByPath 的 up 语义，含多结果交互选择）。
func (s *Service) ResolveProject(dir string) *Project {
	if p := s.FindByPath(dir); p != nil {
		return p
	}
	return s.ResolveMainProject(dir)
}

// ResolveMainProject 把任意目录归并到主项目（1032 路径归并链路）：
// 目录（或其祖先）是 linked worktree 时，顺着 .git 文件定位主仓库并返回对应项目；
// 非 worktree 或主仓库未收录（不在任何 scan-rule 下）返回 nil。
// 只做文件系统探测 + 列表查找，不跑 git 子进程。
func (s *Service) ResolveMainProject(dir string) *Project {
	main := git.WorktreeMain(dir)
	if main == "" {
		return nil
	}
	return s.FindByPath(main)
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
	return MatchScanRule(absPath, s.ScanRules())
}

// --- clone 相关 ---

func (s *Service) MatchCloneRule(repoUrl string) (rule CloneRule, localPath string, ok bool) {
	return MatchCloneRule(repoUrl, s.CloneRules())
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
// 启动后立即刷新一次（避免冷启动空窗；磁盘缓存距上次落盘 < interval 时跳过），之后按 interval 定时刷新。
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

		// 启动即刷一次，避免冷启动空窗；但磁盘缓存仍新鲜（距上次落盘 < interval）时跳过——
		// 开发期 air 等热重载场景每次重启都全量重采上百个仓库，纯属浪费。
		if since := time.Since(s.gitCache.UpdatedAt()); since < interval {
			slog.Debug("git 缓存新鲜，跳过启动刷新", "上次落盘距今", since.Round(time.Second).String())
		} else {
			s.refresh()
		}

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

// OnServerStart 启动后台定时刷新（app 层钩子，仅常驻 server 调用）。等价于 StartRefreshTicker(0)。
func (s *Service) OnServerStart() { s.StartRefreshTicker(0) }

// OnServerStop 停止后台定时刷新 goroutine（app 层钩子，server shutdown 时调）。
func (s *Service) OnServerStop() { s.StopRefreshTicker() }

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
	total, collected, err := s.Refresh()
	if err != nil {
		slog.Warn("刷新 git 缓存失败", "err", err, "projects", total)
		return
	}
	slog.Debug("project 视图刷新完成", "projects", total, "collected", collected)
}

// Refresh 立即完整刷新 project 视图：重扫项目列表（纳入新增/剔除已删）→
// 整表采集 git 信息（写内存 + 落盘 git.json）。返回 (项目总数, 采集成功数, 错误)。
//
// 与 server 定时刷新同一逻辑。CLI 平时只读缓存不写（单写者模型：server 是唯一写方），
// 本方法仅供 dev 调试命令手动触发，用于开发期实测全量采集的时间成本。
func (s *Service) Refresh() (total int, collected int, err error) {
	projects := s.scanCache.Reload()
	s.scanUpdatedAt = time.Now() // 记录项目列表刷新时间

	paths := slicekit.Map(projects, (*Project).Path)
	if s.gitCache == nil {
		return len(paths), 0, errors.New("git 缓存未初始化")
	}
	if err := s.gitCache.Refresh(paths); err != nil {
		return len(paths), 0, err
	}
	return len(paths), s.gitCache.Size(), nil
}
