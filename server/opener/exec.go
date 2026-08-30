package opener

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
)

// execOpener 命令模板形态的打开方式：cmd 字符串分词后 token[0]=可执行文件，其余为参数，
// 用 $0/$1... 占位路径槽位（可嵌在 token 内），经 executor 启动子进程。
type execOpener struct {
	name      string   // 应用名, 唯一标识符
	title     string   // 展示文案，构造时已解析默认值
	cmdLine   string   // 启动命令原文（编辑表单回显用）
	cmdTokens []string // cmdLine 分词结果（构造时解析）
	roles     []Role   // 该 opener 的业务用途集合；slotCount 由 roles 推导
	slotCount int      // 参数槽个数（由 roles 推导，供 BuildArgs 校验占位符）
	icon      Icon     // 图标声明，零值未配置
	executor  Executor // 启动子进程的执行器（测试可注入 fake）
}

var _ Opener = (*execOpener)(nil)

// InitExecOpener 从存储形状构造 execOpener。
//   - cmd 必填（分词后至少一个 token，token[0] 是可执行文件）；
//   - roles 解析为用途枚举并推导 slotCount；缺省为 ["open-dir"]；
//   - cmd 中出现的占位符索引不得 >= slotCount（越界报错）；
//   - executor 可选，缺省装 NewDefaultExecutor()（走 os/exec）；测试传 fake 断言命令。
func InitExecOpener(spec Spec, executor Executor) (*execOpener, error) {
	tokens, err := tokenizeCmd(spec.Cmd)
	if err != nil {
		return nil, fmt.Errorf("opener %q cmd 非法: %w", spec.Name, err)
	}
	if len(tokens) == 0 {
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
	for _, token := range tokens {
		for _, n := range scanPlaceholders(token) {
			if n >= slotCount {
				return nil, fmt.Errorf("opener %q cmd 占位符 $%d 越界（slotCount=%d，合法索引 0..%d）", spec.Name, n, slotCount, slotCount-1)
			}
		}
	}

	// executor 默认值
	if executor == nil {
		executor = NewDefaultExecutor()
	}
	return &execOpener{
		name:      spec.Name,
		title:     title,
		cmdLine:   spec.Cmd,
		cmdTokens: tokens,
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

func (o *execOpener) Cmd() string { return o.cmdLine }

func (o *execOpener) Summary() string {
	return o.cmdLine
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
	rendered, used := renderArgs(o.cmdTokens, slotArgs)
	// cmd 参数里没有任何占位符时，把路径按顺序追加到末尾（兼容纯 "code" 配置）
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

// renderArgs 渲染各 token 内的占位符 $0/$1... 为 paths 对应项（token 内子串替换，
// 支持 --wd=$0 形态；替换值不再分词，路径含空格/引号安全）。
// 返回渲染后的参数列表，以及实际命中占位符的个数。
func renderArgs(args []string, paths []string) ([]string, int) {
	out := make([]string, 0, len(args))
	used := 0
	for _, a := range args {
		rendered, hit := renderPlaceholders(a, paths)
		if hit {
			used++
		}
		out = append(out, rendered)
	}
	return out, used
}

// tokenizeCmd sh 风格分词：空白分隔，单/双引号内内容（含空格）为一个 token；
// 双引号内及裸词态支持反斜杠转义下一个字符。未闭合引号返回中文错误——
// 配置校验发生在保存/加载边界，此时引号未闭合即配置写错，应拦下而非运行期才失败。
func tokenizeCmd(line string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	hasToken := false
	flush := func() {
		if hasToken {
			tokens = append(tokens, cur.String())
			cur.Reset()
			hasToken = false
		}
	}

	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		switch {
		case ch == '\\' && i+1 < len(runes):
			cur.WriteRune(runes[i+1])
			hasToken = true
			i++
		case ch == '\'' || ch == '"':
			quote := ch
			hasToken = true
			closed := false
			for i++; i < len(runes); i++ {
				if runes[i] == quote {
					closed = true
					break
				}
				if quote == '"' && runes[i] == '\\' && i+1 < len(runes) {
					i++
				}
				cur.WriteRune(runes[i])
			}
			if !closed {
				return nil, fmt.Errorf("cmd 引号 %c 未闭合", quote)
			}
		case ch == ' ' || ch == '\t':
			flush()
		default:
			cur.WriteRune(ch)
			hasToken = true
		}
	}
	flush()
	return tokens, nil
}

// scanPlaceholders 扫描 token 内所有 $<数字> 占位符（数字取最长连续段），返回索引列表。
func scanPlaceholders(token string) []int {
	var indices []int
	for i := 0; i < len(token); i++ {
		if token[i] != '$' || i+1 >= len(token) || !isDigit(token[i+1]) {
			continue
		}
		j := i + 1
		for j < len(token) && isDigit(token[j]) {
			j++
		}
		n, _ := strconv.Atoi(token[i+1 : j])
		indices = append(indices, n)
		i = j - 1
	}
	return indices
}

// renderPlaceholders 把 token 内所有 $<数字>（索引在 paths 范围内）替换为对应路径，
// 返回渲染结果与是否发生替换。越界占位符原样保留。替换发生在分词之后，
// 路径含空格/引号也不会被再次分词。
func renderPlaceholders(token string, paths []string) (string, bool) {
	var b strings.Builder
	used := false
	for i := 0; i < len(token); i++ {
		if token[i] == '$' && i+1 < len(token) && isDigit(token[i+1]) {
			j := i + 1
			for j < len(token) && isDigit(token[j]) {
				j++
			}
			n, _ := strconv.Atoi(token[i+1 : j])
			if n < len(paths) {
				b.WriteString(paths[n])
				used = true
			} else {
				b.WriteString(token[i:j])
			}
			i = j - 1
			continue
		}
		b.WriteByte(token[i])
	}
	return b.String(), used
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
