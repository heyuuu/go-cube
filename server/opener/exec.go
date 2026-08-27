package opener

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
)

// execOpener 命令模板形态的打开方式：cmd[0]=可执行文件，其余为参数，
// 用 $0/$1... 占位路径槽位，经 executor 启动子进程。
type execOpener struct {
	name      string   // 应用名, 唯一标识符
	title     string   // 展示文案，构造时已解析默认值
	cmd       []string // 启动命令模板
	roles     []Role   // 该 opener 的业务用途集合；slotCount 由 roles 推导
	slotCount int      // 参数槽个数（由 roles 推导，供 BuildArgs 校验占位符）
	icon      Icon     // 图标声明，零值未配置
	executor  Executor // 启动子进程的执行器（测试可注入 fake）
}

var _ Opener = (*execOpener)(nil)

// InitExecOpener 从存储形状构造 execOpener。
//   - cmd 必填，cmd[0] 是可执行文件；
//   - roles 解析为用途枚举并推导 slotCount；缺省为 ["open-dir"]；
//   - cmd 中出现的占位符索引不得 >= slotCount（越界报错）；
//   - executor 可选，缺省装 NewDefaultExecutor()（走 os/exec）；测试传 fake 断言命令。
func InitExecOpener(spec Spec, executor Executor) (*execOpener, error) {
	if len(spec.Cmd) == 0 {
		return nil, fmt.Errorf("opener %q 缺少必填字段 cmd", spec.Name)
	}

	roles, slotCount, err := ParseRoles(spec.Roles)
	if err != nil {
		return nil, fmt.Errorf("opener %q roles 解析失败: %w", spec.Name, err)
	}

	title := spec.Title
	if title == "" {
		title = "用 " + spec.Name + " 打开"
	}
	icon, err := InitIcon(spec.Icon)
	if err != nil {
		return nil, fmt.Errorf("opener %q icon 解析失败: %w", spec.Name, err)
	}

	// 校验 cmd 中的占位符索引不越界（slotCount 由 roles 推导）
	for _, arg := range spec.Cmd {
		if n, ok := placeholderIndex(arg); ok && n >= slotCount {
			return nil, fmt.Errorf("opener %q cmd 占位符 $%d 越界（slotCount=%d，合法索引 0..%d）", spec.Name, n, slotCount, slotCount-1)
		}
	}

	// executor 默认值
	if executor == nil {
		executor = NewDefaultExecutor()
	}
	return &execOpener{
		name:      spec.Name,
		title:     title,
		cmd:       slices.Clone(spec.Cmd),
		roles:     roles,
		slotCount: slotCount,
		icon:      icon,
		executor:  executor,
	}, nil
}

func (o *execOpener) Name() string  { return o.name }
func (o *execOpener) Title() string { return o.title }
func (o *execOpener) Roles() []Role { return o.roles }
func (o *execOpener) Icon() Icon    { return o.icon }

func (o *execOpener) Summary() string {
	return strings.Join(o.cmd, " ")
}

// BuildArgs 构造启动该 opener 的完整命令参数（bin + args）。
//   - 参数个数必须等于 slotCount，否则报错；
//   - cmd 中的 $0/$1... 占位符被对应路径替换；无占位符的参数原样保留；
//   - 缺省（cmd 未含占位符时）路径按顺序追加到 args 末尾，兼容 ["code"] + path 形态；
//   - bin 自引用替换（见 resolveBin）：cmd[0] 是 cube 时换成当前进程的可执行文件。
//
// 返回 (bin, args) 供调用方自行启动子进程。
// Open() 是它的便捷封装（role 校验 + BuildArgs + executor.Run）。
func (o *execOpener) BuildArgs(slotArgs ...string) (bin string, args []string, err error) {
	if len(slotArgs) != o.slotCount {
		return "", nil, fmt.Errorf("opener %s 需要 %d 个路径参数，实际传入 %d", o.name, o.slotCount, len(slotArgs))
	}

	// 先对整条 cmd 渲染占位符（cmd[0] 也可能是占位符，如用路径本身作可执行文件），
	// 再取 [0] 为 bin、[1:] 为 args。
	rendered, used := renderArgs(o.cmd, slotArgs)
	// cmd 参数里没有任何占位符时，把路径按顺序追加到末尾（兼容纯 ["code"] 配置）
	if used == 0 {
		rendered = append(rendered, slotArgs...)
	}
	bin = resolveBin(rendered[0])
	args = rendered[1:]
	return bin, args, nil
}

// resolveBin 自引用替换：bin 为 "cube" 或以 "/cube" 结尾时，换成当前进程的可执行文件。
// opener 可组合 cube 自身 CLI（如 ["cube","ui","workbench","$0"]），dev 环境（air/run.sh
// 源码直跑）PATH 里未必有 cube、或装的是旧版本；替换后始终与当前进程同源。
// os.Executable 失败时保留原值降级（交给 PATH 解析兜底）。
func resolveBin(bin string) string {
	if bin != "cube" && !strings.HasSuffix(bin, "/cube") {
		return bin
	}
	self, err := os.Executable()
	if err != nil {
		slog.Debug("解析当前可执行文件失败，opener cmd[0] 保留原值", "err", err)
		return bin
	}
	return self
}

// Open 用该 opener 以指定用途打开一个或多个路径：校验 role 后构造 args 并委托
// executor 启动子进程。需要更灵活的启动方式（自定义 stdio、异步、非阻塞等）时，
// 改用 BuildArgs 自行启动。
func (o *execOpener) Open(role Role, slotArgs ...string) error {
	if !slices.Contains(o.roles, role) {
		return fmt.Errorf("opener %s 不支持 %s", o.name, role)
	}
	bin, args, err := o.BuildArgs(slotArgs...)
	if err != nil {
		return err
	}
	slog.Debug("execOpener.Open", "bin", bin, "args", args)
	return o.executor.Run(bin, args...)
}

// renderArgs 渲染 args 中的占位符 $0/$1... 为 paths 对应项。
// 返回渲染后的参数列表，以及实际命中占位符的个数。
func renderArgs(args []string, paths []string) ([]string, int) {
	out := make([]string, 0, len(args))
	used := 0
	for _, a := range args {
		if n, ok := placeholderIndex(a); ok && n < len(paths) {
			out = append(out, paths[n])
			used++
		} else {
			out = append(out, a)
		}
	}
	return out, used
}

// placeholderIndex 判断 s 是否为纯占位符 "$<n>"，返回 n 和 true；
// 形如 "-old"、"file$0"（占位符非整体）返回 false。
func placeholderIndex(s string) (int, bool) {
	if len(s) < 2 || s[0] != '$' {
		return 0, false
	}
	n, err := strconv.Atoi(s[1:])
	if err != nil {
		return 0, false
	}
	return n, true
}
