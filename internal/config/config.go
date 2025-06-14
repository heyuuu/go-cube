package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type WorkspaceConfig struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	MaxDepth   int      `json:"maxDepth"`
	PreferApps []string `json:"preferApps"`
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
