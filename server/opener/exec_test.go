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

func mkmk(pairs ...any) map[Role]string {
	m := map[Role]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i].(Role)] = pairs[i+1].(string)
	}
	return m
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
		commands map[Role]string
		role     Role
		paths    []string
		wantBin  string
		wantArgs []string
	}{
		{
			name:     "bin 在前 + 占位符参数",
			commands: mkmk(RoleOpenDir, "code $0"),
			role:     RoleOpenDir,
			paths:    []string{"/p"},
			wantBin:  "code",
			wantArgs: []string{"/p"},
		},
		{
			name:     "含空格路径引号包裹",
			commands: mkmk(RoleOpenDir, `"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code" $0`),
			role:     RoleOpenDir,
			paths:    []string{"/p"},
			wantBin:  "/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
			wantArgs: []string{"/p"},
		},
		{
			name:     "token 内嵌占位符",
			commands: mkmk(RoleOpenDir, "tool --wd=$0"),
			role:     RoleOpenDir,
			paths:    []string{"/p"},
			wantBin:  "tool",
			wantArgs: []string{"--wd=/p"},
		},
		{
			name:     "cmd[0] 本身是占位符（用路径作可执行文件）",
			commands: mkmk(RoleOpenFile, "$0"),
			role:     RoleOpenFile,
			paths:    []string{"/bin/sh"},
			wantBin:  "/bin/sh",
			wantArgs: []string{},
		},
		{
			name:     "无占位符：路径追加到末尾",
			commands: mkmk(RoleOpenDir, "code"),
			role:     RoleOpenDir,
			paths:    []string{"/p"},
			wantBin:  "code",
			wantArgs: []string{"/p"},
		},
		{
			name:     "对比工具：双槽",
			commands: mkmk(RoleDiffDir, "bcompare $0 $1", RoleDiffFile, "bcompare $0 $1"),
			role:     RoleDiffDir,
			paths:    []string{"/a", "/b"},
			wantBin:  "bcompare",
			wantArgs: []string{"/a", "/b"},
		},
		{
			name: "同 opener 不同 role 不同 cmd（vscode 场景，本次改造核心动机）",
			commands: mkmk(
				RoleOpenDir, "code $0",
				RoleDiffFile, "code --diff $0 $1",
			),
			role:     RoleDiffFile,
			paths:    []string{"/a", "/b"},
			wantBin:  "code",
			wantArgs: []string{"--diff", "/a", "/b"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o, err := InitExecOpener(Spec{Name: "t", Commands: c.commands}, &fakeExecutor{})
			if err != nil {
				t.Fatalf("InitExecOpener 失败: %v", err)
			}
			bin, args, err := o.BuildArgs(c.role, c.paths...)
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
			o, err := InitExecOpener(Spec{Name: "t", Commands: mkmk(RoleOpenDir, c.cmd)}, &fakeExecutor{})
			if err != nil {
				t.Fatalf("InitExecOpener 失败: %v", err)
			}
			bin, args, err := o.BuildArgs(RoleOpenDir, "/proj")
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
			o, err := InitExecOpener(Spec{Name: "t", Commands: mkmk(RoleOpenDir, bin0)}, &fakeExecutor{})
			if err != nil {
				t.Fatalf("InitExecOpener 失败: %v", err)
			}
			bin, _, err := o.BuildArgs(RoleOpenDir, "/proj")
			if err != nil {
				t.Fatalf("BuildArgs 失败: %v", err)
			}
			if bin != bin0 {
				t.Fatalf("%q 不应被替换, got %q", bin0, bin)
			}
		}
	})
}

// ---------- InitExecOpener commands 校验 ----------

func TestInitOpenerCommandsValidation(t *testing.T) {
	cases := []struct {
		name     string
		commands map[Role]string
		wantErr  bool
	}{
		{"commands 缺失报错", nil, true},
		{"commands 空报错", map[Role]string{}, true},
		{"未知 role 报错", map[Role]string{"unknown-role": "code"}, true},
		{"单条 cmd 缺失报错", map[Role]string{RoleOpenDir: ""}, true},
		{"单条 cmd 纯空白报错", map[Role]string{RoleOpenDir: "   "}, true},
		{"cmd 引号未闭合报错", map[Role]string{RoleOpenDir: `code "abc`}, true},
		{"仅可执行文件(无占位符)合法", map[Role]string{RoleOpenDir: "code"}, false},
		{"占位符不越本 role 槽数合法", map[Role]string{RoleDiffFile: "bcompare $0 $1"}, false},
		{"占位符越本 role 槽数报错", map[Role]string{RoleOpenDir: "bcompare $0 $1"}, true},
		{"token 内嵌占位符同样校验越界", map[Role]string{RoleOpenDir: "tool --wd=$1"}, true},
		{"不同 role 槽数不同可并存（无一致性约束）", map[Role]string{RoleOpenDir: "code $0", RoleDiffFile: "code --diff $0 $1"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec := Spec{Name: "test", Commands: c.commands}
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
		commands map[Role]string
		role     Role
		paths    []string
		wantBin  string
		wantArgs []string
	}{
		{
			name:     "单槽占位符",
			commands: mkmk(RoleOpenDir, "code $0"),
			role:     RoleOpenDir,
			paths:    []string{"/proj"},
			wantBin:  "code",
			wantArgs: []string{"/proj"},
		},
		{
			name:     "无占位符路径追加末尾",
			commands: mkmk(RoleOpenDir, "code"),
			role:     RoleOpenDir,
			paths:    []string{"/proj"},
			wantBin:  "code",
			wantArgs: []string{"/proj"},
		},
		{
			name:     "双槽 diff 工具",
			commands: mkmk(RoleDiffDir, "bcompare $0 $1"),
			role:     RoleDiffDir,
			paths:    []string{"/a", "/b"},
			wantBin:  "bcompare",
			wantArgs: []string{"/a", "/b"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &fakeExecutor{}
			o, err := InitExecOpener(Spec{Name: "t", Commands: c.commands}, fake)
			if err != nil {
				t.Fatalf("InitExecOpener 失败: %v", err)
			}
			if err := o.Open(c.role, c.paths...); err != nil {
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
	o, err := InitExecOpener(Spec{Name: "t", Commands: mkmk(RoleOpenDir, "code $0")}, fake)
	if err != nil {
		t.Fatalf("InitExecOpener 失败: %v", err)
	}
	// open-dir 1 槽但传 2 个路径，应在 BuildArgs 阶段报错，executor 不被调用
	err = o.Open(RoleOpenDir, "/a", "/b")
	if err == nil {
		t.Fatalf("期望槽个数不匹配报错，实际 nil")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("期望 executor 未被调用，实际调用 %d 次", len(fake.calls))
	}
}

// ---------- Open role 前置校验（收敛在实现内） ----------

func TestOpenRoleUnsupported(t *testing.T) {
	fake := &fakeExecutor{}
	o, err := InitExecOpener(Spec{Name: "code", Commands: mkmk(RoleOpenDir, "code")}, fake)
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

// ---------- tokenizeCmd ----------

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

// ---------- scanPlaceholders ----------

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
