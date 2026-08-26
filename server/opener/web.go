package opener

import (
	"fmt"
	"net/url"
	"runtime"
	"slices"
)

// web target 白名单（1016：target 是白名单枚举，后续加值是纯加法）。
const (
	TargetWorkbench = "workbench"
)

// webOpener 跳转 cube Web 页面形态的打开方式：不启动子进程，而是打开浏览器
// 到 server 的对应页面。Web 前端场景不经过它——前端识别 kind=web 后直接路由
// 跳转、不发 open 请求；这里服务于 CLI（cube open -o xxx）与 open API 兜底。
type webOpener struct {
	name     string
	target   string // 白名单枚举，v1 只有 workbench
	roles    []Role
	icon     Icon
	baseURL  string   // cube server 根地址（如 http://localhost:6001），装配时注入
	executor Executor // 打开浏览器的执行器（复用 Executor 抽象，测试注入 fake）
}

var _ Opener = (*webOpener)(nil)

// InitWebOpener 从存储形状构造 webOpener。
//   - target 必填且必须在白名单内（v1 只有 workbench）；
//   - roles / icon 校验同 execOpener；slotCount 语义同 exec（open-dir 单槽）。
func InitWebOpener(spec Spec, baseURL string, executor Executor) (*webOpener, error) {
	if spec.Target == "" {
		return nil, fmt.Errorf("opener %q 缺少必填字段 target", spec.Name)
	}
	if spec.Target != TargetWorkbench {
		return nil, fmt.Errorf("opener %q 未知的 web target %q（合法值：workbench）", spec.Name, spec.Target)
	}
	roles, _, err := ParseRoles(spec.Roles)
	if err != nil {
		return nil, fmt.Errorf("opener %q roles 解析失败: %w", spec.Name, err)
	}
	icon, err := InitIcon(spec.Icon)
	if err != nil {
		return nil, fmt.Errorf("opener %q icon 解析失败: %w", spec.Name, err)
	}

	// executor 默认值
	if executor == nil {
		executor = NewDefaultExecutor()
	}
	return &webOpener{
		name:     spec.Name,
		target:   spec.Target,
		roles:    roles,
		icon:     icon,
		baseURL:  baseURL,
		executor: executor,
	}, nil
}

func (w *webOpener) Name() string  { return w.name }
func (w *webOpener) Roles() []Role { return w.roles }
func (w *webOpener) Icon() Icon    { return w.icon }
func (w *webOpener) Kind() string  { return SpecTypeWeb }

// Summary 展示串：web 形态显示 target。
func (w *webOpener) Summary() string { return w.target }

// Open 打开浏览器到目标页面。slotArgs 语义与 exec 相同（$0=主路径），
// 目前所有 target 都只消费第一个槽（工作台页 path 参数）。
//
// baseURL 为空（装配缺失）时报错而非静默；浏览器唤起命令按平台分发，
// 非 darwin 平台暂不支持（TODO：linux xdg-open / windows start）。
func (w *webOpener) Open(role Role, slotArgs ...string) error {
	if !w.hasRole(role) {
		return fmt.Errorf("opener %s 不支持 %s", w.name, role)
	}
	if w.baseURL == "" {
		return fmt.Errorf("opener %s 需要 server 地址（config.Server.Port），当前未配置", w.name)
	}
	if len(slotArgs) < 1 {
		return fmt.Errorf("opener %s 需要 1 个路径参数，实际传入 0", w.name)
	}
	pageURL := fmt.Sprintf("%s/%s?path=%s", w.baseURL, w.target, url.QueryEscape(slotArgs[0]))

	var launchCmd string
	switch runtime.GOOS {
	case "darwin":
		launchCmd = "open"
	default:
		return fmt.Errorf("web opener 暂不支持 %s 平台（TODO：xdg-open / start）", runtime.GOOS)
	}
	return w.executor.Run(launchCmd, pageURL)
}

func (w *webOpener) hasRole(role Role) bool {
	return slices.Contains(w.roles, role)
}
