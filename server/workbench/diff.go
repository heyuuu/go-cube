package workbench

import (
	"crypto/sha1"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// sourceFileMap 收集一个源的「相对路径 → blob sha」全量平铺。
func sourceFileMap(root string, src TreeSource, includeIgnored bool) (map[string]string, error) {
	switch src.Type {
	case SourceTypeWorktree:
		return fsFileMap(src.Id, includeIgnored)
	case SourceTypeCommit, SourceTypeRef:
		return git.FileShasAtRef(root, src.Id)
	default:
		return nil, fmt.Errorf("未知的 sourceType: %q", src.Type)
	}
}

// fsFileMap walk 工作副本目录，跳过 .git；忽略项默认排除（includeIgnored=true 时保留）。
// 忽略判定：git.LoadIgnored 给出的忽略文件/目录集合（目录级命中即其下全部忽略）。
func fsFileMap(wtDir string, includeIgnored bool) (map[string]string, error) {
	ig := loadIgnoredDegrade(wtDir, "")
	result := map[string]string{}
	err := filepath.WalkDir(wtDir, func(p string, d fs.DirEntry, err error) error {
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
		if !includeIgnored && (ig.Files[rel] || underAnyDir(rel, ig.Dirs)) {
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

func underAnyDir(rel string, dirs map[string]bool) bool {
	for d := range dirs {
		if strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

// blobSha 计算 git blob 对象 sha（sha1("blob <len>\0" + 内容)），
// 与 git ls-tree 给出的 sha 同算法，两侧可直比。
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

// readSide 取一个源下 file 的内容（worktree 走 fs，commit/ref 走 git show）。
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

// emptyTreeSha git 空树对象 sha（根提交没有父，与空树比即「全部为新增」）
const emptyTreeSha = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
