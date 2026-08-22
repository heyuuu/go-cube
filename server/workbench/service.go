// Package workbench 是工作台领域包（提案 docs/proposals/1010-workbench基座）。
// 工作台以任意本机 git 目录为输入（不依赖 project scan、不走 gitcache），
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
	"sync"

	"cube/util/git"
	"cube/util/slicekit"

	"github.com/coder/websocket"
)

// Service 工作台领域服务。git 读路径无状态（每次调用直接调 git）；
// 唯一的运行期状态是 PTY 会话注册表（server 停机时统一回收，见 pty.go）。
type Service struct {
	ptyMu      sync.Mutex
	ptySeq     int
	ptyCancels map[int]context.CancelFunc
}

func NewService() *Service {
	return &Service{ptyCancels: map[int]context.CancelFunc{}}
}

// --- git 面板（info / 分支与 tag / commit 日志 / 工作副本快照）---

// Info 读工作台项目信息：入口目录向上探测仓库根、默认分支。
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

// DiffTrees 对比两个 TreeSource 的目录树。实现在 diff.go（git 模式 / fs 扫描模式）。
// 筛选项：statusFilter（逗号分隔 added,deleted,modified,renamed）、pathPrefix；
// showIgnored / showUntracked 仅 fs 模式有效（git 模式下收进 IgnoredFilters）。
func (s *Service) DiffTrees(
	path string, left TreeSource, right TreeSource,
	showIgnored bool, showUntracked bool, statusFilter string, pathPrefix string,
) (*DiffTreesResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	if left.Type == SourceTypeWorktree || right.Type == SourceTypeWorktree {
		return diffTreesFs(root, left, right, showIgnored, showUntracked, statusFilter, pathPrefix)
	}
	result, err := diffTreesGit(root, left, right)
	if err != nil {
		return nil, err
	}
	if showIgnored || showUntracked {
		result.IgnoredFilters = append(result.IgnoredFilters, "showIgnored/showUntracked（git 模式下恒隐藏）")
	}
	applyDiffFilters(&result.List, statusFilter, pathPrefix)
	return result, nil
}

// ReadFileDiff 对比两个源下同一相对路径的文件。实现在 diff.go。
func (s *Service) ReadFileDiff(path string, left TreeSource, right TreeSource, file string) (*FileDiffResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	return readFileDiff(root, left, right, file)
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
	var base string
	switch src.Type {
	case SourceTypeWorktree:
		head, err := git.HeadSha(src.Id)
		if err != nil {
			return nil, fmt.Errorf("读取工作副本 HEAD 失败: dir=%s: %w", src.Id, err)
		}
		base = head
	case SourceTypeCommit, SourceTypeRef:
		parent, err := git.ParentSha(root, src.Id)
		if err != nil {
			// 根提交没有父：与空树比
			base = emptyTreeSha
		} else {
			base = parent
		}
	default:
		return nil, fmt.Errorf("未知的 sourceType: %q", src.Type)
	}
	res, err := s.DiffTrees(path, TreeSource{Type: SourceTypeCommit, Id: base}, src, false, false, "", "")
	if err != nil {
		return nil, err
	}
	annotateChangeStats(res, root, src, base)
	return res, nil
}

// annotateChangeStats 为 Changes 的变更清单注入行级增删统计（git.Numstat）：
//   - worktree 源：numstat 与工作区比（不含 untracked），未命中的 added 即 untracked，按文件行数计 adds；
//   - commit/ref 源：numstat 与父提交比，全部命中。
//
// 统计失败只降级（adds/dels 留零值），不阻断变更清单——文件列表本身仍可用。
func annotateChangeStats(res *DiffTreesResult, root string, src TreeSource, base string) {
	var stats map[string]git.NumstatEntry
	var err error
	if src.Type == SourceTypeWorktree {
		stats, err = git.Numstat(src.Id, base, "")
	} else {
		stats, err = git.Numstat(root, base, src.Id)
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
		// 未命中 numstat 的新增 = untracked 文件：按文件行数计 adds（二进制探测 NUL 字节）
		if src.Type == SourceTypeWorktree && e.Status == "added" {
			adds, binary := countFileLines(filepath.Join(src.Id, e.Path))
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
