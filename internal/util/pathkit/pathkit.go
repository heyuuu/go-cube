package pathkit

import (
	"os"
	"path/filepath"
	"strings"
)

func RealPath(path string) string {
	// 支持 ~ 前缀
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(os.Getenv("HOME"), path[2:])
	}
	return path
}
