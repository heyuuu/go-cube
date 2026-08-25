package opener

// opener 形态（Spec.Type 取值）。
const (
	SpecTypeExec = "exec" // 命令模板（默认，Type 缺省视为 exec）
	SpecTypeWeb  = "web"  // 跳转 cube Web 页面（需配 Target）
)

// Spec opener 在 settings.json openers 节内的存储形状。
// 节内结构是 opener 域的知识，DTO 归本包（settings 包不感知节内形状）。
type Spec struct {
	Name   string   `json:"name"`
	Type   string   `json:"type,omitempty"`   // exec（默认）| web
	Cmd    []string `json:"cmd"`              // exec：启动命令，cmd[0]=可执行文件，其余为参数；用 $0/$1... 占位路径槽位
	Target string   `json:"target,omitempty"` // web：目标页面白名单枚举（workbench）
	Roles  []string `json:"roles"`            // 业务用途枚举，如 ["open-dir"]、["diff-dir","diff-file"]；缺省视为 ["open-dir"]
	Icon   *Icon    `json:"icon,omitempty"`   // 图标声明（前端渲染方式），缺省无图标
}
