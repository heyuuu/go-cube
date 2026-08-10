package cmd

import (
	"errors"
	"fmt"
	"os"

	"cube/opener"
	"cube/project"
	"cube/util/pathkit"
	"cube/util/tui"
)

// getArg 从 args 切片中安全取第 index 个元素，越界返回空字符串。
func getArg(args []string, index int) string {
	if len(args) > index {
		return args[index]
	}
	return ""
}

// isPathQuery 判断 query 是否为路径(以`.`/`~`/`/` 开头时，当做路径)
func isPathQuery(query string) bool {
	return len(query) > 0 && (query[0] == '.' || query[0] == '~' || query[0] == '/')
}

// searchProjects 搜索项目列表
//
// query 为搜索关键词，默认为搜索项目名；当以`.`/`~`/`/` 开头时，当做路径
// upper 表示是否向上搜索。仅 query 为路径时生效，用于在项目子目录标定当前目录时使用。
func searchProjects(service *project.Service, query string, up bool) []*project.Project {
	if isPathQuery(query) {
		return service.SearchByPath(query, up)
	} else {
		return service.SearchByName(query)
	}
}

// pickProject 根据关键词匹配项目：精确匹配直接返回，多项匹配则交互选择。
//
// 非交互环境不支持多项选择，会报错提示使用精确名称或路径。
func pickProject(service *project.Service, query string) (*project.Project, error) {
	projects := searchProjects(service, query, true)
	if len(projects) == 0 {
		return nil, fmt.Errorf("未找到匹配的 project: query=`%s`", query)
	} else if len(projects) == 1 {
		return projects[0], nil
	}

	// 匹配多个 project 时，触发用户选择
	pick, err := tui.SelectItem("选择 project", projects, (*project.Project).Name)
	if err != nil {
		if errors.Is(err, tui.ErrNotTTY) {
			return nil, fmt.Errorf("非 TTY 环境请使用精确项目名或项目路径，避免匹配多项。: query=`%s`", query)
		}
		if errors.Is(err, tui.ErrUserAborted) {
			return nil, fmt.Errorf("用户取消了 project 选择: %w", err)
		}
		return nil, err
	}
	return pick, nil
}

// checkOpenPath 解析并校验路径：返回绝对路径及其是否为目录。
func checkOpenPath(path string) (absPath string, isDir bool, err error) {
	// 获取绝对路径
	absPath, err = pathkit.ResolvePath(path)
	if err != nil {
		return "", false, err
	}
	// 获取文件信息
	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, fmt.Errorf("路径不存在: %s", path)
		}
		return "", false, fmt.Errorf("读取路径失败: %w", err)
	}

	return absPath, info.IsDir(), nil
}

// pickOpener 根据 role 和关键词匹配 opener：精确匹配直接返回，多项匹配则交互选择。
//
// 非交互环境不支持多项选择，会报错提示使用精确 opener 名。
func pickOpener(service *opener.Service, role opener.Role, name string) (*opener.Opener, error) {
	openers := service.SearchFor(role, name)
	if len(openers) == 0 {
		return nil, fmt.Errorf("未找到匹配的 opener: role=%s, name=`%s`", role, name)
	} else if len(openers) == 1 {
		return openers[0], nil
	}

	// 匹配多个 opener 时，触发用户选择
	pick, err := tui.SelectItem("选择 opener", openers, (*opener.Opener).Name)
	if err != nil {
		if errors.Is(err, tui.ErrNotTTY) {
			return nil, fmt.Errorf("非 TTY 环境请使用精确 opener 名，避免匹配多项: name=`%s`", name)
		}
		if errors.Is(err, tui.ErrUserAborted) {
			return nil, fmt.Errorf("用户取消了 opener 选择: %w", err)
		}
		return nil, err
	}
	return pick, nil
}
