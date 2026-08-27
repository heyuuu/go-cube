package store

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic 原子写文件：tmp 文件 + rename。
// 目标目录不存在时自动递归创建（0755）。
// tmp 落同目录以保证与目标同文件系统（rename 原子性前提，跨文件系统会退化为 copy+unlink 的非原子写）；
// 中途失败会清理 tmp，目标文件要么是旧内容要么是完整新内容。
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: dir=%s err=%w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		if tmpPath != "" {
			tmp.Close()
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("设置临时文件权限失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("重命名临时文件失败: %w", err)
	}
	tmpPath = "" // rename 已接管 tmp，跳过 defer 清理
	return nil
}
