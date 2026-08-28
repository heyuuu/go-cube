// Package pick 提供 cmd 层的「选项目」交互助手（cmd 包与各子命令组共用，
// 子包不能反向 import cmd 父包，故独立成 internal 包）。
//
// 这里的内容属出口层职责（cwd 解析、交互编排），按分层纪律不沉淀进 util/domain。
package pick

import (
	"errors"
	"fmt"

	"cube/project"
	"cube/util/pathkit"
	"cube/util/tui"
)

// GetArg 取命令位置参数，越界返回空串。
func GetArg(args []string, index int) string {
	if len(args) > index {
		return args[index]
	}
	return ""
}

// IsPathQuery 判断 query 是否为路径（以 `.` / `~` / `/` 开头时当作路径）。
func IsPathQuery(query string) bool {
	return len(query) > 0 && (query[0] == '.' || query[0] == '~' || query[0] == '/')
}

// SearchProjects 搜索项目列表：query 为空默认搜索项目名；为路径时按路径搜（upper 表示向上搜）。
// 路径 query 在本层用 AbsPath 基于 cwd 解析为绝对路径后再传入 domain——
// cwd 依赖属于出口层职责，domain 只接受绝对路径/~ 前缀。
func SearchProjects(service *project.Service, query string, up bool) ([]*project.Project, error) {
	if IsPathQuery(query) {
		absPath, err := pathkit.AbsPath(query)
		if err != nil {
			return nil, fmt.Errorf("解析路径 query 失败: query=%s err=%w", query, err)
		}
		return service.SearchByPath(absPath, up), nil
	}
	return service.SearchByName(query), nil
}

// PickProject 根据关键词匹配项目：精确匹配直接返回，多项匹配交互选择。
// 非 TTY 环境不支持多项选择，报错提示使用精确名称或路径。
// 路径 query 命中 worktree 目录时归并到主项目（1032）。
// --local 模式的 query 缺省补 "." 属调用方（cmd 包持有 localMode 状态）。
func PickProject(service *project.Service, query string) (*project.Project, error) {
	projects, err := SearchProjects(service, query, true)
	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		// 路径 query 落在 worktree 内：SearchByPath 找不到（worktree 通常在项目目录之外），归并主项目
		if IsPathQuery(query) {
			if absPath, err := pathkit.AbsPath(query); err == nil {
				if proj := service.ResolveProject(absPath); proj != nil {
					return proj, nil
				}
			}
		}
		return nil, fmt.Errorf("未找到匹配的 project: query=`%s`", query)
	} else if len(projects) == 1 {
		return projects[0], nil
	}

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
