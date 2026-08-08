package app

import (
	"fmt"
	"os"
	"path/filepath"

	"cube/util/pathkit"
)

// 子项相对根的文件/目录名。
const (
	dbFileName   = "data.db"
	cacheDirName = "cache"
)

// Paths cube 持有的数据目录的路径
// 零值不可用，必须用 New() 构造。
type Paths struct {
	dataDir string // 展开后的绝对路径（已 MkdirAll）
}

func NewPaths(dataDir string) *Paths {
	dataDir = pathkit.RealPath(dataDir)
	if dataDir == "" {
		panic(fmt.Errorf("数据目录不可为空: dir=%s", dataDir))
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		panic(fmt.Errorf("创建数据目录失败: dir=%s err=%w", dataDir, err))
	}
	return &Paths{dataDir: dataDir}
}

func (p *Paths) DataDir() string    { return p.dataDir }
func (p *Paths) DataDbFile() string { return filepath.Join(p.dataDir, dbFileName) }
func (p *Paths) CacheDir() string   { return filepath.Join(p.dataDir, cacheDirName) }
