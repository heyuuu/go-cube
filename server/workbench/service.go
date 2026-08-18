// Package workbench 是工作台领域包（提案 docs/proposals/1010-workbench基座）。
// 工作台以任意本机 git 目录为输入（不依赖 project scan、不走 gitcache），
// 信息全部直接调 git 获取（util/git 读能力），实时性由前端缓存控制。
// 分层注意：本包不 import project 包，git 读能力沉淀在 util/git。
package workbench

import (
	"bytes"
	"context"
	"cube/util/git"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/creack/pty"
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

// Info 读工作台项目信息：入口目录向上探测仓库根、现场发现全部工作副本、默认分支。
func (s *Service) Info(path string) (*Info, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	worktrees, err := git.WorktreeList(root)
	if err != nil {
		return nil, err
	}
	defaultBranch, _ := git.DefaultBranch(root) // 无 remote 返回空，可接受
	return &Info{
		Root:          root,
		Worktrees:     worktrees,
		DefaultBranch: defaultBranch,
	}, nil
}

// Refs 分支与 tag 列表，作为 git 树面板 / 双选交互的候选目标。
func (s *Service) Refs(path string) (*Refs, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	locals, current, err := git.Branches(root)
	if err != nil {
		return nil, err
	}
	remotes, _ := git.RemoteBranches(root) // 无远程分支返回空，可接受
	tags, _ := git.Tags(root)              // 无 tag 返回空，可接受
	return &Refs{
		Locals:  locals,
		Current: current,
		Remotes: remotes,
		Tags:    tags,
	}, nil
}

// Commits 拉取 commit 图一页。scope=all 走全部分支（--all，首屏拓扑全景），
// scope=ref 时按 ref 单线历史（大仓库首屏降级路径）。
func (s *Service) Commits(path string, scope string, ref string, cursor int, limit int) (*CommitsPageResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	if scope == "" {
		scope = "all"
	}
	if scope != "all" && scope != "ref" {
		return nil, fmt.Errorf("未知的 scope: %q（合法值 all/ref）", scope)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if cursor < 0 {
		cursor = 0
	}
	if scope == "ref" && ref == "" {
		// 单线模式没给 ref：退化为当前 HEAD（与 all 的区别仍是不带 --all）
		ref = "HEAD"
	}

	// 从头拉 cursor+limit 条再整体算 lane（保证跨页泳道一致），只返回本页切片
	all, err := git.CommitsPage(root, scope == "all", ref, 0, cursor+limit)
	if err != nil {
		return nil, err
	}
	nodes, wires := computeGraph(all)
	end := cursor + limit
	if end > len(nodes) {
		end = len(nodes)
	}
	page := nodes[cursor:end]

	// 本页 wires：上一页末行 → 本页首行的接续段（cursor-1 起）+ 本页内部各行段
	var pageWires []GraphWire
	fromRow := cursor - 1
	if fromRow < 0 {
		fromRow = 0
	}
	for _, w := range wires {
		if w.Row >= fromRow && w.Row < end-1 {
			pageWires = append(pageWires, w)
		}
	}

	return &CommitsPageResult{
		List:       page,
		Wires:      pageWires,
		NextCursor: cursor + limit,
		HasMore:    len(all) == cursor+limit,
	}, nil
}

// WorktreeStatus 单个工作副本的状态（提案 1011 状态区）。dir 为该工作副本目录
// （主目录或 linked worktree），不传时取仓库根。
func (s *Service) WorktreeStatus(path string, dir string) (*git.RepoStatus, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	if dir == "" {
		dir = root
	}
	st, err := git.LoadRepoStatus(dir)
	if err != nil {
		return nil, fmt.Errorf("读取工作副本状态失败: dir=%s: %w", dir, err)
	}
	return st, nil
}

// ServePty 处理一个 PTY WebSocket 连接：升级 → 起子进程 → 双向泵 → 任一端结束后清理。
// 退出路径（三方任一结束都触发整体清理）：
//   - 客户端断开（read pump 出错）→ cancel → 杀进程
//   - 子进程退出（ptmx Read io.EOF）→ 发 exit 帧 → 关连接
//   - server 关闭（ctx cancel）→ 杀进程
func (s *Service) ServePty(ctx context.Context, conn *websocket.Conn, dir string, cols, rows int) error {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("path 目录不可用: path=%s: %w", dir, err)
	}

	cmd := exec.Command(shell)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return fmt.Errorf("启动 shell 失败: %w", err)
	}
	defer func() {
		// 兜底 SIGKILL：正常退出路径已 wait，这里只处理异常残留；不留孤儿进程是硬要求
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer s.trackPty(cancel)()

	// 子进程退出信号（wait 只能调一次，由独立 goroutine 持有）
	procDone := make(chan error, 1)
	go func() { procDone <- cmd.Wait() }()

	// pty → 客户端 输出泵
	outputErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
				werr := conn.Write(wctx, websocket.MessageText,
					mustJSON(ptyMessage{Type: "output", Data: string(buf[:n])}))
				wcancel()
				if werr != nil {
					outputErr <- werr
					return
				}
			}
			if err != nil {
				outputErr <- err
				return
			}
		}
	}()

	// 客户端 → pty 输入泵（在调用方 goroutine 内同步读）
	readErr := make(chan error, 1)
	go func() {
		for {
			mt, data, err := conn.Read(ctx)
			if err != nil {
				readErr <- err
				return
			}
			if mt != websocket.MessageText {
				continue
			}
			var msg ptyMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "input":
				_, _ = ptmx.Write([]byte(msg.Data))
			case "resize":
				_ = pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(msg.Cols), Rows: uint16(msg.Rows)})
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-readErr:
		// 客户端断开（含页面关闭）：杀进程退出
		slog.Debug("pty 客户端断开", "dir", dir, "err", err)
		return nil
	case err := <-outputErr:
		// pty 输出结束 = 子进程退出：发 exit 帧后收尾
		var exitCode int
		if waitErr := <-procDone; waitErr != nil {
			var exitErr *exec.ExitError
			if errors.As(waitErr, &exitErr) {
				exitCode = exitErr.ExitCode()
			}
		}
		wctx, wcancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = conn.Write(wctx, websocket.MessageText,
			mustJSON(ptyMessage{Type: "exit", Code: exitCode}))
		wcancel()
		_ = conn.Close(websocket.StatusNormalClosure, "process exited")
		_ = err
		return nil
	}
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

