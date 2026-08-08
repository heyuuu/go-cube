package project

import (
	"fmt"

	"cube/project"
	"cube/util/tui"
)

// selectProject 按查询词匹配项目：0 个提示、1 个直接返回、多个交互选择。
// 供 list/info/open 等需要"定位单个项目"的命令复用。
func selectProject(service *project.Service, query string) *project.Project {
	projects := service.Search(query)
	switch len(projects) {
	case 0:
		fmt.Println("没有匹配的项目")
		return nil
	case 1:
		return projects[0]
	default:
		proj, err := tui.SelectItem("选择项目", projects, (*project.Project).Name)
		if err != nil {
			fmt.Printf("选择项目失败: %v\n", err)
			return nil
		}
		return proj
	}
}
