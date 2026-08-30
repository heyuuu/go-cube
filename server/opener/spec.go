package opener

// Spec opener 在 settings.json openers 节内的存储形状。
// 节内结构是 opener 域的知识，DTO 归本包（settings 包不感知节内形状）。
type Spec struct {
	Name    string          `json:"name"`
	Title   string          `json:"title,omitempty"` // 展示文案（如「打开所在目录」「使用 VS Code 打开」），缺省由 name 生成
	Actions map[Role]string `json:"actions"`         // role → 动作串 `<kind>:<模板>`（kind: exec 命令 / url 链接，见 exec.go）；$0/$1... 占位路径槽位，可嵌在模板内（如 --wd=$0）。声明了哪些 role 即键集合
	Icon    *Icon           `json:"icon,omitempty"`  // 图标声明（前端渲染方式），缺省无图标
}
