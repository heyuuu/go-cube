package workbench

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"cube/util/git"
)

// TreeEntry 目录树接口的一层子项（worktree 走文件系统，commit/ref 走 ls-tree）。
type TreeEntry struct {
	Name    string `json:"name"`
	Dir     bool   `json:"dir"`
	Size    int64  `json:"size"`
	Ignored bool   `json:"ignored"` // 仅 worktree 源 + showIgnored 时有意义
}

// maxFileBytes 单文件读取上限（提案 1012：超大文件拒绝）
const maxFileBytes = 2 * 1024 * 1024

// loadIgnoredDegrade 加载工作副本的忽略集合；判定失败不致命，降级为空集合
// （宁可多显示，不可误隐藏）。treeFs 与目录对比（fsFileMap）共用此降级策略。
func loadIgnoredDegrade(wtDir string, subDir string) *git.Ignored {
	ig, err := git.LoadIgnored(wtDir, subDir)
	if err != nil {
		slog.Debug("忽略判定失败，降级为不过滤", "dir", wtDir, "err", err)
		return &git.Ignored{Dirs: map[string]bool{}, Files: map[string]bool{}}
	}
	return ig
}

// FileResult 文件内容读取结果。
type FileResult struct {
	Content string `json:"content"` // 文本内容（binary=true 时为空）
	Binary  bool   `json:"binary"`  // 是否二进制
	Size    int64  `json:"size"`
}

// secureJoin 把 rel 拼进 base 并校验不逃逸（防 file 参数越出目标目录读任意文件）。
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