// Tree 列某 TreeSource 下 subDir（相对该源根，空 = 根）的一层子项。
// showIgnored 仅对 worktree 源生效：false（默认）时忽略项不返回；true 时返回并标记。
func (s *Service) Tree(path string, src TreeSource, subDir string, showIgnored bool) ([]TreeEntry, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	switch src.Type {
	case SourceTypeWorktree:
		return s.treeFs(src.Id, subDir, showIgnored)
	case SourceTypeCommit, SourceTypeRef:
		entries, err := git.ListTreeAtRef(root, src.Id, subDir)
		if err != nil {
			return nil, fmt.Errorf("读取 %s 下的树失败: dir=%s: %w", src.Id, subDir, err)
		}
		return toTreeEntries(entries), nil
	default:
		return nil, fmt.Errorf("未知的 sourceType: %q", src.Type)
	}
}

// treeFs 读工作副本目录的真实文件树：fs 遍历 + git 忽略规则过滤（git.LoadIgnored）。
func (s *Service) treeFs(wtDir string, subDir string, showIgnored bool) ([]TreeEntry, error) {
	base, err := secureJoin(wtDir, subDir)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: dir=%s: %w", base, err)
	}

	ig := loadIgnoredDegrade(wtDir, subDir)

	var entries []TreeEntry
	for _, item := range items {
		if item.Name() == ".git" {
			continue
		}
		rel := item.Name()
		if subDir != "" {
			rel = subDir + "/" + item.Name()
		}
		if ig.Has(rel) && !showIgnored {
			continue
		}
		size := int64(0)
		if !item.IsDir() {
			if info, err := item.Info(); err == nil {
				size = info.Size()
			}
		}
		entries = append(entries, TreeEntry{
			Name:    item.Name(),
			Dir:     item.IsDir(),
			Size:    size,
			Ignored: ig.Has(rel),
		})
	}
	// 排序与虚拟树（ls-tree）一致：目录在前，目录/文件各自按字典序
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// ReadFile 读某 TreeSource 下 file 的内容。二进制检测：前 8KB 含 NUL 判为二进制。
func (s *Service) ReadFile(path string, src TreeSource, file string) (*FileResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	var data []byte
	switch src.Type {
	case SourceTypeWorktree:
		full, err := secureJoin(src.Id, file)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(full)
		if err != nil {
			return nil, fmt.Errorf("读取文件失败: file=%s: %w", file, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("目标是目录而非文件: file=%s", file)
		}
		if info.Size() > maxFileBytes {
			return nil, fmt.Errorf("文件过大（超过 2MB）: file=%s", file)
		}
		data, err = os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("读取文件失败: file=%s: %w", file, err)
		}
	case SourceTypeCommit, SourceTypeRef:
		var err error
		data, err = git.ReadFileAtRef(root, src.Id, file)
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxFileBytes {
			return nil, fmt.Errorf("文件过大（超过 2MB）: file=%s", file)
		}
	default:
		return nil, fmt.Errorf("未知的 sourceType: %q", src.Type)
	}

	binary := isBinary(data)
	content := ""
	if !binary {
		content = string(data)
	}
	return &FileResult{Content: content, Binary: binary, Size: int64(len(data))}, nil
}

// SaveFile 写工作副本文件（提案 1012 唯一落盘写路径）：只允许 worktree 源，
// 不做任何 git 操作；内容未变化时跳过写。
func (s *Service) SaveFile(path string, src TreeSource, file string, content string) (*FileResult, error) {
	if src.Type != SourceTypeWorktree {
		return nil, errors.New("只有 worktree 源（真实文件树）可以编辑保存")
	}
	if _, ok := git.FindGitRoot(path); !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	full, err := secureJoin(src.Id, file)
	if err != nil {
		return nil, err
	}
	data := []byte(content)
	if old, err := os.ReadFile(full); err == nil && bytesEqual(old, data) {
		return &FileResult{Size: int64(len(data))}, nil
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return nil, fmt.Errorf("写文件失败: file=%s: %w", file, err)
	}
	return &FileResult{Size: int64(len(data))}, nil
}

