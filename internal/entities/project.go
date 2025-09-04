package entities

import "path/filepath"

type Project struct {
	workspaceName string // 所属工作区名
	name          string // 项目名，展示用
	path          string // 项目路径
	repoUrl       string
	tags          []string // 标签列表
}

func NewProject(ws *Workspace, path string, tags []string) *Project {
	// 尝试使用相对工作区路径作为项目名；若整个工作区即为当前项目，则直接使用工作区名
	subName, _ := filepath.Rel(ws.Path(), path)
	if subName == "." {
		subName = ws.Name()
	}

	// 构建项目数据
	return &Project{
		workspaceName: ws.Name(),
		name:          ws.Name() + ":" + subName,
		path:          path,
		tags:          tags,
	}
}

func (p *Project) WorkspaceName() string { return p.workspaceName }

func (p *Project) Name() string    { return p.name }
func (p *Project) Path() string    { return p.path }
func (p *Project) RepoUrl() string { return p.repoUrl }
func (p *Project) Tags() []string  { return p.tags }
