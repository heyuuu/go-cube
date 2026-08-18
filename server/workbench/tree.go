package workbench

import (
	"errors"
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

// treeFs 读工作副本目录的真实文件树：fs 遍历 + git 忽略规则过滤。
// 忽略判定用 `git ls-files --others --ignored --exclude-standard --directory`：
// 它列出的正是「被忽略且不在索引中」的路径，与 git status 的忽略口径一致。
func (s *Service) treeFs(wtDir string, subDir string, showIgnored bool) ([]TreeEntry, error) {
	base, err := secureJoin(wtDir, subDir)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: dir=%s: %w", base, err)
	}

	ignoredSet, err := ignoredUnder(wtDir, subDir)
	if err != nil {
		return nil, err
	}

	var entries []TreeEntry
	for _, item := range items {
		if item.Name() == ".git" {
			continue
		}
		rel := item.Name()
		if subDir != "" {
			rel = subDir + "/" + item.Name()
		}
		if ignoredSet[rel] {
			if !showIgnored {
				continue
			}
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
			Ignored: ignoredSet[rel],
		})
	}
	return entries, nil
}

// ignoredUnder 返回 wtDir 下 subDir 内被 git 忽略的条目相对路径集合。
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
