package opener

import (
	"os"
	"reflect"
	"testing"
)

// fakeExecutor 记录 Run 调用的 bin+args，不真正启动子进程。供 Open 全链路测试用。
type fakeExecutor struct {
	calls []fakeCall
}
type fakeCall struct {
	bin  string
	args []string
}

func (f *fakeExecutor) Run(bin string, args ...string) error {
	f.calls = append(f.calls, fakeCall{bin: bin, args: append([]string(nil), args...)})
	return nil
}

// ---------- renderArgs ----------

func TestRenderArgs(t *testing.T) {
	paths := []string{"/a", "/b"}
	cases := []struct {
		name string
		args []string
		want []string
		used int
	}{
		{"无占位符原样保留", []string{"-old", "/x"}, []string{"-old", "/x"}, 0},
		{"单占位符", []string{"$0"}, []string{"/a"}, 1},
		{"flag + 占位符混合", []string{"-old", "$0", "-new", "$1"}, []string{"-old", "/a", "-new", "/b"}, 2},
		{"占位符乱序", []string{"$1", "$0"}, []string{"/b", "/a"}, 2},
		{"占位符越界保留原样", []string{"$0", "$9"}, []string{"/a", "$9"}, 1},
		{"token 内嵌占位符", []string{"--wd=$0", "x$1y"}, []string{"--wd=/a", "x/by"}, 2},
		{"多位数字占位符整体识别", []string{"$10"}, []string{"$10"}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, used := renderArgs(c.args, paths)
			if !reflect.DeepEqual(got, c.want) || used != c.used {
				t.Fatalf("renderArgs(%v) = (%v,%d), want (%v,%d)", c.args, got, used, c.want, c.used)
			}
		})
	}
}

// ---------- BuildArgs ----------

func TestBuildArgs(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		roles    []string
		paths    []string
		wantBin  string
		wantArgs []string
	}{
		{
			name:     "bin 在前 + 占位符参数",
			cmd:      "code $0",
			roles:    []string{"open-dir"},
			paths:    []string{"/p"},
			wantBin:  "code",
			wantArgs: []string{"/p"},
		},
		{
			name:     "含空格路径引号包裹",
			cmd:      `"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code" $0`,
			roles:    []string{"open-dir"},
			paths:    []string{"/p"},
			wantBin:  "/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
			wantArgs: []string{"/p"},
		},
		{
			name:     "token 内嵌占位符",
			cmd:      "tool --wd=$0",
			roles:    []string{"open-dir"},
			paths:    []string{"/p"},
			wantBin:  "tool",
			wantArgs: []string{"--wd=/p"},
		},
		{
			name:     "cmd[0] 本身是占位符（用路径作可执行文件）",
			cmd:      "$0",
			roles:    []string{"open-file"},
			paths:    []string{"/bin/sh"},
			wantBin:  "/bin/sh",
			wantArgs: []string{},
		},
		{
			name:     "无占位符：路径追加到末尾",
			cmd:      "code",
			roles:    []string{"open-dir"},
			paths:    []string{"/p"},
			wantBin:  "code",
			wantArgs: []string{"/p"},
		},
		{
			name:     "对比工具：双槽",
			cmd:      "bcompare $0 $1",
			roles:    []string{"diff-dir", "diff-file"},
			paths:    []string{"/a", "/b"},
			wantBin:  "bcompare",
			wantArgs: []string{"/a", "/b"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o, err := InitExecOpener(Spec{Name: "t", Cmd: c.cmd, Roles: c.roles}, &fakeExecutor{})
			if err != nil {
				t.Fatalf("InitExecOpener 失败: %v", err)
			}
			bin, args, err := o.BuildArgs(c.paths...)
			if err != nil {
				t.Fatalf("BuildArgs 失败: %v", err)
			}
			if bin != c.wantBin || !reflect.DeepEqual(args, c.wantArgs) {
				t.Fatalf("BuildArgs(%v) = (%q,%v), want (%q,%v)", c.paths, bin, args, c.wantBin, c.wantArgs)
			}
		})
	}
}

// ---------- cmd[0] 自引用替换（resolveBin） ----------

