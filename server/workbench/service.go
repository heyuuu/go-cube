// Package workbench 是工作台领域包（提案 docs/proposals/1010-workbench基座）。
// 工作台以任意本机 git 目录为输入（不依赖 project scan、不走 projcache），
// 信息全部直接调 git 获取（util/git 读能力），实时性由前端缓存控制。
// 分层注意：本包不 import project 包，git 读能力沉淀在 util/git。
package workbench

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"cube/util/git"
	"cube/util/slicekit"

	"github.com/coder/websocket"
)

// Service 工作台领域服务。git 读路径无状态（每次调用直接调 git）；
// 运行期状态是 PTY 会话注册表（server 停机时统一回收，见 pty.go）与写侧
// projcache 定向刷新回调（app 装配点注入 project 域实现，本包不依赖 project）。
type Service struct {
	cacheRefresh func(path string) error // 写操作成功后定向刷新 projcache（nil = 无刷新能力，跳过）

	ptyMu      sync.Mutex
	ptySeq     int
	ptyCancels map[int]context.CancelFunc
}

func NewService(cacheRefresh func(path string) error) *Service {
	return &Service{cacheRefresh: cacheRefresh, ptyCancels: map[int]context.CancelFunc{}}
}

// refreshCache 写操作成功后定向刷新主项目快照。刷新失败只 Warn（写操作本身已
// 成功，不应因此报错回滚用户视角），快照等 TTL 整表重建自愈。
func (s *Service) refreshCache(mainRoot string) {
	if s.cacheRefresh == nil {
		return
	}
	if err := s.cacheRefresh(mainRoot); err != nil {
		slog.Warn("写操作后定向刷新 git 缓存失败，等待 TTL 自愈", "path", mainRoot, "err", err)
	}
}

// --- git 面板（info / 分支与 tag / commit 日志 / 工作副本快照）---

// Info 读工作台仓库信息：入口目录向上探测仓库根、默认分支。
func (s *Service) Info(path string) (*Info, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	defaultBranch, _ := git.DefaultBranch(root) // 无 remote 返回空，可接受
	return &Info{Root: root, DefaultBranch: defaultBranch}, nil
}

// Refs 分支与 tag 列表（规范全名，见 Refs 注释）+ 当前检出分支，作为 git 树面板 /
// 双选交互的候选目标。当前分支是 HEAD 状态而非 ref 枚举的一部分，单独取（HeadRef）。
func (s *Service) Refs(path string) (*Refs, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}

	head := git.HeadRef(root)
	refs, err := git.Refs(root)
	if err != nil {
		return nil, err
	}

	getName := func(r git.Ref) string { return r.Name }
	return &Refs{
		Head:    head,
		Locals:  slicekit.Map(refs.Locals, getName),
		Remotes: slicekit.Map(refs.Remotes, getName),
		Tags:    slicekit.Map(refs.Tags, getName),
	}, nil
}

// Remotes 列出仓库配置的 remote（名字 + 抓取地址 + 网页地址），git 树面板
// 「远端」分组展示。url 无法解析成网页地址时 webUrl 为空，前端隐藏跳转按钮。
func (s *Service) Remotes(path string) ([]RemoteEntry, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	remotes, err := git.Remotes(root)
	if err != nil {
		return nil, err
	}
	list := make([]RemoteEntry, 0, len(remotes))
	for _, r := range remotes {
		entry := RemoteEntry{Name: r.Name, Url: r.Fetch}
		if repoUrl, err := git.ParseRepoUrl(r.Fetch); err == nil {
			entry.WebUrl = repoUrl.WebUrl()
		}
		list = append(list, entry)
	}
	return list, nil
}

