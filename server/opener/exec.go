package opener

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// ActionKind 动作类型：动作串 `<kind>:<模板>` 的前缀（首个冒号前，前后空格可选）。
type ActionKind string

const (
	// KindExec 启动子进程执行命令（sh 风格分词 + $N 占位符渲染）。
	KindExec ActionKind = "exec"
	// KindURL 打开 URL：`/` 开头为站内路由（拼 baseURL 后打开），`http(s)://` 开头为外部 URL。
	KindURL ActionKind = "url"
)

// sysOpenBin 系统默认打开器（打开 URL / 文件）。cube 是纯 macOS 工具，恒为 open。
func sysOpenBin() string { return "open" }

// parseAction 解析动作串 `<kind>:<模板>`：只认首个冒号，冒号后空格可选；
// 前缀必须是合法 kind（无默认前缀），模板非空。
func parseAction(raw string) (ActionKind, string, error) {
	idx := strings.Index(raw, ":")
	if idx < 0 {
		return "", "", fmt.Errorf("动作串缺少 kind 前缀（应为 exec: 或 url: 开头）: %q", raw)
	}
	kind := ActionKind(strings.TrimSpace(raw[:idx]))
	switch kind {
	case KindExec, KindURL:
	default:
		return "", "", fmt.Errorf("未知动作前缀 %q（合法值：exec/url）", raw[:idx])
	}
	tmpl := strings.TrimSpace(raw[idx+1:])
	if tmpl == "" {
		return "", "", fmt.Errorf("动作串 %q 前缀后的模板为空", kind)
	}
	return kind, tmpl, nil
}

// validateURLTemplate 校验 url 动作模板的值域：`/` 开头（站内路由）或 `http(s)://` 开头（外部 URL）。
func validateURLTemplate(tmpl string) error {
	if strings.HasPrefix(tmpl, "/") || strings.HasPrefix(tmpl, "http://") || strings.HasPrefix(tmpl, "https://") {
		return nil
	}
	return fmt.Errorf("url 动作模板必须以 /（站内路由）或 http(s)://（外部 URL）开头: %q", tmpl)
}

