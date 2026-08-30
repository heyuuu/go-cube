package opener

// Spec opener 在 settings.json openers 节内的存储形状。
// 节内结构是 opener 域的知识，DTO 归本包（settings 包不感知节内形状）。
type Spec struct {
	Name     string          `json:"name"`
	Title    string          `json:"title,omitempty"` // 展示文案（如「打开所在目录」「使用 VS Code 打开」），缺省由 name 生成
	Commands map[Role]string `json:"commands"`        // role → 启动命令（sh 风格字符串，token[0]=可执行文件）；$0/$1... 占位路径槽位，可嵌在 token 内（如 --wd=$0）。声明了哪些 role 即键集合
	Icon     *Icon           `json:"icon,omitempty"`  // 图标声明（前端渲染方式），缺省无图标
}
