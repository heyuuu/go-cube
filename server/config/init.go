package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"cube/util/pathkit"
)

// 默认配置目录
const defaultCfgPath = "~/.config/cube/"

func Init(cfgPath string) error {
	if len(cfgPath) == 0 {
		cfgPath = defaultCfgPath
	}
	cfgPath = pathkit.RealPath(cfgPath)

	// 若 cfgPath 不存在则创建
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.MkdirAll(cfgPath, 0755); err != nil {
			return fmt.Errorf("创建 config 目录失败: %w", err)
		}
	}

	// 记录日志目录
	configPath = cfgPath

	// 初始化配置文件 config.json
	return initDefaultConf(cfgPath)
}

// config path
var configPath string

func Path() string {
	return configPath
}

// config file (config.json)
var defaultConf Config

func Default() Config {
	return defaultConf
}

// DefaultPtr 返回 defaultConf 的指针，供需要修改配置的场景（如 web 写接口）使用。
// 注意：调用方修改后需调 Save() 才能持久化到 config.json。
func DefaultPtr() *Config {
	return &defaultConf
}

// ConfigFile 返回 config.json 的完整路径。
func ConfigFile() string {
	return filepath.Join(configPath, "config.json")
}

// Save 把当前 defaultConf 序列化写回 config.json（格式化 2 空格缩进，便于人工查看）。
func Save() error {
	data, err := json.MarshalIndent(defaultConf, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 config 失败: %w", err)
	}
	data = append(data, '\n')
	cfgFile := ConfigFile()
	tmp := cfgFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("写入 config 临时文件失败: %w", err)
	}
	if err := os.Rename(tmp, cfgFile); err != nil {
		return fmt.Errorf("重命名 config 临时文件失败: %w", err)
	}
	return nil
}

func initDefaultConf(cfgPath string) error {
	cfgFile := filepath.Join(cfgPath, "config.json")
	// 若配置文件不存在则跳过
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		return nil
	}
	return parseConfigFile(cfgFile, &defaultConf)
}

func parseConfigFile(cfgFile string, cfg *Config) error {
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return fmt.Errorf("读取 config 文件失败: %w", err)
	}

	err = json.Unmarshal(data, cfg)
	if err != nil {
		return fmt.Errorf("解析 config 数据失败: %w", err)
	}

	return nil
}
