package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// TreeEntry 某个 ref/commit 下的一层目录项（提案 1012 虚拟文件树）。
type TreeEntry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"` // blob 字节数（tree 为 0）
}

// ListTreeAtRef 列 ref（分支/tag/commit）下 dir（相对仓库根，空 = 根）的一层子项，按名字排序。
// dir 不存在时返回错误（git ls-tree 会静默输出空，因此先探测 ref:dir）。
func ListTreeAtRef(dir string, ref string, subDir string) ([]TreeEntry, error) {
	target := ref
	if subDir != "" {
		target = ref + ":" + subDir
	}
	out, err := runOut(dir, "ls-tree", "-z", "-l", "--", target)
	if err != nil {
		return nil, fmt.Errorf("git ls-tree 执行失败: %w", err)
	}
	return parseLsTree(out), nil
}

// parseLsTree 解析 `git ls-tree -z -l` 输出：`<mode> <type> <object> <size>\t<name>\0`。
// -z 时名字中的特殊字符不转义，size 字段 tree 项为 "-"。
func parseLsTree(out string) []TreeEntry {
	var entries []TreeEntry
	for _, rec := range strings.Split(out, "\x00") {
		if rec == "" {
			continue
		}
		tab := strings.IndexByte(rec, '\t')
		if tab < 0 {
			continue
		}
		meta, name := rec[:tab], rec[tab+1:]
		fields := strings.Fields(meta)
		if len(fields) < 4 {
			continue
		}
		size := int64(0)
		if fields[3] != "-" {
			fmt.Sscanf(fields[3], "%d", &size)
		}
		entries = append(entries, TreeEntry{Name: name, Dir: fields[1] == "tree", Size: size})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// ReadFileAtRef 读 ref 下 file（相对仓库根）的原始字节（二进制安全）。
// 文件不存在时 git 报错原样返回（含 stderr 摘要）。
func ReadFileAtRef(dir string, ref string, file string) ([]byte, error) {
	cmd := exec.Command("git", "--no-optional-locks", "-c", "core.quotePath=false", "-C", dir, "show", ref+":"+file)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_PAGER=cat")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		return nil, fmt.Errorf("git show %s:%s 失败: %w: %s", ref, file, err, msg)
	}
	return stdout.Bytes(), nil
}