func TestBuildArgsResolvesSelfCube(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable 失败: %v", err)
	}
	cases := []struct {
		name     string
		cmd      string
		wantArgs []string
	}{
		{"裸名 cube", "cube ui workbench $0", []string{"ui", "workbench", "/proj"}},
		{"绝对路径 cube", "/usr/local/bin/cube md", []string{"md", "/proj"}},
		{"相对路径 cube", "tmp/cube md", []string{"md", "/proj"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o, err := InitExecOpener(Spec{Name: "t", Cmd: c.cmd, Roles: []string{"open-dir"}}, &fakeExecutor{})
			if err != nil {
				t.Fatalf("InitExecOpener 失败: %v", err)
			}
			bin, args, err := o.BuildArgs("/proj")
			if err != nil {
				t.Fatalf("BuildArgs 失败: %v", err)
			}
			if bin != self {
				t.Fatalf("cmd 应替换为当前可执行文件 %q, got %q", self, bin)
			}
			if !reflect.DeepEqual(args, c.wantArgs) {
				t.Fatalf("args = %v, want %v", args, c.wantArgs)
			}
		})
	}

	t.Run("近似名不替换", func(t *testing.T) {
		for _, bin0 := range []string{"cubed", "cube-x", "mycube", "/bin/cubecase"} {
			o, err := InitExecOpener(Spec{Name: "t", Cmd: bin0, Roles: []string{"open-dir"}}, &fakeExecutor{})
			if err != nil {
				t.Fatalf("InitExecOpener 失败: %v", err)
			}
			bin, _, err := o.BuildArgs("/proj")
			if err != nil {
				t.Fatalf("BuildArgs 失败: %v", err)
			}
			if bin != bin0 {
				t.Fatalf("%q 不应被替换, got %q", bin0, bin)
			}
		}
	})
}

// ---------- InitExecOpener cmd 校验 ----------

func TestInitOpenerCmdValidation(t *testing.T) {
	cases := []struct {
		name    string
		cmd     string
		roles   []string
		wantErr bool
	}{
		{"cmd 缺失报错", "", nil, true},
		{"cmd 纯空白报错", "   ", nil, true},
		{"cmd 引号未闭合报错", `code "abc`, nil, true},
		{"cmd 仅可执行文件(无占位符)合法", "code", nil, false},
		{"cmd 含合法占位符", "bcompare $0 $1", []string{"diff-file"}, false},
		{"cmd 占位符越界报错", "bcompare $0 $1", []string{"open-dir"}, true},
		{"token 内嵌占位符同样校验越界", "tool --wd=$1", []string{"open-dir"}, true},
		{"cmd 占位符等于slotCount边界合法", "code $0", []string{"open-dir"}, false},
		{"未知 role 报错", "code", []string{"unknown-role"}, true},
		{"slotCount 不一致报错", "code $0 $1", []string{"open-dir", "diff-file"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec := Spec{Name: "test", Cmd: c.cmd, Roles: c.roles}
			o, err := InitExecOpener(spec, &fakeExecutor{})
			if c.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际 o=%+v err=nil", o)
				}
				return
			}
			if err != nil {
				t.Fatalf("意外报错: %v", err)
			}
		})
	}
}

// ---------- Open 全链路（注入 fakeExecutor）----------

func TestOpenInvokesExecutor(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		roles    []string
		paths    []string
		wantBin  string
		wantArgs []string
	}{
		{
			name:     "单槽占位符",
			cmd:      "code $0",
			roles:    []string{"open-dir"},
			paths:    []string{"/proj"},
			wantBin:  "code",
			wantArgs: []string{"/proj"},
		},
		{
			name:     "无占位符路径追加末尾",
			cmd:      "code",
			roles:    []string{"open-dir"},
			paths:    []string{"/proj"},
			wantBin:  "code",
			wantArgs: []string{"/proj"},
		},
		{
			name:     "双槽 diff 工具",
			cmd:      "bcompare $0 $1",
			roles:    []string{"diff-dir"},
			paths:    []string{"/a", "/b"},
			wantBin:  "bcompare",
			wantArgs: []string{"/a", "/b"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &fakeExecutor{}
			o, err := InitExecOpener(Spec{Name: "t", Cmd: c.cmd, Roles: c.roles}, fake)
			if err != nil {
				t.Fatalf("InitExecOpener 失败: %v", err)
			}
			roles, _, err := ParseRoles(c.roles)
			if err != nil {
				t.Fatalf("roles 解析失败: %v", err)
			}
			if err := o.Open(roles[0], c.paths...); err != nil {
				t.Fatalf("Open 失败: %v", err)
			}
			if len(fake.calls) != 1 {
				t.Fatalf("期望 executor 被调用 1 次，实际 %d 次", len(fake.calls))
			}
			call := fake.calls[0]
			if call.bin != c.wantBin || !reflect.DeepEqual(call.args, c.wantArgs) {
				t.Fatalf("Open(%v) 执行 (%q,%v), 期望 (%q,%v)", c.paths, call.bin, call.args, c.wantBin, c.wantArgs)
			}
		})
	}
}

