package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DataDir string         `json:"dataDir"` // 数据目录
	Log     LogConfig      `json:"log"`
	Project ProjectConfig  `json:"project"`
	Openers []OpenerConfig `json:"openers"`
}

type LogConfig struct {
	Path   string `json:"path"`
	Level  string `json:"level"`
	Format string `json:"format"`
}

type ProjectConfig struct {
	Scan  []ScanRuleConfig  `json:"scan"`
	Clone []CloneRuleConfig `json:"clone"`
}

type ScanRuleConfig struct {
	Group    string `json:"group"`
	Path     string `json:"path"`
	MaxDepth int    `json:"maxDepth"`
}

type CloneRuleConfig struct {
	RepoHost   string `json:"repoHost"`
	RepoPrefix string `json:"repoPrefix"`
	LocalPath  string `json:"localPath"`
}

type OpenerConfig struct {
	Name  string   `json:"name"`
	Cmd   []string `json:"cmd"`   // 启动命令，cmd[0]=可执行文件，其余为参数；用 $0/$1... 占位路径槽位
	Roles []string `json:"roles"` // 该 opener 的业务用途枚举，如 ["open-dir"]、["diff-dir","diff-file"]；缺省视为 ["open-dir"]
}

// Load 从 path 读取 JSON 配置。
func Load(path string) (*Config, error) {
	cfg := &Config{}

	raw, err := os.ReadFile(path)
	if err != nil {
		// 文件不存在（或权限等不可读问题）→ 降级为默认值，不报错。
		if os.IsNotExist(err) {
			return applyDefaults(cfg, path), nil
		}
		return nil, fmt.Errorf("读取配置文件失败: path=%s err=%w", path, err)
	}

	// 反序列化
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: path=%s err=%w", path, err)
	}

	return applyDefaults(cfg, path), nil
}

func applyDefaults(cfg *Config, path string) *Config {
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Dir(path)
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Path == "" {
		cfg.Log.Path = filepath.Join(cfg.DataDir, "log.json")
	}
	return cfg
}
