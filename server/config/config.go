package config

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"cube/util/pathkit"
)

type Config struct {
	DataDir string       `json:"dataDir"` // 数据目录
	Log     LogConfig    `json:"log"`
	Server  ServerConfig `json:"server"`
	Create  CreateConfig `json:"create"`
}

type CreateConfig struct {
	TemplateSource string `json:"templateSource"` // cube create 未显式传 --tpl 时的默认模板来源（本地目录或 git url）
}

type LogConfig struct {
	Path   string `json:"path"`
	Level  string `json:"level"`
	Format string `json:"format"`
}

type ServerConfig struct {
	Port int `json:"port"`
}

// Load 从 path 读取 JSON 配置。
// path 通常来自 -c 命令行参数（或默认值 ~/.config/cube/config.json），
// 故按命令行输入解析：支持 ~ 前缀与基于 cwd 的相对路径。
func Load(path string) (*Config, error) {
	absPath, err := pathkit.AbsPath(path)
	if err != nil {
		return nil, fmt.Errorf("解析配置路径失败: path=%s err=%w", path, err)
	}

	cfg := &Config{}

	raw, err := os.ReadFile(absPath)
	if err != nil {
		// 文件不存在（或权限等不可读问题）→ 降级为默认值，不报错。
		if os.IsNotExist(err) {
			return applyDefaults(cfg, absPath), nil
		}
		return nil, fmt.Errorf("读取配置文件失败: path=%s err=%w", absPath, err)
	}

	// 反序列化
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: path=%s err=%w", absPath, err)
	}

	return applyDefaults(cfg, absPath), nil
}

func applyDefaults(cfg *Config, path string) *Config {
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Dir(path)
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Path == "" {
		cfg.Log.Path = cfg.DataDir
	}
	return cfg
}

// Save 把 cfg 原子写入 path：先写临时文件（同目录，随机后缀），再 os.Rename 替换。
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建配置父目录失败: dir=%s err=%w", filepath.Dir(path), err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 config 失败: %w", err)
	}
	data = append(data, '\n')
	// 临时文件同目录（保证 rename 同文件系统原子），带随机后缀避免并发写互相覆盖。
	tmp := fmt.Sprintf("%s.tmp.%d", path, rand.Int31())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("写临时配置文件失败: tmp=%s err=%w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp) // 清理临时文件（best-effort）
		return fmt.Errorf("替换配置文件失败: path=%s err=%w", path, err)
	}
	return nil
}