// DiffTrees 对比两个 TreeSource 的目录树。
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
		return s.diffTreesFs(root, left, right, showIgnored, showUntracked, statusFilter, pathPrefix)
	}
	result, err := s.diffTreesGit(root, left, right)
	if err != nil {
		return nil, err
	}
	if showIgnored || showUntracked {
		result.IgnoredFilters = append(result.IgnoredFilters, "showIgnored/showUntracked（git 模式下恒隐藏）")
	}
	applyDiffFilters(&result.List, statusFilter, pathPrefix)
	return result, nil
}

// diffTreesGit 两侧都是 tree-ish：走 git.DiffFiles（name-status + rename 检测），
// 状态字母映射为前端语义词。
func (s *Service) diffTreesGit(root string, left TreeSource, right TreeSource) (*DiffTreesResult, error) {
	files, err := git.DiffFiles(root, left.Id, right.Id)
	if err != nil {
		return nil, err
	}
	list := make([]DiffEntry, len(files))
	for i, f := range files {
		list[i] = DiffEntry{Path: f.Path, OldPath: f.OldPath, Status: letterToStatus(f.Code[0])}
	}
	return &DiffTreesResult{Mode: "git", List: list}, nil
}

// diffTreesFs 文件系统层扫描对比（Beyond Compare 模式）：
// 两侧各收集「路径 → 内容指纹（git blob sha1）」，按路径对齐。
// worktree 侧走 fs walk；commit/ref 侧走 ls-tree -r（blob sha 现成）。
// 两个 sha 算法一致（blob sha = sha1("blob <len>\0" + content)），可直接比较。
func (s *Service) diffTreesFs(
	root string, left TreeSource, right TreeSource,
	showIgnored bool, showUntracked bool, statusFilter string, pathPrefix string,
) (*DiffTreesResult, error) {
	leftMap, err := sourceFileMap(root, left, showIgnored)
	if err != nil {
		return nil, fmt.Errorf("扫描左侧失败: %w", err)
	}
	rightMap, err := sourceFileMap(root, right, showIgnored)
	if err != nil {
		return nil, fmt.Errorf("扫描右侧失败: %w", err)
	}

	var list []DiffEntry
	for p, h := range leftMap {
		rh, ok := rightMap[p]
		switch {
		case !ok:
			list = append(list, DiffEntry{Path: p, Status: "deleted"})
		case rh != h:
			list = append(list, DiffEntry{Path: p, Status: "modified"})
		}
	}
	for p := range rightMap {
		if _, ok := leftMap[p]; !ok {
			list = append(list, DiffEntry{Path: p, Status: "added"})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Path < list[j].Path })
	result := &DiffTreesResult{Mode: "fs", List: list}
	if !showUntracked {
		// fs 模式只过滤 untracked 时按两侧 worktree 的 index 判定成本高，MVP 不做减法，
		// 返回全集（untracked 也是「工作区状态」的一部分）；显式告知前端该筛选未生效
		result.IgnoredFilters = append(result.IgnoredFilters, "showUntracked=false（fs 模式下恒包含 untracked）")
	}
	applyDiffFilters(&result.List, statusFilter, pathPrefix)
	return result, nil
}

// ReadFileDiff 对比两个源下同一相对路径的文件。
// 两侧内容经各自渠道取出（worktree 走 fs、tree-ish 走 git show），
// 行级 diff 用 git.DiffNoIndex（算法与展示语义和 git 完全一致），
// 内容落临时文件后比较，结束清理。
func (s *Service) ReadFileDiff(path string, left TreeSource, right TreeSource, file string) (*FileDiffResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	leftData, err := readSide(root, left, file)
	if err != nil {
		return nil, fmt.Errorf("读取左侧文件失败: %w", err)
	}
	rightData, err := readSide(root, right, file)
	if err != nil {
		return nil, fmt.Errorf("读取右侧文件失败: %w", err)
	}
	if bytes.Equal(leftData, rightData) {
		return &FileDiffResult{}, nil
	}
	if isBinary(leftData) || isBinary(rightData) {
		return &FileDiffResult{Binary: true}, nil
	}

	tmpA, err := os.CreateTemp("", "cube-diff-a-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpA.Name())
	tmpB, err := os.CreateTemp("", "cube-diff-b-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpB.Name())
	if _, err := tmpA.Write(leftData); err != nil {
		return nil, err
	}
	if _, err := tmpB.Write(rightData); err != nil {
		return nil, err
	}
	tmpA.Close()
	tmpB.Close()

	out, err := git.DiffNoIndex(tmpA.Name(), tmpB.Name())
	if err != nil {
		return nil, err
	}
	return &FileDiffResult{Hunks: parseUnifiedDiff(out)}, nil
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
	return s.DiffTrees(path, TreeSource{Type: SourceTypeCommit, Id: base}, src, false, false, "", "")
}
