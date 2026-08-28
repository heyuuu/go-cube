package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"cube/cmd/internal/pick"
	"cube/opener"
	"cube/project"
	"cube/util/pathkit"
	"cube/util/tui"
)

// getArg 从 args 切片中安全取第 index 个元素，越界返回空字符串。
func getArg(args []string, index int) string { return pick.GetArg(args, index) }

// prettyTime 把时间格式化为「x秒前 / x分钟前 / x小时前 ...」等相对形式。
//
// 过去时间以「前」结尾，未来时间以「后」结尾（如「30秒后」）。
// 单位按秒/分钟/小时/天/月/年逐级放大，不保留零头（3 小时 20 分 → 「3小时前」）。
func prettyTime(t time.Time) string {
	d := time.Since(t)
	suffix := "前"
	if d < 0 {
		d = -d
		suffix = "后"
	}

	secs := int64(d.Seconds())
	switch {
	case secs < 60:
		return fmt.Sprintf("%d秒%s", secs, suffix)
	case secs < 3600:
		return fmt.Sprintf("%d分钟%s", secs/60, suffix)
	case secs < 86400:
		return fmt.Sprintf("%d小时%s", secs/3600, suffix)
	case secs < 30*86400:
		return fmt.Sprintf("%d天%s", secs/86400, suffix)
	case secs < 365*86400:
		return fmt.Sprintf("%d月%s", secs/(30*86400), suffix)
	default:
		return fmt.Sprintf("%d年%s", secs/(365*86400), suffix)
	}
}

func isPathQuery(query string) bool { return pick.IsPathQuery(query) }

func searchProjects(service *project.Service, query string, up bool) ([]*project.Project, error) {
	return pick.SearchProjects(service, query, up)
}

// pickProject 见 pick.PickProject；--local 模式（cubex 入口）下 query 缺省视作 "."，
// 以 cwd 为起点定位项目（localMode 状态在 cmd 包，不进共享包）。
func pickProject(service *project.Service, query string) (*project.Project, error) {
	if localMode && query == "" {
		query = "."
	}
	return pick.PickProject(service, query)
}

// checkOpenPath 解析并校验路径：返回绝对路径及其是否为目录。
func checkOpenPath(path string) (absPath string, isDir bool, err error) {
	// 获取绝对路径
	absPath, err = pathkit.AbsPath(path)
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
func pickOpener(service *opener.Service, role opener.Role, name string) (opener.Opener, error) {
	openers := service.SearchFor(role, name)
	if len(openers) == 0 {
		return nil, fmt.Errorf("未找到匹配的 opener: role=%s, name=`%s`", role, name)
	} else if len(openers) == 1 {
		return openers[0], nil
	}

	// 匹配多个 opener 时，触发用户选择
	pick, err := tui.SelectItem("选择 opener", openers, opener.Opener.Name)
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
