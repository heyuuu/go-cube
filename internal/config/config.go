package config

type Config struct {
	Workspaces   []WorkspaceConfig   `json:"workspaces"`
	Remotes      []RemoteConfig      `json:"remotes"`
	Applications []ApplicationConfig `json:"applications"`

	// 日志相关
	LogPath   string `json:"log_path"`
	LogLevel  string `json:"log_level"`
	LogFormat string `json:"log_format"`
}

type WorkspaceConfig struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	MaxDepth   int      `json:"maxDepth"`
	PreferApps []string `json:"preferApps"`
}

type RemoteConfig struct {
	Name string `json:"name"`
	Host string `json:"host"`

	DefaultPath string `json:"defaultPath"`
}

type ApplicationConfig struct {
	Name string `json:"name"`
	Bin  string `json:"bin"`
}
