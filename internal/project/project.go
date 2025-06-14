package project

import "github.com/heyuuu/go-cube/internal/repo"

type Project struct {
	name string
	path string
	repo repo.Repo
	tags []string
}

func NewProject(name string, path string, tags []string) *Project {
	return &Project{name: name, path: path}
}

func (t *Project) Name() string    { return t.name }
func (t *Project) Path() string    { return t.path }
func (t *Project) Repo() repo.Repo { return t.repo }

func (t *Project) RepoUrl() string {
	if t.repo == nil {
		return ""
	}
	return t.repo.Url()
}
