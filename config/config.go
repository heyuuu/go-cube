package config

type Config struct {
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
