package workbench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
