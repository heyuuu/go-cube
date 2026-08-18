package workbench

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"cube/util/git"
)

// DiffEntry 目录级对比的单条变更。
type DiffEntry struct {
	Path    string `json:"path"`    // 新侧相对路径（rename 为新路径）
	OldPath string `json:"oldPath"` // rename 时的旧路径，其余为空
	Status  string `json:"status"`  // added / deleted / modified / renamed
}

// DiffTreesResult 目录级对比结果。Mode 显式返回实际使用的模式，前端不猜：
//   - git：两侧都是 commit/ref，走 git diff（快、语义准，但不含 untracked/ignored）
//   - fs：任一侧是 worktree（其语义就是「工作区当前状态」），走文件系统扫描对比
type DiffTreesResult struct {
	Mode           string      `json:"mode"`
	List           []DiffEntry `json:"list"`
	IgnoredFilters []string    `json:"ignoredFilters"` // 请求了但在该模式下无效的筛选项名
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

// diffTreesGit 两侧都是 tree-ish：`git diff --name-status -z -M`。
func (s *Service) diffTreesGit(root string, left TreeSource, right TreeSource) (*DiffTreesResult, error) {
	out, err := git.RunRead(root, "diff", "--name-status", "-z", "-M", "--", left.Id, right.Id)
	if err != nil {
		return nil, fmt.Errorf("git diff 执行失败: %w", err)
	}
	var list []DiffEntry
	parts := strings.Split(out, "\x00")
	for i := 0; i < len(parts); i++ {
		status := parts[i]
		if status == "" {
			continue
		}
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			// rename/copy：status\0旧路径\0新路径
			if i+2 < len(parts) {
				list = append(list, DiffEntry{Path: parts[i+2], OldPath: parts[i+1], Status: "renamed"})
				i += 2
				continue
			}
		}
		if i+1 < len(parts) {
			list = append(list, DiffEntry{Path: parts[i+1], Status: letterToStatus(status[0])})
			i++
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Path < list[j].Path })
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

// sourceFileMap 收集一个源的「相对路径 → blob sha」全量平铺。
func sourceFileMap(root string, src TreeSource, includeIgnored bool) (map[string]string, error) {
	switch src.Type {
	case SourceTypeWorktree:
		return fsFileMap(src.Id, includeIgnored)
	case SourceTypeCommit, SourceTypeRef:
		return gitFileMap(root, src.Id)
	default:
		return nil, fmt.Errorf("未知的 sourceType: %q", src.Type)
	}
}

// fsFileMap walk 工作副本目录，跳过 .git；忽略项默认排除（includeIgnored=true 时保留）。
// 忽略判定：`ls-files --others --ignored --exclude-standard --directory` 给出的
// 忽略文件/目录集合（目录级命中即其下全部忽略）。
func fsFileMap(wtDir string, includeIgnored bool) (map[string]string, error) {
	ignoredDirs, ignoredFiles, err := ignoredSets(wtDir)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	err = filepath.WalkDir(wtDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(wtDir, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !includeIgnored && (ignoredFiles[rel] || underAnyDir(rel, ignoredDirs)) {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil // 单文件读失败跳过（权限等），不阻断整体对比
		}
		result[rel] = blobSha(data)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描目录失败: dir=%s: %w", wtDir, err)
	}
	return result, nil
}

// gitFileMap `git ls-tree -r -z -l <ref>`：blob sha 现成，不用读内容。
func gitFileMap(root string, ref string) (map[string]string, error) {
	out, err := git.RunRead(root, "ls-tree", "-r", "-z", "--", ref)
	if err != nil {
		return nil, fmt.Errorf("git ls-tree -r 执行失败: %w", err)
	}
	result := map[string]string{}
	for _, rec := range strings.Split(out, "\x00") {
		tab := strings.IndexByte(rec, '\t')
		if tab < 0 {
			continue
		}
		fields := strings.Fields(rec[:tab])
		if len(fields) < 3 || fields[1] != "blob" {
			continue
		}
		result[rec[tab+1:]] = fields[2]
	}
	return result, nil
}

// ignoredSets 返回工作副本的忽略文件集合与忽略目录集合（相对路径）。
func ignoredSets(wtDir string) (dirs map[string]bool, files map[string]bool, err error) {
	dirs, files = map[string]bool{}, map[string]bool{}
	out, err := git.RunRead(wtDir, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory")
	if err != nil {
		return dirs, files, nil // 判定失败降级为不过滤
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, "/") {
			dirs[strings.TrimSuffix(line, "/")] = true
		} else {
			files[line] = true
		}
	}
	return dirs, files, nil
}

func underAnyDir(rel string, dirs map[string]bool) bool {
	for d := range dirs {
		if strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

func blobSha(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func letterToStatus(c byte) string {
	switch c {
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	default:
		return "modified"
	}
}

func applyDiffFilters(list *[]DiffEntry, statusFilter string, pathPrefix string) {
	if statusFilter != "" {
		want := map[string]bool{}
		for _, s := range strings.Split(statusFilter, ",") {
			if s != "" {
				want[strings.TrimSpace(s)] = true
			}
		}
		filtered := (*list)[:0]
		for _, e := range *list {
			if want[e.Status] {
				filtered = append(filtered, e)
			}
		}
		*list = filtered
	}
	if pathPrefix != "" {
		filtered := (*list)[:0]
		for _, e := range *list {
			if strings.HasPrefix(e.Path, pathPrefix) {
				filtered = append(filtered, e)
			}
		}
		*list = filtered
	}
}

// --- 文件级 diff ---

// DiffLine 单行变更。Kind：ctx（上下文）/ add（+）/ del（-）。
type DiffLine struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// Hunk 一个差异块（含双方起始行号与行数）。
type Hunk struct {
	OldStart int        `json:"oldStart"`
	OldCount int        `json:"oldCount"`
	NewStart int        `json:"newStart"`
	NewCount int        `json:"newCount"`
	Lines    []DiffLine `json:"lines"`
}

// FileDiffResult 单文件 diff。二进制文件只置 Binary 不带 hunks。
type FileDiffResult struct {
	Binary bool   `json:"binary"`
	Hunks  []Hunk `json:"hunks"`
}

// ReadFileDiff 对比两个源下同一相对路径的文件。
// 两侧内容经各自渠道取出（worktree 走 fs、tree-ish 走 git show），
// 行级 diff 用 `git diff --no-index`（算法与展示语义和 git 完全一致），
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

	out, err := runDiffNoIndex(tmpA.Name(), tmpB.Name())
	if err != nil {
		return nil, fmt.Errorf("git diff --no-index 执行失败: %w", err)
	}
	return &FileDiffResult{Hunks: parseUnifiedDiff(out)}, nil
}

func readSide(root string, src TreeSource, file string) ([]byte, error) {
	if src.Type == SourceTypeWorktree {
		full, err := secureJoin(src.Id, file)
		if err != nil {
			return nil, err
		}
		return os.ReadFile(full)
	}
	return git.ReadFileAtRef(root, src.Id, file)
}

// parseUnifiedDiff 解析 unified diff 输出（跳过 +++/--- 头，只取 @@ 与正文行）。
func parseUnifiedDiff(out string) []Hunk {
	var hunks []Hunk
	var cur *Hunk
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "@@") {
			os_, ns, oc, nc := parseHunkHeader(line)
			hunks = append(hunks, Hunk{OldStart: os_, NewStart: ns, OldCount: oc, NewCount: nc})
			cur = &hunks[len(hunks)-1]
			continue
		}
		if cur == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index "):
			continue
		case strings.HasPrefix(line, "+"):
			cur.Lines = append(cur.Lines, DiffLine{Kind: "add", Text: line[1:]})
		case strings.HasPrefix(line, "-"):
			cur.Lines = append(cur.Lines, DiffLine{Kind: "del", Text: line[1:]})
		case strings.HasPrefix(line, "\\"):
			continue // "\ No newline at end of file"
		default:
			if line == "" {
				// 末尾空行可能是上下文空行
				cur.Lines = append(cur.Lines, DiffLine{Kind: "ctx", Text: ""})
			} else {
				cur.Lines = append(cur.Lines, DiffLine{Kind: "ctx", Text: line[1:]})
			}
		}
	}
	return hunks
}

func parseHunkHeader(line string) (oldStart, newStart, oldCount, newCount int) {
	// @@ -oldStart,oldCount +newStart,newCount @@
	fields := strings.Fields(line)
	parse := func(f string) (int, int) {
		f = strings.TrimPrefix(strings.TrimPrefix(f, "-"), "+")
		parts := strings.Split(f, ",")
		start, _ := strconv.Atoi(parts[0])
		count := 1
		if len(parts) > 1 {
			count, _ = strconv.Atoi(parts[1])
		}
		return start, count
	}
	if len(fields) > 1 {
		oldStart, oldCount = parse(fields[1])
	}
	if len(fields) > 2 {
		newStart, newCount = parse(fields[2])
	}
	return
}

// runDiffNoIndex 比较两个临时文件。git diff --no-index 在有差异时退出码为 1，
// 输出仍然有效（runOut 会丢弃非零退出的 stdout），因此这里直接走 exec、只认「无输出」为失败。
func runDiffNoIndex(a, b string) (string, error) {
	cmd := exec.Command("git", "--no-optional-locks", "-c", "core.quotePath=false", "diff", "--no-index", "-U3", "--", a, b)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_PAGER=cat")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil && stdout.Len() == 0 {
		return "", fmt.Errorf("%w；stderr: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
