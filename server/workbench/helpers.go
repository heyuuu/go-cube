package workbench

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cube/util/git"
)

// secureJoin 把 rel 拼进 base 并校验不逃逸（防 file 参数越出目标目录读任意文件）。
func secureJoin(base string, rel string) (string, error) {
	// 含 .. 段的相对路径直接拒绝：归一化虽能把它困在 base 内（../.. 被清洗成
	// base 下的普通路径），但「企图越界」与「恰好不存在的文件」语义不同——
	// 文件缺失如今返回 deleted 标记而非错误，逃逸企图必须仍显式报错
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return "", fmt.Errorf("路径越界: %s", rel)
		}
	}
	clean := filepath.Clean("/" + rel) // 归一化，消灭 ../ 与开头 /
	full := filepath.Join(base, clean)
	if !strings.HasPrefix(full, filepath.Clean(base)+string(os.PathSeparator)) && full != filepath.Clean(base) {
		return "", fmt.Errorf("路径越界: %s", rel)
	}
	return full, nil
}

func isBinary(data []byte) bool {
	limit := min(len(data), 8192) // 只看头部即可粗判
	return bytes.IndexByte(data[:limit], 0) >= 0
}

// repoRoot Service 各读写方法共用的入口守卫：向上探测 git 仓库根，非 git 目录统一报错。
func repoRoot(path string) (string, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return "", fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	return root, nil
}
