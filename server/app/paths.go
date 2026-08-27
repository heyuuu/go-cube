package app

import (
	"fmt"
	"os"
	"path/filepath"

	"cube/util/pathkit"
)

// 子项相对根的文件/目录名。
const (
	settingsFileName = "settings.json"
	stateDirName     = "state"
	usageFileName    = "usage.jsonl"
	cacheDirName     = "cache"
)

// Paths cube 持有的数据目录的路径
// 零值不可用，必须用 New() 构造。
type Paths struct {
	dataDir string // 展开后的绝对路径（已 MkdirAll）
}

func NewPaths(dataDir string) *Paths {
	// dataDir 来自 config.json，配置路径不得依赖执行目录（否则换个 cwd 运行数据就漂移），
	// 相对路径直接 panic 而非静默降级
	absDataDir, err := pathkit.StaticAbsPath(dataDir)
	if err != nil {
		panic(fmt.Errorf("数据目录路径无效: dir=%s err=%w", dataDir, err))
	}
	if err = os.MkdirAll(absDataDir, 0o755); err != nil {
		panic(fmt.Errorf("创建数据目录失败: dir=%s err=%w", absDataDir, err))
	}
	return &Paths{dataDir: absDataDir}
}

func (p *Paths) DataDir() string      { return p.dataDir }
func (p *Paths) SettingsFile() string { return filepath.Join(p.dataDir, settingsFileName) }

// StateDir 运行期状态目录（如 usage.jsonl）：由日常使用产生、非配置非缓存——
// 丢了可接受但不理想，区别于 cache/ 的「可整体删除且行为不变差」。
func (p *Paths) StateDir() string  { return filepath.Join(p.dataDir, stateDirName) }
func (p *Paths) UsageFile() string { return filepath.Join(p.dataDir, stateDirName, usageFileName) }

// CacheDir 纯缓存目录（如 git.json）：可整体删除且 cube 行为不变差，随时可重建。
func (p *Paths) CacheDir() string { return filepath.Join(p.dataDir, cacheDirName) }