// Commits 拉取 commit 日志一页（--all 全分支；纯列表，泳道布局由前端对已持有数据计算）。
func (s *Service) Commits(path string, cursor int, limit int) (*CommitsPageResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if cursor < 0 {
		cursor = 0
	}

	// 多取 1 条探测：恰好读到总数为 limit 整数倍时，len==limit 不代表还有更多
	list, err := git.CommitsPage(root, cursor, limit+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(list) > limit
	if hasMore {
		list = list[:limit]
	}
	return &CommitsPageResult{
		List:       list,
		NextCursor: cursor + limit,
		HasMore:    hasMore,
	}, nil
}

// WorktreeStatuses 返回全部工作副本的状态快照。工作副本徽标与 commit 图
// 虚拟节点（前端构造）共用这一份数据——status 只在这里拉，不再分散到各接口。
func (s *Service) WorktreeStatuses(path string) ([]WorktreeStatus, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	worktrees, err := git.WorktreeList(root)
	if err != nil {
		return nil, err
	}
	result := make([]WorktreeStatus, 0, len(worktrees))
	for _, wt := range worktrees {
		item := WorktreeStatus{
			Path:     wt.Path,
			Head:     wt.Head,
			Branch:   wt.Branch,
			Detached: wt.Detached,
			Bare:     wt.Bare,
		}
		// 单副本状态失败不拖垮整张快照（bare 副本 LoadRepoStatus 天然返回零值）
		if st, err := git.LoadRepoStatus(wt.Path); err != nil {
			slog.Debug("工作副本状态读取失败，降级为零值", "dir", wt.Path, "err", err)
		} else if st != nil {
			item.Dirty, item.Ahead, item.Behind = st.Dirty, st.Ahead, st.Behind
			item.Staged, item.Unstaged, item.Untracked = st.Staged, st.Unstaged, st.Untracked
		}
		result = append(result, item)
	}
	return result, nil
}

// --- worktree / 分支写侧（提案 1031；主题逻辑见 worktree_write.go）---

// WorktreeAdd 新增 worktree。branch / commitish 决定形态（新建分支 / 检出已有 /
// detached，语义见 git.WorktreeAdd）；branch 为规范全名时剥 refs/heads/ 前缀。
// targetPath 为空时按决策 1 预填 <repoName>.worktrees/<分支名>/。成功后返回新副本
// 信息（供 UI 直接发起 open），并定向刷新主项目快照。
func (s *Service) WorktreeAdd(path string, branch string, commitish string, targetPath string) (*WorktreeCreated, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	branch = strings.TrimPrefix(branch, "refs/heads/")

	mainRoot := mainRootOf(root)
	if targetPath == "" {
		targetPath = prefillWorktreePath(mainRoot, branch, commitish)
	}
	if err := checkTargetDir(targetPath); err != nil {
		return nil, err
	}
	wt, err := git.WorktreeAdd(root, targetPath, branch, commitish)
	if err != nil {
		return nil, err
	}
	s.refreshCache(mainRoot)
	return &WorktreeCreated{Path: wt.Path, Branch: wt.Branch, Detached: wt.Detached}, nil
}

// WorktreeRemove 删除 worktree（删目录 + prune 收尾）。非 force 先预检
// （未提交改动 / 未跟踪 / 未推送），有风险项返回 *WorktreeRemoveDenied 由 UI
// 二次确认升级 force；主仓库工作目录无论 force 均拒绝（那是删仓库本身）。
func (s *Service) WorktreeRemove(path string, targetPath string, force bool) error {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	mainRoot := mainRootOf(root)
	if targetPath == mainRoot {
		return fmt.Errorf("主仓库工作目录不能删除: %s", mainRoot)
	}
	if !force {
		reasons, err := worktreeRemoveBlockers(targetPath)
		if err != nil {
			return err
		}
		if len(reasons) > 0 {
			return &WorktreeRemoveDenied{Reasons: reasons}
		}
	}
	if err := git.WorktreeRemove(root, targetPath, force); err != nil {
		return err
	}
	if err := git.WorktreePrune(mainRoot); err != nil {
		return err
	}
	s.refreshCache(mainRoot)
	return nil
}

// BranchDelete 删除本地分支。被任一工作副本（含主目录）检出的分支是硬约束，
// 无论 force 均拒绝并说明检出位置；其余走 force 开关语义（见 git.BranchDelete）。
// BranchAdd 新建本地分支（不检出、不切 HEAD——「建分支并切过去」由 WorktreeAdd 覆盖）。
// branch 支持规范全名或短名；commitish 为基点（空 = HEAD）。成功后刷新主项目快照。
func (s *Service) BranchAdd(path string, branch string, commitish string) error {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	branch = strings.TrimPrefix(branch, "refs/heads/")
	if err := git.BranchAdd(root, branch, commitish); err != nil {
		return err
	}
	s.refreshCache(mainRootOf(root))
	return nil
}

func (s *Service) BranchDelete(path string, branch string, force bool) error {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	list, err := git.WorktreeList(root)
	if err != nil {
		return err
	}
	for _, wt := range list {
		if wt.Branch == branch {
			return fmt.Errorf("分支 %s 正被工作副本检出，无法删除: %s", branch, wt.Path)
		}
	}
	if err := git.BranchDelete(root, branch, force); err != nil {
		return err
	}
	s.refreshCache(mainRootOf(root))
	return nil
}

// --- 文件树 / 文件读写 / diff（代码阅读面板与 diff 面板）---

// Tree 全量列出 TreeSource 下 git 管理的文件（扁平相对路径，前端组树，不再逐层请求）。
// 统一口径：worktree 源 = tracked + 未跟踪未忽略（ls-files，含已暂存未提交）；
// commit/ref 源 = 该提交树内的全部文件（ls-tree -r）。被忽略文件在两种源下都不返回。
func (s *Service) Tree(path string, src TreeSource) (*TreeListResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	switch src.Type {
	case SourceTypeWorktree:
		files, err := git.ListFiles(src.Id)
		if err != nil {
			return nil, fmt.Errorf("读取工作副本文件清单失败: dir=%s: %w", src.Id, err)
		}
		return &TreeListResult{List: files}, nil
	case SourceTypeCommit, SourceTypeRef:
		m, err := git.FileShasAtRef(root, src.Id)
		if err != nil {
			return nil, fmt.Errorf("读取 %s 下的树失败: %w", src.Id, err)
		}
		list := make([]string, 0, len(m))
		for p := range m {
			list = append(list, p)
		}
		sort.Strings(list)
		return &TreeListResult{List: list}, nil
	default:
		return nil, fmt.Errorf("未知的 sourceType: %q", src.Type)
	}
}

// ReadFile 读某 TreeSource 下 file 的内容。实现在 file.go（二进制检测：前 8KB 含 NUL）。
func (s *Service) ReadFile(path string, src TreeSource, file string) (*FileResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	return readFile(root, src, file)
}

// SaveFile 写工作副本文件（提案 1012 唯一落盘写路径）。实现在 file.go。
func (s *Service) SaveFile(path string, src TreeSource, file string, content string) (*FileResult, error) {
	return saveFile(path, src, file, content)
}

// DiffTrees 对比 base → current 两个 TreeSource 的目录树。实现在 diff.go（git 模式 / fs 扫描模式）。
// 状态/路径筛选在 diff 面板前端本地做（变更清单一次全量返回）。
func (s *Service) DiffTrees(path string, base TreeSource, current TreeSource) (*DiffTreesResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	if base.Type == SourceTypeWorktree || current.Type == SourceTypeWorktree {
		result, err := diffTreesFs(root, base, current)
		if err != nil {
			return nil, err
		}
		annotateDiffStats(result, root, base, current)
		return result, nil
	}
	result, err := diffTreesGit(root, base, current)
	if err != nil {
		return nil, err
	}
	annotateDiffStats(result, root, base, current)
	return result, nil
}

// changeBase 解析「相对上一版本」的基准：worktree → HEAD；commit/ref → 父提交（根提交落空树）。
// Changes 与 ReadFileDiff（base 缺省时）共用
func changeBase(root string, src TreeSource) (string, error) {
	switch src.Type {
	case SourceTypeWorktree:
		head, err := git.HeadSha(src.Id)
		if err != nil {
			return "", fmt.Errorf("读取工作副本 HEAD 失败: dir=%s: %w", src.Id, err)
		}
		return head, nil
	case SourceTypeCommit, SourceTypeRef:
		parent, err := git.ParentSha(root, src.Id)
		if err != nil {
			return emptyTreeSha, nil // 根提交没有父：与空树比
		}
		return parent, nil
	default:
		return "", fmt.Errorf("未知的 sourceType: %q", src.Type)
	}
}

// ReadFileDiff 对比 base → current 两个源下某文件。实现在 diff.go。baseFile 为基准侧路径
// （rename 条目与当前侧路径不同，空则同 file）；base 为零值时按「相对基准」对比
// （worktree vs HEAD、ref/commit vs 父提交，同 Changes）。
func (s *Service) ReadFileDiff(path string, base TreeSource, current TreeSource, file string, baseFile string) (*FileDiffResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	if base == (TreeSource{}) {
		baseSha, err := changeBase(root, current)
		if err != nil {
			return nil, err
		}
		base = TreeSource{Type: SourceTypeCommit, Id: baseSha}
	}
	return readFileDiff(root, base, current, file, baseFile)
}

// Changes 列出源相对「上一版本」的变更文件（代码阅读面板的差异模式）：
//   - commit / ref：与父提交（<id>^）比；
//   - worktree：与该工作副本当前 HEAD 比（= 工作区变更，含 untracked）。
//
// 复用 DiffTrees：含 worktree 侧自动走 fs 模式。
func (s *Service) Changes(path string, src TreeSource) (*DiffTreesResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	base, err := changeBase(root, src)
	if err != nil {
		return nil, err
	}
	res, err := s.DiffTrees(path, TreeSource{Type: SourceTypeCommit, Id: base}, src)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// annotateDiffStats 为 DiffTrees 的变更清单注入行级增删统计（git.Numstat，-M rename 检测）：
//   - 当前侧 worktree：numstat 与工作区比（不含 untracked），未命中的 added 即 untracked，
//     按文件行数计 adds（二进制探测 NUL 字节）；
//   - 双侧 tree-ish：numstat 直接可比，全部命中；
//   - 基准侧 worktree（含 worktree vs worktree）：numstat 方向不便，不注入（统计留零值）。
//
// 统计失败只降级（adds/dels 留零值），不阻断变更清单——文件列表本身仍可用。
func annotateDiffStats(res *DiffTreesResult, root string, base TreeSource, current TreeSource) {
	if base.Type == SourceTypeWorktree {
		return
	}
	var stats map[string]git.NumstatEntry
	var err error
	if current.Type == SourceTypeWorktree {
		stats, err = git.Numstat(current.Id, base.Id, "")
	} else {
		stats, err = git.Numstat(root, base.Id, current.Id)
	}
	if err != nil {
		slog.Debug("变更行数统计失败，降级为零值", "err", err)
		return
	}
	mergeChangeRenames(res, stats)
	for i := range res.List {
		e := &res.List[i]
		if st, ok := stats[e.Path]; ok {
			e.Adds, e.Dels, e.Binary = st.Adds, st.Dels, st.Binary
			continue
		}
		// 未命中 numstat 的新增 = untracked 文件：按文件行数计 adds
		if current.Type == SourceTypeWorktree && e.Status == "added" {
			adds, binary := countFileLines(filepath.Join(current.Id, e.Path))
			e.Adds, e.Binary = adds, binary
		}
	}
}

// mergeChangeRenames 用 numstat 的 rename 检测（-M）把 fs 对比拆出的
// added+deleted 两条合并为 renamed 一条（fs 模式按路径对齐 blob sha，天然无 rename 概念）。
// git 模式的 DiffFiles 本就带 rename，无 added/deleted 对可合并，此函数自然为空操作。
func mergeChangeRenames(res *DiffTreesResult, stats map[string]git.NumstatEntry) {
	oldToNew := make(map[string]string)
	newToOld := make(map[string]string)
	for _, st := range stats {
		if st.OldPath != "" {
			oldToNew[st.OldPath] = st.Path
			newToOld[st.Path] = st.OldPath
		}
	}
	if len(oldToNew) == 0 {
		return
	}
	merged := make([]DiffEntry, 0, len(res.List))
	for _, e := range res.List {
		if e.Status == "deleted" {
			if _, ok := oldToNew[e.Path]; ok {
				continue // 旧路径由 renamed 条目吸收
			}
		}
		if e.Status == "added" {
			if old, ok := newToOld[e.Path]; ok {
				e.Status, e.OldPath = "renamed", old
			}
		}
		merged = append(merged, e)
	}
	res.List = merged
}

// countFileLines 统计文件行数；读不了（已删除等）或疑似二进制（前 8KB 含 NUL）返回 0。
func countFileLines(absPath string) (lines int, binary bool) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return 0, false
	}
	probe := data
	if len(probe) > 8192 {
		probe = probe[:8192]
	}
	if bytes.IndexByte(probe, 0) >= 0 {
		return 0, true
	}
	return bytes.Count(data, []byte{'\n'}), false
}

