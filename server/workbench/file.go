package workbench

// 文件内容读写（提案 1012 代码阅读面板）：读走 fs / git show 双源，
// 写是唯一落盘路径（仅 worktree 源）。入口为 Service.ReadFile / SaveFile 的委托。

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"cube/util/git"
)

// maxFileBytes 单文件读取上限（提案 1012：超大文件拒绝）
const maxFileBytes = 2 * 1024 * 1024

// maxRawFileBytes 原始字节读取上限（图片等二进制预览用；图片常超 2MB，放宽到 16MB）
const maxRawFileBytes = 16 * 1024 * 1024

// FileResult 文件内容读取结果。
type FileResult struct {
	Content string `json:"content"` // 文本内容（binary=true 时为空）
	Binary  bool   `json:"binary"`  // 是否二进制
	Size    int64  `json:"size"`
	Deleted bool   `json:"deleted"` // 文件在该源中已删除（差异树的删除行可选中，内容区显示话术而非报错）
}

// readFile 读某 TreeSource 下 file 的内容。二进制检测：前 8KB 含 NUL 判为二进制。
func readFile(root string, src TreeSource, file string) (*FileResult, error) {
	data, deleted, err := readFileData(root, src, file, maxFileBytes)
	if err != nil {
		return nil, err
	}
	if deleted {
		return &FileResult{Deleted: true}, nil
	}
	binary := isBinary(data)
	content := ""
	if !binary {
		content = string(data)
	}
	return &FileResult{Content: content, Binary: binary, Size: int64(len(data))}, nil
}

// readFileData 读某 TreeSource 下 file 的原始字节（文本读与 raw 预览共用）。
// deleted=true 表示文件在该源中已删除（差异树的删除行）。
func readFileData(root string, src TreeSource, file string, maxBytes int64) (data []byte, deleted bool, err error) {
	switch src.Type {
	case SourceTypeWorktree:
		full, err := secureJoin(src.Id, file)
		if err != nil {
			return nil, false, err
		}
		info, err := os.Stat(full)
		if err != nil {
			if os.IsNotExist(err) {
				// 差异树里的删除行可选中：文件已不在工作区，返回标记而非错误
				return nil, true, nil
			}
			return nil, false, fmt.Errorf("读取文件失败: file=%s: %w", file, err)
		}
		if info.IsDir() {
			return nil, false, fmt.Errorf("目标是目录而非文件: file=%s", file)
		}
		if info.Size() > maxBytes {
			return nil, false, fmt.Errorf("文件过大（超过 %dMB）: file=%s", maxBytes/1024/1024, file)
		}
		data, err = os.ReadFile(full)
		if err != nil {
			return nil, false, fmt.Errorf("读取文件失败: file=%s: %w", file, err)
		}
		return data, false, nil
	case SourceTypeCommit, SourceTypeRef:
		// 该提交里被删除的文件（差异树的删除行）：返回标记而非错误
		if !git.ExistsAtRef(root, src.Id, file) {
			return nil, true, nil
		}
		data, err := git.ReadFileAtRef(root, src.Id, file)
		if err != nil {
			return nil, false, err
		}
		if int64(len(data)) > maxBytes {
			return nil, false, fmt.Errorf("文件过大（超过 %dMB）: file=%s", maxBytes/1024/1024, file)
		}
		return data, false, nil
	default:
		return nil, false, fmt.Errorf("未知的 sourceType: %q", src.Type)
	}
}

// readFileRaw 读原始字节（图片等二进制预览）：不做二进制检测，直接返回字节。
func readFileRaw(root string, src TreeSource, file string) ([]byte, error) {
	data, deleted, err := readFileData(root, src, file, maxRawFileBytes)
	if err != nil {
		return nil, err
	}
	if deleted {
		return nil, fmt.Errorf("文件在该源中已删除: file=%s", file)
	}
	return data, nil
}

// saveFile 写工作副本文件（提案 1012 唯一落盘写路径）：只允许 worktree 源，
// 不做任何 git 操作；内容未变化时跳过写。
func saveFile(path string, src TreeSource, file string, content string) (*FileResult, error) {
	if src.Type != SourceTypeWorktree {
		return nil, errors.New("只有 worktree 源（真实文件树）可以编辑保存")
	}
	if _, err := repoRoot(path); err != nil {
		return nil, err
	}
	full, err := secureJoin(src.Id, file)
	if err != nil {
		return nil, err
	}
	data := []byte(content)
	if old, err := os.ReadFile(full); err == nil && bytes.Equal(old, data) {
		return &FileResult{Size: int64(len(data))}, nil
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return nil, fmt.Errorf("写文件失败: file=%s: %w", file, err)
	}
	return &FileResult{Size: int64(len(data))}, nil
}
