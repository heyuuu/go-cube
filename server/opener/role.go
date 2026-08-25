package opener

import (
	"fmt"
	"strings"
)

// Role 描述 opener 的业务用途（固定枚举）。筛选时按用途而非参数类型，
// 避免「参数类型相同但用途不同」导致的筛选错位。
//
// 取值见 RoleXxx 常量。每个 role 内含一组参数槽类型签名（见 roleSlots），
// 用于推导参数槽个数（slotCount），供 cmd 占位符 $0/$1 越界校验。
type Role string

const (
	// RoleOpenDir 打开目录（如项目根、worktree）。槽签名：[dir]，slotCount=1。
	RoleOpenDir Role = "open-dir"
	// RoleOpenFile 打开单个文件。槽签名：[file]，slotCount=1。
	RoleOpenFile Role = "open-file"
	// RoleDiffDir 对比两个目录。槽签名：[dir,dir]，slotCount=2。
	RoleDiffDir Role = "diff-dir"
	// RoleDiffFile 对比两个文件。槽签名：[file,file]，slotCount=2。
	RoleDiffFile Role = "diff-file"
)

// 参数类型常量（内部用，role 签名的元素）。
const (
	typeDir  = "dir"
	typeFile = "file"
)

// roleSlots 是 role → 每槽参数类型签名 的映射。
// 类型仅区分 dir/file（内部细节，不暴露给配置与筛选层）。
var roleSlots = map[Role][]string{
	RoleOpenDir:  {typeDir},
	RoleOpenFile: {typeFile},
	RoleDiffDir:  {typeDir, typeDir},
	RoleDiffFile: {typeFile, typeFile},
}

// roleSlotCount 返回 role 对应的参数槽个数。
func roleSlotCount(r Role) int { return len(roleSlots[r]) }

// ParseRoles 解析 role 字符串数组。
//   - 每个 role 必须是合法枚举值，否则返回中文错误；
//   - 同一 opener 声明的所有 role 必须 slotCount 一致（cmd 的 $0/$1 占位符个数唯一），
//     否则报错；
//   - 返回解析后的 role 列表与统一 slotCount。
func ParseRoles(raw []string) (roles []Role, slotCount int, err error) {
	if len(raw) == 0 {
		// 缺省视为 open-dir（最常见的「打开目录」场景）
		return []Role{RoleOpenDir}, roleSlotCount(RoleOpenDir), nil
	}

	roles = make([]Role, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		r := Role(s)
		if _, ok := roleSlots[r]; !ok {
			return nil, 0, fmt.Errorf("未知的 opener role %q（合法值：open-dir/open-file/diff-dir/diff-file）", s)
		}
		roles = append(roles, r)
	}

	// 校验所有 role 的 slotCount 一致
	slotCount = roleSlotCount(roles[0])
	for _, r := range roles[1:] {
		if roleSlotCount(r) != slotCount {
			return nil, 0, fmt.Errorf("opener 声明的 role slotCount 不一致（占位符个数需唯一）：涉及 %s", RolesString(roles))
		}
	}
	return roles, slotCount, nil
}

// RolesString 把 role 声明格式化为 "open-dir,diff-file"（展示用）。
func RolesString(roles []Role) string {
	parts := make([]string, len(roles))
	for i, r := range roles {
		parts[i] = string(r)
	}
	return strings.Join(parts, ",")
}
