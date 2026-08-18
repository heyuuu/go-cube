package git

// diff 相关读操作：目录级变更列举（name-status）与文件级行 diff（--no-index）。
// 与 run.go 的约定一致：只解析机器可读输出，环境注入走 runOut。

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// DiffFile 两个 ref 之间变更的单个文件。
type DiffFile struct {
	Code    string // 状态码（含相似度后缀，如 "R100"）：A 新增 / D 删除 / M 修改 / R 重命名 / C 复制
	Path    string // 新侧路径（rename/copy 的目标路径）
	OldPath string // 仅 rename/copy 有值：源路径
}

// DiffFiles 列两个 tree-ish（分支/tag/commit）之间变更的文件清单，按路径排序。
// 带 -M 启用 rename 检测。路径分隔用 -z，含特殊字符的路径无需反转义。
func DiffFiles(dir string, left string, right string) ([]DiffFile, error) {
	// 末尾的 -- 必须放在两个 ref 之后：若夹在中间会把后面的 ref 当成 pathspec；
	// ref 形如 sha 时 git 无法自行消歧
	out, err := runOut(dir, "diff", "--name-status", "-z", "-M", left, right, "--")
	if err != nil {
		return nil, fmt.Errorf("git diff 执行失败: %w", err)
	}
	return parseDiffNameStatus(out), nil
}

// parseDiffNameStatus 解析 `git diff --name-status -z` 输出：NUL 分隔的字段流。
// 普通状态为 "状态码\0路径"，rename/copy 为 "状态码\0旧路径\0新路径"（三段连续）。
func parseDiffNameStatus(out string) []DiffFile {
	parts := strings.Split(out, "\x00")
	var files []DiffFile
	for i := 0; i < len(parts); i++ {
		status := parts[i]
		if status == "" {
			continue
		}
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			if i+2 < len(parts) {
				files = append(files, DiffFile{Code: status, OldPath: parts[i+1], Path: parts[i+2]})
				i += 2
				continue
			}
		}
		if i+1 < len(parts) {
			files = append(files, DiffFile{Code: status, Path: parts[i+1]})
			i++
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

// DiffNoIndex 比较两个文件（无需仓库上下文），返回 unified diff 文本（-U3）。
// 供调用方落临时文件后取行级差异，输出语义与 `git diff` 完全一致。
//
// git diff --no-index 在「有差异」时退出码为 1，这是正常语义而非错误；
// 只有 stdout 为空才算真失败（命令没跑起来 / 路径不存在），因此不走 runOut
// （runOut 会丢弃非零退出码进程的输出）。
func DiffNoIndex(a string, b string) (string, error) {
	cmd := exec.Command("git", "--no-optional-locks", "-c", "core.quotePath=false", "diff", "--no-index", "-U3", "--", a, b)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_PAGER=cat")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil && stdout.Len() == 0 {
		return "", fmt.Errorf("git diff --no-index 执行失败: %w；stderr: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
