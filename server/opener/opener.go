package opener

// Opener 一种「打开方式」的抽象（1016 接口化）。
// 唯一实现是 execOpener（命令模板，见 exec.go）——「打开工作台页」这类需求
// 通过 exec 命令组合 cube 自身 CLI（cube web ui）实现，不设独立 web 形态。
type Opener interface {
	Name() string
	Roles() []Role

	// Icon 图标声明，零值表示未配置（前端 fallback 默认图标）。
	Icon() Icon

	// Summary 展示串（CLI 表格 / alfred 副标题 / Web DTO）：
	// exec 显示命令模板，web 显示 target。
	Summary() string

	// Open 以指定用途打开路径。role 校验收敛在实现内——不支持该 role 时返回
	// 中文错误，调用方无需再自行核对 HasRole。
	Open(role Role, slotArgs ...string) error
}
