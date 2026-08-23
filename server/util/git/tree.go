package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListFiles 返回 path 工作副本中 git 管理的全部文件路径（相对副本根，字典序）：
// tracked（index，含已暂存未提交的）+ 未跟踪且未被忽略的（--others --exclude-standard，
// git 原生扫工作区并应用忽略链）。被 .gitignore/全局忽略的文件不出现。
// 已从工作区删除但未暂存删除的文件仍在 index 中，会保留为条目（git status 视角它仍受管）。
// 非仓库目录返回 (nil, nil)，不视为错误。
func ListFiles(path string) ([]string, error) {
	if !isGitRepo(path) {
		return nil, nil
	}
	out, err := runOut(path, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, fmt.Errorf("git ls-files 执行失败: %w", err)
	}
	var files []string
	for _, f := range strings.Split(out, "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// ListFilesUnder 返回 dir（可以是仓库内任意子目录）下全部非忽略文件的绝对路径：
// 在 dir 下执行 ls-files，cwd 相对路径天然限定在 dir 子树内，而 --exclude-standard
// 的忽略链（git 根 .gitignore / 祖先各层 / 全局 / .git/info/exclude）整条生效——
// 仓库根的 .gitignore 对子目录同样过滤。已删除但仍在 index 的条目按工作区实际
// 存在过滤掉。dir 不在任意仓库内、或 dir 自身被忽略链忽略时返回 (nil, nil)
// （调用方据此降级普通遍历），不视为错误。
func ListFilesUnder(dir string) ([]string, error) {
	if _, ok := FindGitRoot(dir); !ok {
		return nil, nil
	}
	out, err := runOut(dir, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, fmt.Errorf("git ls-files 执行失败: %w", err)
	}
	// 目录自身就被祖先 .gitignore 忽略时 ls-files 输出为空（如仓库内被忽略的
	// 构建产物目录）——返回 nil 交上层降级普通遍历，而不是误判成「空目录」。
	if out == "" {
		if _, ignErr := runOut(dir, "check-ignore", "--quiet", "--", "."); ignErr == nil {
			return nil, nil
		}
		return []string{}, nil
	}
	var files []string
	for _, f := range strings.Split(out, "\x00") {
		if f == "" {
			continue
		}
		abs := filepath.Join(dir, f)
		if info, err := os.Stat(abs); err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, abs)
	}
	return files, nil
}

// FileShasAtRef 返回 ref 下全部文件的「相对路径 → blob sha」递归平铺映射。
// 供目录级内容对比：两侧各取一份，按路径对齐、按 sha 比内容——两侧只要有一方
// 是 worktree（sha 需现场算）也能对上，因为 blob sha = sha1("blob <len>\0" + 内容)。
// ref 不存在时返回错误。
func FileShasAtRef(dir string, ref string) (map[string]string, error) {
	out, err := runOut(dir, "ls-tree", "-r", "-z", "--", ref)
	if err != nil {
		return nil, fmt.Errorf("git ls-tree -r 执行失败: %w", err)
	}
	return parseLsTreeBlobMap(out), nil
}

// parseLsTreeBlobMap 解析 `git ls-tree -r -z` 输出（无 -l）：`<mode> <type> <object>\t<name>\0`。
// 只收 blob（文件），tree 项被 -r 展开为子路径后不再出现。
func parseLsTreeBlobMap(out string) map[string]string {
	result := map[string]string{}
	for _, rec := range strings.Split(out, "\x00") {
		if rec == "" {
			continue
		}
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
	return result
}

// ReadFileAtRef 读 ref 下 file（相对仓库根）的原始字节（二进制安全）。
// 文件不存在时 git 报错原样返回（含 stderr 摘要）。
func ReadFileAtRef(dir string, ref string, file string) ([]byte, error) {
	out, err := runOut(dir, "show", ref+":"+file)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

// ExistsAtRef 判断 ref 下是否存在路径 file（git show 对缺失路径会以 exit 128 报错，
// cat-file -e 缺失时 exit 1 且无输出，适合做存在性探测）。
func ExistsAtRef(dir string, ref string, file string) bool {
	_, err := runOut(dir, "cat-file", "-e", ref+":"+file)
	return err == nil
}
