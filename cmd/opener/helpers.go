package opener

import (
	"errors"
	"fmt"
	"os"

	"github.com/heyuuu/cube/cmd/util/tui"
	"github.com/heyuuu/cube/opener"
)

// PathType 区分路径类型（目录 / 文件），用于按 role 选择 opener。
type PathType string

const (
	TypeDir  PathType = "dir"
	TypeFile PathType = "file"
)

// detectPathType 用 os.Stat 判断路径类型。路径不存在时返回中文错误。
// 存在且为目录返回 TypeDir，否则返回 TypeFile。
func detectPathType(p string) (PathType, error) {
	info, err := os.Stat(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("路径不存在: %s", p)
		}
		return "", fmt.Errorf("读取路径失败: %w", err)
	}
	if info.IsDir() {
		return TypeDir, nil
	}
	return TypeFile, nil
}

// pickOpener 按 TTY 与否分派到对应实现：
//   - TTY：交互环境，可模糊匹配 / 列表选择 opener。
//   - 非 TTY：必须显式指定 --opener，按精确名查找并校验 role。
//
// 用户取消选择（ErrUserAborted）时返回该错误，由调用方决定是否静默。
func pickOpener(service *opener.Service, role opener.Role, name string) (*opener.Opener, error) {
	if tui.IsTTY() {
		return pickOpenerTTY(service, role, name)
	}
	return pickOpenerNonTTY(service, role, name)
}

// pickOpenerTTY 处理交互环境（终端）下的 opener 选择。
//   - name != ""：SearchFor 模糊匹配；命中多个则交互选择，命中零个报错。
//   - name == ""：从 RoleOpeners(role) 中交互选择。
func pickOpenerTTY(service *opener.Service, role opener.Role, name string) (*opener.Opener, error) {
	openers := service.SearchFor(role, name)
	if len(openers) == 0 {
		return nil, fmt.Errorf("未找到匹配的 opener: role=%s, name=`%s`", role, name)
	} else if len(openers) == 1 {
		return openers[0], nil
	} else { // 匹配多个 opener 时，触发用户选择
		pick, err := tui.SelectItem("选择 opener", openers, (*opener.Opener).Name)
		if err != nil {
			return nil, err
		}
		return pick, nil
	}
}

// pickOpenerNonTTY 处理非交互环境（脚本 / alfred 等）下的 opener 选择：
// 必须显式指定 --opener，按精确名查找，命中后还会校验是否支持给定 role。
func pickOpenerNonTTY(service *opener.Service, role opener.Role, name string) (*opener.Opener, error) {
	if name == "" {
		return nil, errors.New("非交互环境(tty)下必须通过 --opener 指定 opener 名")
	}
	pick := service.FindByName(name)
	if pick == nil {
		return nil, fmt.Errorf("未找到指定 opener: %s", name)
	}
	if !pick.HasRole(role) {
		return nil, fmt.Errorf("opener %s 不支持该用途(需声明 roles:[%q])，当前 roles=%s", name, role, pick.RolesString())
	}
	return pick, nil
}
