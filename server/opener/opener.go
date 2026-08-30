package opener

// Opener 一种「打开方式」的抽象（1016 接口化）。
// 目前唯一实现是 execOpener（命令模板，见 exec.go）
type Opener interface {
	Name() string

	// Title 展示文案（如「打开所在目录」「使用 VS Code 打开」），
	// 缺省配置时由 name 生成，永远有值。
	Title() string
	Roles() []Role

	// Icon 图标声明，零值表示未配置（前端 fallback 默认图标）。
	Icon() Icon

	// Summary 展示串（CLI 表格 / alfred 副标题 / Web DTO）：
	// exec 显示命令模板，web 显示 target。
	Summary() string

	// Commands 各 role 的命令模板原文（sh 风格字符串），供 Web 编辑表单无损回显——
	// 含空格的 token 以引号包裹，token 原文可从字符串无损恢复。
	// 每个 role 一条：同一 opener 的不同 role 可配不同命令（如 dir 与 diff）。
	Commands() map[Role]string

	// Open 以指定用途打开路径。role 校验收敛在实现内——不支持该 role 时返回
	// 中文错误，调用方无需再自行核对 HasRole。
	Open(role Role, slotArgs ...string) error
}
