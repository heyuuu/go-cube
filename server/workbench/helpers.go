package workbench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
