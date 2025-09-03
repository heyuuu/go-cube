package config

type Config struct {
	Workspaces   []WorkspaceConfig   `json:"workspaces"`
	Remotes      []RemoteConfig      `json:"remotes"`
	Applications []ApplicationConfig `json:"applications"`
}

type WorkspaceConfig struct {
	Code       string   `json:"code"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	MaxDepth   int      `json:"maxDepth"`
	PreferApps []string `json:"preferApps"`
}

type RemoteConfig struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Type string `json:"type"`
	Host string `json:"host"`

	DefaultPath string `json:"defaultPath"`
}

type ApplicationConfig struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Bin  string `json:"bin"`
}
