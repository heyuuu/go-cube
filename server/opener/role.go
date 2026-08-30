package opener

import (
	"fmt"
	"strings"
)

// Role 描述 opener 的参数约束（固定枚举，封闭稳定）：role 决定调用时需要几个
// 路径参数（槽个数），是 per-role cmd（见 Spec.Commands）占位符 $0/$1 校验的依据。
// 「场景/意图」（如终端打开、git 客户端打开）不属于 role——那是 defaults 的键空间，
// 由 role 之外的映射承载。
//
// 取值见 RoleXxx 常量与 roleSlotCount。
type Role string

const (
	// RoleOpenDir 打开目录（如项目根、worktree）。槽签名：[dir]，1 槽。
	RoleOpenDir Role = "open-dir"
	// RoleOpenFile 打开单个文件。槽签名：[file]，1 槽。
	RoleOpenFile Role = "open-file"
	// RoleDiffDir 对比两个目录。槽签名：[dir,dir]，2 槽。
	RoleDiffDir Role = "diff-dir"
	// RoleDiffFile 对比两个文件。槽签名：[file,file]，2 槽。
	RoleDiffFile Role = "diff-file"
)

// roleOrder role 的固定展示序（Roles/Summary 等遍历用）。
var roleOrder = []Role{RoleOpenDir, RoleOpenFile, RoleDiffDir, RoleDiffFile}

// RoleOrder 返回 role 固定序副本（展示/校验提示用）。
func RoleOrder() []Role { return append([]Role(nil), roleOrder...) }

// roleSlotCounts role → 槽个数。
var roleSlotCounts = map[Role]int{
	RoleOpenDir:  1,
	RoleOpenFile: 1,
	RoleDiffDir:  2,
	RoleDiffFile: 2,
}

// roleSlotCount 返回 role 的槽个数；未知 role 第二返回值为 false。
func roleSlotCount(r Role) (int, bool) {
	n, ok := roleSlotCounts[r]
	return n, ok
}

// ParseRole 解析单个 role 字符串，未知值返回中文错误。
func ParseRole(s string) (Role, error) {
	r := Role(strings.TrimSpace(s))
	if _, ok := roleSlotCount(r); !ok {
		return "", fmt.Errorf("未知的 opener role %q（合法值：%s）", s, RolesString(roleOrder))
	}
	return r, nil
}

// RolesString 把 role 声明格式化为 "open-dir,diff-file"（展示用）。
func RolesString(roles []Role) string {
	parts := make([]string, len(roles))
	for i, r := range roles {
		parts[i] = string(r)
	}
	return strings.Join(parts, ",")
}
