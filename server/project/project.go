package project

import "path/filepath"

type Project struct {
	path  string   // 项目路径，唯一标识
	group string   // 所属工作区名
	name  string   // 项目展示名，格式 `{组名}:{组内相对路径}`
	tags  []string // 标签列表
}

func newProject(r ScanRule, path string, tags []string) *Project {
	// 构建项目数据
	return &Project{
		path:  path,
		group: r.Group,
		name:  projectName(r, path),
		tags:  tags,
	}
}

// projectName 计算项目展示名 `{组名}:{组内相对路径}`。
// 若整个工作区即为当前项目（path 即规则根），则直接使用工作区名。
func projectName(r ScanRule, path string) string {
	subName, _ := filepath.Rel(r.Path, path)
	if subName == "." {
		subName = r.Group
	}
	return r.Group + ":" + subName
}

func (p *Project) Group() string  { return p.group }
func (p *Project) Name() string   { return p.name }
func (p *Project) Path() string   { return p.path }
func (p *Project) Tags() []string { return p.tags }