// renderTemplate 渲染模板内所有 $<数字>（索引在 paths 范围内）为对应路径，返回渲染结果
// 与是否发生替换；越界占位符原样保留。escape=true（url 站内路由）时替换值做 query encode
// （路由端 decode 还原）；escape=false（exec 分词后）照常子串替换，路径含空格/引号也不会被再次分词。
func renderTemplate(tmpl string, paths []string, escape bool) (string, bool) {
	var b strings.Builder
	used := false
	for i := 0; i < len(tmpl); i++ {
		if tmpl[i] == '$' && i+1 < len(tmpl) && isDigit(tmpl[i+1]) {
			j := i + 1
			for j < len(tmpl) && isDigit(tmpl[j]) {
				j++
			}
			n, _ := strconv.Atoi(tmpl[i+1 : j])
			if n < len(paths) {
				if escape {
					b.WriteString(url.QueryEscape(paths[n]))
				} else {
					b.WriteString(paths[n])
				}
				used = true
			} else {
				b.WriteString(tmpl[i:j])
			}
			i = j - 1
			continue
		}
		b.WriteByte(tmpl[i])
	}
	return b.String(), used
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
	return renderTemplate(token, paths, false)
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// mustRender url 模板渲染（不关心 used，url 动作占位符缺失时原样保留即可）。
func mustRender(tmpl string, paths []string, escape bool) string {
	s, _ := renderTemplate(tmpl, paths, escape)
	return s
}

// actionSpec 单个 role 的动作模板：构造时完成前缀解析、值域与占位符校验。
type actionSpec struct {
	kind   ActionKind // exec / url
	raw    string     // 动作串原文（含前缀，编辑表单回显用）
	tmpl   string     // 前缀后的模板
	tokens []string   // exec 形态的 tmpl 分词结果（url 形态为 nil）
}

// actionOpener 打开方式的唯一实现：按 role 各配一条动作串（`exec:` 命令 / `url:` 链接）。
// exec 动作：sh 风格分词，token[0]=可执行文件，$0/$1... 占位路径槽位（可嵌在 token 内），
// 经 executor 启动子进程；url 动作：渲染占位符（站内路由 query encode + 拼 baseURL）
// 后经系统 opener 打开，同样走 executor（可注入测试）。
type actionOpener struct {
	name     string              // 应用名, 唯一标识符
	title    string              // 展示文案，构造时已解析默认值
	actions  map[Role]actionSpec // role → 动作模板；声明了哪些 role 即键集合
	baseURL  string              // 站内路由的基地址（如 http://127.0.0.1:6001），装配注入
	icon     Icon                // 图标声明，零值未配置
	executor Executor            // 启动子进程的执行器（测试可注入 fake）
}

var _ Opener = (*actionOpener)(nil)

// InitActionOpener 从存储形状构造 actionOpener。
//   - actions 必填非空：键须为合法 role；每条动作串须为合法 `<kind>:<模板>`；
//   - exec 模板须可分词出至少一个 token（token[0] 是可执行文件）；
//   - url 模板须以 /（站内）或 http(s)://（外部）开头；
//   - 每条模板的占位符索引不得越该 role 的槽个数（逐条局部校验，role 间无一致性约束）；
//   - baseURL 供站内路由拼接（空串时站内动作构造报错，装配层负责注入）；
//   - executor 可选，缺省装 NewDefaultExecutor()（走 os/exec）；测试传 fake 断言命令。
func InitActionOpener(spec Spec, executor Executor, baseURL string) (*actionOpener, error) {
	if len(spec.Actions) == 0 {
		return nil, fmt.Errorf("opener %q 缺少必填字段 actions", spec.Name)
	}

	title := spec.Title
	if title == "" {
		title = "用 " + spec.Name + " 打开"
	}
	icon, err := InitIcon(spec.Icon)
	if err != nil {
		return nil, fmt.Errorf("opener %q icon 解析失败: %w", spec.Name, err)
	}

	actions := make(map[Role]actionSpec, len(spec.Actions))
	for role, raw := range spec.Actions {
		slotCount, ok := roleSlotCount(role)
		if !ok {
			return nil, fmt.Errorf("opener %q 存在未知 role %q（合法值：%s）", spec.Name, role, RolesString(RoleOrder()))
		}
		kind, tmpl, err := parseAction(raw)
		if err != nil {
			return nil, fmt.Errorf("opener %q role %s 动作非法: %w", spec.Name, role, err)
		}
		action := actionSpec{kind: kind, raw: raw, tmpl: tmpl}
		switch kind {
		case KindExec:
			tokens, err := tokenizeCmd(tmpl)
			if err != nil {
				return nil, fmt.Errorf("opener %q role %s cmd 非法: %w", spec.Name, role, err)
			}
			if len(tokens) == 0 {
				return nil, fmt.Errorf("opener %q role %s 缺少 cmd", spec.Name, role)
			}
			action.tokens = tokens
		case KindURL:
			if err := validateURLTemplate(tmpl); err != nil {
				return nil, fmt.Errorf("opener %q role %s 动作非法: %w", spec.Name, role, err)
			}
			if strings.HasPrefix(tmpl, "/") && baseURL == "" {
				return nil, fmt.Errorf("opener %q role %s 站内路由需要 baseURL（server 装配注入）", spec.Name, role)
			}
		}
		for _, n := range scanPlaceholders(tmpl) {
			if n >= slotCount {
				return nil, fmt.Errorf("opener %q role %s 动作占位符 $%d 越界（槽个数=%d，合法索引 0..%d）", spec.Name, role, n, slotCount, slotCount-1)
			}
		}
		actions[role] = action
	}

	// executor 默认值
	if executor == nil {
		executor = NewDefaultExecutor()
	}
	return &actionOpener{
		name:     spec.Name,
		title:    title,
		actions:  actions,
		baseURL:  baseURL,
		icon:     icon,
		executor: executor,
	}, nil
}

func (o *actionOpener) Name() string  { return o.name }
func (o *actionOpener) Title() string { return o.title }
func (o *actionOpener) Icon() Icon    { return o.icon }

// Roles 声明的 role 列表（按枚举固定序），= actions 的键集合。
func (o *actionOpener) Roles() []Role {
	roles := make([]Role, 0, len(o.actions))
	for _, r := range RoleOrder() {
		if _, ok := o.actions[r]; ok {
			roles = append(roles, r)
		}
	}
	return roles
}

// Actions 各 role 的动作串原文（编辑表单回显用）。
func (o *actionOpener) Actions() map[Role]string {
	out := make(map[Role]string, len(o.actions))
	for r, a := range o.actions {
		out[r] = a.raw
	}
	return out
}

// Summary 展示串（CLI 表格 / alfred 副标题 / Web DTO）：
// 按 role 固定序拼接 "role:动作串"。
func (o *actionOpener) Summary() string {
	parts := make([]string, 0, len(o.actions))
	for _, r := range o.Roles() {
		parts = append(parts, string(r)+":"+o.actions[r].raw)
	}
	return strings.Join(parts, "; ")
}

// BuildArgs 构造以指定 role 启动该 opener 的完整命令参数（bin + args）。
//   - 该 role 必须已声明、参数个数必须等于其槽个数，否则报错；
//   - exec 动作：cmd 中的 $0/$1... 占位符被对应路径替换；无占位符的参数原样保留；
//     缺省（cmd 未含占位符时）路径按顺序追加到 args 末尾，兼容 "code" + path 形态；
//     bin 自引用替换（见 resolveBin）：cmd[0] 是 cube 时换成当前进程的可执行文件；
//   - url 动作：站内路由拼 baseURL + 占位符 query encode；外部 URL 占位符照常替换；
//     bin 为系统 opener（macOS: open），args 为渲染后的完整 URL。
//
// 返回 (bin, args) 供调用方自行启动子进程。
// Open() 是它的便捷封装（role 校验 + BuildArgs + executor.Run）。
func (o *actionOpener) BuildArgs(role Role, slotArgs ...string) (bin string, args []string, err error) {
	action, ok := o.actions[role]
	if !ok {
		return "", nil, fmt.Errorf("opener %s 不支持 %s", o.name, role)
	}
	slotCount, _ := roleSlotCount(role)
	if len(slotArgs) != slotCount {
		return "", nil, fmt.Errorf("opener %s 的 %s 需要 %d 个路径参数，实际传入 %d", o.name, role, slotCount, len(slotArgs))
	}

	if action.kind == KindURL {
		var full string
		if strings.HasPrefix(action.tmpl, "/") {
			full = o.baseURL + mustRender(action.tmpl, slotArgs, true)
		} else {
			full = mustRender(action.tmpl, slotArgs, false)
		}
		return sysOpenBin(), []string{full}, nil
	}

	// 先对整条 cmd 渲染占位符（cmd[0] 也可能是占位符，如用路径本身作可执行文件），
	// 再取 [0] 为 bin、[1:] 为 args。
	rendered, used := renderArgs(action.tokens, slotArgs)
	// cmd 参数里没有任何占位符时，把路径按顺序追加到末尾（兼容纯 "code" 配置）
	if used == 0 {
		rendered = append(rendered, slotArgs...)
	}
	bin = resolveBin(rendered[0])
	args = rendered[1:]
	return bin, args, nil
}

// resolveBin 自引用替换：bin 为 "cube" 或以 "/cube" 结尾时，换成当前进程的可执行文件。
// opener 可组合 cube 自身 CLI（如 "cube ui workbench $0"），dev 环境（air/run.sh
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
func (o *actionOpener) Open(role Role, slotArgs ...string) error {
	if _, ok := o.actions[role]; !ok {
		return fmt.Errorf("opener %s 不支持 %s", o.name, role)
	}
	bin, args, err := o.BuildArgs(role, slotArgs...)
	if err != nil {
		return err
	}
	slog.Debug("actionOpener.Open", "bin", bin, "args", args)
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
