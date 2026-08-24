package workbench

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"
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

	// 行级增删统计（仅 Changes 注入；DiffTrees 双树对比不填）。
	// untracked 新增按文件行数计 adds；二进制文件不统计
	Adds   int  `json:"adds"`
	Dels   int  `json:"dels"`
	Binary bool `json:"binary"`
}

// DiffTreesResult 目录级对比结果。Mode 显式返回实际使用的模式，前端不猜：
//   - git：两侧都是 commit/ref，走 git diff（快、语义准，但不含 untracked/ignored）
//   - fs：任一侧是 worktree（其语义就是「工作区当前状态」），走文件系统扫描对比
type DiffTreesResult struct {
	Mode string      `json:"mode"`
	List []DiffEntry `json:"list"`
}

// sourceFileMap 收集一个源的「相对路径 → blob sha」全量平铺（ignored 文件不在产品范围，恒排除）。
func sourceFileMap(root string, src TreeSource) (map[string]string, error) {
	switch src.Type {
	case SourceTypeWorktree:
		return lsFilesFileMap(src.Id)
	case SourceTypeCommit, SourceTypeRef:
		return git.FileShasAtRef(root, src.Id)
	default:
		return nil, fmt.Errorf("未知的 sourceType: %q", src.Type)
	}
}

// lsFilesFileMap 用 ls-files --cached --others --exclude-standard 圈定非忽略文件全集
// （tracked + 未跟踪未忽略，忽略链由 git 自带判定），再逐文件算 blob sha。
// 与 fsFileMap 的产物等价，但免去 status --ignored + 目录树 walk——忽略目录
// （如 node_modules）的存在会让那条路走到秒级，而 ls-files 对文件数是线性的。
// 磁盘上已不存在的 tracked 文件读失败跳过（与 fsFileMap 的 walk 语义一致：不在
// 磁盘上就不出现在工作区侧，diff 表现为 deleted）。
func lsFilesFileMap(wtDir string) (map[string]string, error) {
	files, err := git.ListFiles(wtDir)
	if err != nil {
		return nil, fmt.Errorf("读取工作副本文件清单失败: dir=%s: %w", wtDir, err)
	}
	result := make(map[string]string, len(files))
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(wtDir, f))
		if err != nil {
			continue // 权限等单点失败跳过，不阻断整体对比
		}
		result[f] = blobSha(data)
	}
	return result, nil
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
	case 'R', 'C': // copy 与 rename 同形（都有旧路径），前端词表里没有单独的 copied
		return "renamed"
	default:
		return "modified"
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

// readSide 取一个源下 file 的内容（worktree 走 fs，commit/ref 走 git show）。
// readSide 读取一侧文件内容；文件在该侧不存在时返回空内容而非错误——
// 新增/删除文件的对比天然是「一侧全文、一侧空白」，由行级 diff 自然呈现整体增删。
func readSide(root string, src TreeSource, file string) ([]byte, error) {
	if src.Type == SourceTypeWorktree {
		full, err := secureJoin(src.Id, file)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(full)
		if os.IsNotExist(err) {
			return nil, nil
		}
		return data, err
	}
	if !git.ExistsAtRef(root, src.Id, file) {
		return nil, nil
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
				// strings.Split 的行尾空串：真实空上下文行在 diff 输出中带前导空格（走 line[1:] 分支）
				continue
			}
			cur.Lines = append(cur.Lines, DiffLine{Kind: "ctx", Text: line[1:]})
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

// emptyTreeSha git 空树对象 sha（根提交没有父，与空树比即「全部为新增」）
const emptyTreeSha = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// diffTreesGit 两侧都是 tree-ish：走 git.DiffFiles（name-status + rename 检测），
// 状态字母映射为前端语义词。
func diffTreesGit(root string, base TreeSource, current TreeSource) (*DiffTreesResult, error) {
	files, err := git.DiffFiles(root, base.Id, current.Id)
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
// worktree 侧走 ls-files 清单 + 磁盘读；commit/ref 侧走 ls-tree -r（blob sha 现成）。
// 两个 sha 算法一致（blob sha = sha1("blob <len>\0" + content)），可直接比较。
// untracked 也是「工作区状态」的一部分，恒包含（不做减法）。
func diffTreesFs(root string, base TreeSource, current TreeSource) (*DiffTreesResult, error) {
	baseMap, err := sourceFileMap(root, base)
	if err != nil {
		return nil, fmt.Errorf("扫描基准侧失败: %w", err)
	}
	currentMap, err := sourceFileMap(root, current)
	if err != nil {
		return nil, fmt.Errorf("扫描当前侧失败: %w", err)
	}

	var list []DiffEntry
	for p, h := range baseMap {
		ch, ok := currentMap[p]
		switch {
		case !ok:
			list = append(list, DiffEntry{Path: p, Status: "deleted"})
		case ch != h:
			list = append(list, DiffEntry{Path: p, Status: "modified"})
		}
	}
	for p := range currentMap {
		if _, ok := baseMap[p]; !ok {
			list = append(list, DiffEntry{Path: p, Status: "added"})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Path < list[j].Path })
	return &DiffTreesResult{Mode: "fs", List: list}, nil
}

// readFileDiff 对比 base → current 两个源下某文件的行级差异。file 为当前侧（新侧）路径；
// rename 条目在基准侧的路径不同——baseFile 非空时基准侧按旧路径读，
// 未改内容的 rename 两侧字节相同，自然落进「内容一致」分支。
// 两侧内容经各自渠道取出（worktree 走 fs、tree-ish 走 git show），
// 行级 diff 用 git.DiffNoIndex（算法与展示语义和 git 完全一致），
// 内容落临时文件后比较，结束清理。
func readFileDiff(root string, base TreeSource, current TreeSource, file string, baseFile string) (*FileDiffResult, error) {
	if baseFile == "" {
		baseFile = file
	}
	baseData, err := readSide(root, base, baseFile)
	if err != nil {
		return nil, fmt.Errorf("读取基准侧文件失败: %w", err)
	}
	currentData, err := readSide(root, current, file)
	if err != nil {
		return nil, fmt.Errorf("读取当前侧文件失败: %w", err)
	}
	if bytes.Equal(baseData, currentData) {
		return &FileDiffResult{}, nil
	}
	if isBinary(baseData) || isBinary(currentData) {
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
	if _, err := tmpA.Write(baseData); err != nil {
		return nil, err
	}
	if _, err := tmpB.Write(currentData); err != nil {
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
