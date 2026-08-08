package project

import (
	"path/filepath"

	"cube/project/gitcache"
)

type GitInfo = gitcache.Entry

type Project struct {
	path    string   // 项目路径，唯一标识
	group   string   // 所属工作区名
	name    string   // 项目展示名，格式 `{组名}:{组内相对路径}`
	tags    []string // 标签列表
	gitInfo *GitInfo // git 信息
}

func newProject(r ScanRule, path string, tags []string) *Project {
	// 尝试使用相对工作区路径作为项目名；若整个工作区即为当前项目，则直接使用工作区名
	subName, _ := filepath.Rel(r.Path, path)
	if subName == "." {
		subName = r.Group
	}

	// 构建项目数据
	return &Project{
		path:  path,
		group: r.Group,
		name:  r.Group + ":" + subName,
		tags:  tags,
	}
}

func (p *Project) Group() string  { return p.group }
func (p *Project) Name() string   { return p.name }
func (p *Project) Path() string   { return p.path }
func (p *Project) Tags() []string { return p.tags }

func (p *Project) GitInfo() *GitInfo { return p.gitInfo }
func (p *Project) RepoUrl() string {
	if p.gitInfo == nil {
		return ""
	}
	return p.gitInfo.RepoUrl
}
