package response

type ProjectDto struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	RepoUrl string   `json:"repo_url"`
	Tags    []string `json:"tags"`
}

type WorkspaceDto struct {
	Name       string   `json:"name"`
	Root       string   `json:"root"`
	MaxDepth   int      `json:"max_depth"`
	PreferApps []string `json:"prefer_apps"`
}