func TestOpenSlotCountMismatch(t *testing.T) {
	fake := &fakeExecutor{}
	o, err := InitExecOpener(Spec{Name: "t", Cmd: "code $0", Roles: []string{"open-dir"}}, fake)
	if err != nil {
		t.Fatalf("InitExecOpener 失败: %v", err)
	}
	// slotCount=1 但传 2 个路径，应在 BuildArgs 阶段报错，executor 不被调用
	err = o.Open(RoleOpenDir, "/a", "/b")
	if err == nil {
		t.Fatalf("期望 slotCount 不匹配报错，实际 nil")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("期望 executor 未被调用，实际调用 %d 次", len(fake.calls))
	}
}

// ---------- Open role 前置校验（收敛在实现内） ----------

func TestOpenRoleUnsupported(t *testing.T) {
	fake := &fakeExecutor{}
	o, err := InitExecOpener(Spec{Name: "code", Cmd: "code", Roles: []string{"open-dir"}}, fake)
	if err != nil {
		t.Fatalf("InitExecOpener 失败: %v", err)
	}
	// 用未声明的 role 调 Open 应在实现内报错，且不触达 executor
	if err := o.Open(RoleDiffDir, "/a"); err == nil {
		t.Fatal("期望 role 不支持报错")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("期望 executor 未被调用，实际调用 %d 次", len(fake.calls))
	}
}

func TestTokenizeCmd(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{"空串", "", nil},
		{"纯空白", "   ", nil},
		{"空格分隔", "code $0 -n", []string{"code", "$0", "-n"}},
		{"多空白折叠", "code   $0", []string{"code", "$0"}},
		{"双引号含空格", `"/Applications/Visual Studio Code.app/bin/code" $0`, []string{"/Applications/Visual Studio Code.app/bin/code", "$0"}},
		{"单引号含空格", `'a b' c`, []string{"a b", "c"}},
		{"双引号内转义引号", `"a\"b"`, []string{`a"b`}},
		{"裸词反斜杠转义", `a\ b`, []string{"a b"}},
		{"tab 分隔", "a\tb", []string{"a", "b"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := tokenizeCmd(c.line)
			if err != nil || !reflect.DeepEqual(got, c.want) {
				t.Fatalf("tokenizeCmd(%q) = (%v,%v), want (%v,nil)", c.line, got, err, c.want)
			}
		})
	}
	// 引号未闭合应报错（保存/加载边界拦截，不拖到运行期 exec 失败）
	for _, line := range []string{`"abc`, `'abc`, `a "b c`, `"a\"`} {
		if _, err := tokenizeCmd(line); err == nil {
			t.Fatalf("tokenizeCmd(%q) 引号未闭合应报错", line)
		}
	}
}

func TestScanPlaceholders(t *testing.T) {
	cases := []struct {
		token string
		want  []int
	}{
		{"$0", []int{0}},
		{"--wd=$1", []int{1}},
		{"a$0b$1", []int{0, 1}},
		{"$12", []int{12}},
		{"$x", nil},
		{"no placeholder", nil},
		{"$", nil},
	}
	for _, c := range cases {
		if got := scanPlaceholders(c.token); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("scanPlaceholders(%q) = %v, want %v", c.token, got, c.want)
		}
	}
}
