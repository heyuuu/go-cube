package config

import (
	"encoding/json"
	"fmt"
	"github.com/heyuuu/go-cube/internal/util/pathkit"
	"log/slog"
	"os"
	"path/filepath"
)

// 默认配置目录
const defaultCfgPath = "~/.go-cube/"

func InitConfig(cfgPath string) (err error) {
	if len(cfgPath) == 0 {
		cfgPath = defaultCfgPath
	}
	if IsDebug() {
		slog.Info("init config", "cfgPath", cfgPath)
	}
	cfgPath = pathkit.RealPath(cfgPath)

	// 初始化配置文件 config.json
	err = initDefaultConf(cfgPath)
	if err != nil {
		return err
	}

	return nil
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
		return fmt.Errorf("read config file failed: %w", err)
	}

	err = json.Unmarshal(data, cfg)
	if err != nil {
		return fmt.Errorf("unmarshal config data failed: %w", err)
	}

	return nil
}
