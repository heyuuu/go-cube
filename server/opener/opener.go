package opener

// Opener 一种「打开方式」的抽象（1016 接口化）：exec（命令模板，见 exec.go）与
// web（跳转 cube 工作台页，后续步骤）两个实现。
// Icon() 将在 icon 步骤作为纯加法加入本接口。
type Opener interface {
	Name() string
	Roles() []Role

	// Summary 展示串（CLI 表格 / alfred 副标题 / Web DTO）：
	// exec 显示命令模板，web 显示 target。
	Summary() string

	// Open 以指定用途打开路径。role 校验收敛在实现内——不支持该 role 时返回
	// 中文错误，调用方无需再自行核对 HasRole。
	Open(role Role, slotArgs ...string) error
}
