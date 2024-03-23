package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type WorkspaceConfig struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	MaxDepth int    `json:"maxDepth"`
}

type HubConfig struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Host string `json:"host"`

	DefaultPath string `json:"defaultPath"`
}

type ApplicationConfig struct {
	Name string `json:"name"`
	Bin  string `json:"bin"`
}

type Config struct {
	Workspaces   []WorkspaceConfig `json:"workspaces"`
	Repositories struct {
		Hubs []HubConfig `json:"hubs"`
	} `json:"repositories"`
	Applications []ApplicationConfig `json:"applications"`
}

func ParseConfigFile(cfgFile string, cfg *Config) error {
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

// default

var defaultConf Config

func Default() Config { return defaultConf }

func InitConfig(cfgFile string) error {
	if len(cfgFile) == 0 {
		cfgFile = "~/.go-cube/config.json"
	}

	if IsDebug() {
		log.Println("cfgFile = " + cfgFile)
	}

	cfgFile = RealPath(cfgFile)
	return ParseConfigFile(cfgFile, &defaultConf)
}

func RealPath(path string) string {
	// 支持 ~ 前缀
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(os.Getenv("HOME"), path[2:])
	}
	return path
}
