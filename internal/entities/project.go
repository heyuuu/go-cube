package entities

type Project struct {
	name    string
	path    string
	repoUrl string
	tags    []string
}

func NewProject(name string, path string, tags []string) *Project {
	return &Project{
		name: name,
		path: path,
		tags: tags,
	}
}

func (p *Project) Name() string {
	return p.name
}

func (p *Project) Path() string {
	return p.path
}

func (p *Project) RepoUrl() string {
	return p.repoUrl
}

func (p *Project) Tags() []string {
	return p.tags
}
