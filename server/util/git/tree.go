package git

import (
	"fmt"
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
