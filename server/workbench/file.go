package workbench

// 文件内容读写（提案 1012 代码阅读面板）：读走 fs / git show 双源，
// 写是唯一落盘路径（仅 worktree 源）。入口为 Service.ReadFile / SaveFile 的委托。

import (
	"errors"
	"fmt"
	"os"

	"cube/util/git"
)

// maxFileBytes 单文件读取上限（提案 1012：超大文件拒绝）
const maxFileBytes = 2 * 1024 * 1024

// FileResult 文件内容读取结果。
type FileResult struct {
	Content string `json:"content"` // 文本内容（binary=true 时为空）
	Binary  bool   `json:"binary"`  // 是否二进制
	Size    int64  `json:"size"`
	Deleted bool   `json:"deleted"` // 文件在该源中已删除（差异树的删除行可选中，内容区显示话术而非报错）
}

// readFile 读某 TreeSource 下 file 的内容。二进制检测：前 8KB 含 NUL 判为二进制。
func readFile(root string, src TreeSource, file string) (*FileResult, error) {
	var data []byte
	switch src.Type {
	case SourceTypeWorktree:
		full, err := secureJoin(src.Id, file)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(full)
		if err != nil {
			if os.IsNotExist(err) {
				// 差异树里的删除行可选中：文件已不在工作区，返回标记而非错误
				return &FileResult{Deleted: true}, nil
			}
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

// saveFile 写工作副本文件（提案 1012 唯一落盘写路径）：只允许 worktree 源，
// 不做任何 git 操作；内容未变化时跳过写。
func saveFile(path string, src TreeSource, file string, content string) (*FileResult, error) {
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