// --- PTY 会话（提案 1014，server 停机时由 OnServerStop 收尾）---

// ServePty 处理一个 PTY WebSocket 连接。会话机制在 pty.go（servePtySession）；
// 这里负责入参校验与把 cancel 登记进注册表（server 停机时 StopPtySessions 广播）。
func (s *Service) ServePty(ctx context.Context, conn *websocket.Conn, dir string, cols, rows int) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("path 目录不可用: path=%s: %w", dir, err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer s.trackPty(cancel)()
	return servePtySession(ctx, conn, dir, cols, rows)
}

// StopPtySessions 向所有活跃 PTY 会话发取消（server 停机时调用），确保无孤儿 shell。
// MVP 用 Service 上的轻量注册表；会话数 = 浏览器页面数，量级极小。
func (s *Service) StopPtySessions() {
	s.ptyMu.Lock()
	defer s.ptyMu.Unlock()
	for _, cancel := range s.ptyCancels {
		cancel()
	}
	s.ptyCancels = map[int]context.CancelFunc{}
}

// trackPty 把会话 cancel 登记进注册表，返回的 untrack 供会话结束时 defer 注销。
func (s *Service) trackPty(cancel context.CancelFunc) (untrack func()) {
	s.ptyMu.Lock()
	s.ptySeq++
	id := s.ptySeq
	s.ptyCancels[id] = cancel
	s.ptyMu.Unlock()
	return func() {
		s.ptyMu.Lock()
		delete(s.ptyCancels, id)
		s.ptyMu.Unlock()
	}
}

// OnServerStop 实现 app 的 serverStopHook：server 停机时杀掉全部 PTY 子进程。
func (s *Service) OnServerStop() {
	s.StopPtySessions()
}
