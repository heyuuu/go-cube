package project

type Project struct {
	Name       string
	Path       string
	GitRepoUrl string // todo
}

func NewProject(name string, path string) Project {
	return Project{Name: name, Path: path}
}
