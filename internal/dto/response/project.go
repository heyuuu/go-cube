package response

type ProjectDto struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	RepoUrl string   `json:"repoUrl"`
	Tags    []string `json:"tags"`
}

type WorkspaceDto struct {
	Name       string   `json:"name"`
	Root       string   `json:"root"`
	PreferApps []string `json:"preferApps"`
	Scanner    any      `json:"scanner"`
}
