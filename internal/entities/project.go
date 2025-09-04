package entities

type Project struct {
	name    string
	path    string
	repoUrl string
	tags    []string
}

func NewProject(name string, path string, tags []string) *Project {
	return &Project{name: name, path: path, tags: tags}
}

func (t *Project) Name() string { return t.name }
func (t *Project) Path() string { return t.path }
func (t *Project) RepoUrl() string {
	return t.repoUrl
}
func (t *Project) Tags() []string {
	return t.tags
}
