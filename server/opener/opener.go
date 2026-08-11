package opener

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"cube/config"
)

type Opener struct {
	name      string   // 应用名, 唯一标识符
	cmd       []string // 启动命令模板：cmd[0]=可执行文件，其余为参数，含 $0/$1 占位符
	roles     []Role   // 该 opener 的业务用途集合；slotCount 由 roles 推导
	slotCount int      // 参数槽个数（由 roles 推导，供 BuildArgs 校验占位符）
	executor  Executor // 启动子进程的执行器（测试可注入 fake）
}

// InitOpener 从配置构造 Opener。
//   - cmd 必填，cmd[0] 是可执行文件；
//   - roles 解析为用途枚举并推导 slotCount；缺省为 ["open-dir"]；
//   - cmd 中出现的占位符索引不得 >= slotCount（越界报错）；
//   - executor 可选，缺省装 NewDefaultExecutor()（走 os/exec）；测试传 fake 断言命令。
func InitOpener(cfg config.OpenerConfig, executor Executor) (*Opener, error) {
	if len(cfg.Cmd) == 0 {
		return nil, fmt.Errorf("opener %q 缺少必填字段 cmd", cfg.Name)
	}

	roles, slotCount, err := ParseRoles(cfg.Roles)
	if err != nil {
		return nil, fmt.Errorf("opener %q roles 解析失败: %w", cfg.Name, err)
	}

	// 校验 cmd 中的占位符索引不越界（slotCount 由 roles 推导）
	for _, arg := range cfg.Cmd {
		if n, ok := placeholderIndex(arg); ok && n >= slotCount {
			return nil, fmt.Errorf("opener %q cmd 占位符 $%d 越界（slotCount=%d，合法索引 0..%d）", cfg.Name, n, slotCount, slotCount-1)
		}
	}

	// executor 默认值
	if executor == nil {
		executor = NewDefaultExecutor()
	}
	return &Opener{
		name:      cfg.Name,
		cmd:       slices.Clone(cfg.Cmd),
		roles:     roles,
		slotCount: slotCount,
		executor:  executor,
	}, nil
}

func (o *Opener) Name() string   { return o.name }
func (o *Opener) Cmd() []string  { return o.cmd }
func (o *Opener) Roles() []Role  { return o.roles }
func (o *Opener) SlotCount() int { return o.slotCount }

// HasRole 报告该 opener 是否声明了指定用途。
func (o *Opener) HasRole(role Role) bool {
	return slices.Contains(o.roles, role)
}

// RolesString 返回用途声明的展示，形如 "open-dir,diff-file"。
func (o *Opener) RolesString() string {
	parts := make([]string, len(o.roles))
	for i, r := range o.roles {
		parts[i] = string(r)
	}
	return strings.Join(parts, ",")
}

// CmdString 返回启动命令的展示（空格拼接），如 "bcompare -old $0 -new $1"。
func (o *Opener) CmdString() string {
	return strings.Join(o.cmd, " ")
}

// BuildArgs 构造启动该 opener 的完整命令参数（bin + args）。
//   - 参数个数必须等于 SlotCount()，否则报错；
//   - cmd 中的 $0/$1... 占位符被对应路径替换；无占位符的参数原样保留；
//   - 缺省（cmd 未含占位符时）路径按顺序追加到 args 末尾，兼容 ["code"] + path 形态。
//
// 返回 (bin, args) 供调用方自行启动子进程。
// Open() 是它的便捷封装（BuildArgs + executor.Run）。
func (o *Opener) BuildArgs(slotArgs ...string) (bin string, args []string, err error) {
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
	bin = rendered[0]
	args = rendered[1:]
	return bin, args, nil
}

// Open 用该 opener 打开一个或多个路径：构造 args 后委托 executor 启动子进程。
// 需要更灵活的启动方式（自定义 stdio、异步、非阻塞等）时，改用 BuildArgs 自行启动。
func (o *Opener) Open(slotArgs ...string) error {
	bin, args, err := o.BuildArgs(slotArgs...)
	if err != nil {
		return err
	}
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
