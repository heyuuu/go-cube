package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/heyuuu/cube/util/pathkit"
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
