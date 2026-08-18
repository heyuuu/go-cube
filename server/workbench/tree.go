package workbench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cube/util/git"
)

// TreeEntryResult 目录树接口的一层子项（worktree 走文件系统，commit/ref 走 ls-tree）。
type TreeEntry struct {
	Name    string `json:"name"`
	Dir     bool   `json:"dir"`
	Size    int64  `json:"size"`
	Ignored bool   `json:"ignored"` // 仅 worktree 源 + showIgnored 时有意义
}

// maxFileBytes 单文件读取上限（提案 1012：超大文件拒绝）
const maxFileBytes = 2 * 1024 * 1024

// Tree 列某 TreeSource 下 subDir（相对该源根，空 = 根）的一层子项。
// showIgnored 仅对 worktree 源生效：false（默认）时忽略项不返回；true 时返回并标记。
func ignoredUnder(wtDir string, subDir string) (map[string]bool, error) {
	args := []string{"ls-files", "--others", "--ignored", "--exclude-standard", "--directory"}
	if subDir != "" {
		args = append(args, "--", subDir)
	}
	cmdOut, err := git.RunRead(wtDir, args...)
	if err != nil {
		// 忽略判定失败不致命：降级为不过滤（宁可多显示，不可误隐藏）
		return map[string]bool{}, nil
	}
	set := map[string]bool{}
	for _, line := range strings.Split(cmdOut, "\n") {
		line = strings.TrimSuffix(strings.TrimRight(line, "\r"), "/")
		if line != "" {
			set[line] = true
		}
	}
	return set, nil
}

// FileResult 文件内容读取结果。
type FileResult struct {
	Content string `json:"content"` // 文本内容（binary=true 时为空）
	Binary  bool   `json:"binary"`  // 是否二进制
	Size    int64  `json:"size"`
}

// File 读某 TreeSource 下 file 的内容。二进制检测：前 8KB 含 NUL 判为二进制。
func secureJoin(base string, rel string) (string, error) {
	clean := filepath.Clean("/" + rel) // 归一化，消灭 ../ 与开头 /
	full := filepath.Join(base, clean)
	if !strings.HasPrefix(full, filepath.Clean(base)+string(os.PathSeparator)) && full != filepath.Clean(base) {
		return "", fmt.Errorf("路径越界: %s", rel)
	}
	return full, nil
}

func isBinary(data []byte) bool {
	limit := len(data)
	if limit > 8192 {
		limit = 8192
	}
	return bytesContains(data[:limit], 0)
}

func bytesContains(b []byte, target byte) bool {
	for _, v := range b {
		if v == target {
			return true
		}
	}
	return false
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func toTreeEntries(entries []git.TreeEntry) []TreeEntry {
	result := make([]TreeEntry, len(entries))
	for i, e := range entries {
		result[i] = TreeEntry{Name: e.Name, Dir: e.Dir, Size: e.Size}
	}
	return result
}
