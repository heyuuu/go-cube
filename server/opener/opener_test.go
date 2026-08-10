package opener

import (
	"reflect"
	"testing"

	"cube/config"
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

// ---------- placeholderIndex ----------

func TestPlaceholderIndex(t *testing.T) {
	cases := []struct {
		in     string
		wantN  int
		wantOK bool
	}{
		{"$0", 0, true},
		{"$1", 1, true},
		{"$12", 12, true},
		{"", 0, false},
		{"$x", 0, false},
		{"-old", 0, false},
		{"file$0", 0, false}, // 占位符必须整体
		{"$", 0, false},
		{"  $0", 0, false},
	}
	for _, c := range cases {
		n, ok := placeholderIndex(c.in)
		if n != c.wantN || ok != c.wantOK {
			t.Fatalf("placeholderIndex(%q) = (%d,%v), want (%d,%v)", c.in, n, ok, c.wantN, c.wantOK)
		}
	}
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
		cmd      []string
		roles    []string
		paths    []string
		wantBin  string
		wantArgs []string
	}{
		{
			name:     "bin 在前 + 占位符参数",
			cmd:      []string{"code", "$0"},
			roles:    []string{"open-dir"},
			paths:    []string{"/p"},
			wantBin:  "code",
			wantArgs: []string{"/p"},
		},
		{
			name:     "cmd[0] 本身是占位符（用路径作可执行文件）",
			cmd:      []string{"$0"},
			roles:    []string{"open-file"},
			paths:    []string{"/bin/sh"},
			wantBin:  "/bin/sh",
			wantArgs: []string{},
		},
		{
			name:     "无占位符：路径追加到末尾",
			cmd:      []string{"code"},
			roles:    []string{"open-dir"},
			paths:    []string{"/p"},
			wantBin:  "code",
			wantArgs: []string{"/p"},
		},
		{
			name:     "对比工具：双槽",
			cmd:      []string{"bcompare", "$0", "$1"},
			roles:    []string{"diff-dir", "diff-file"},
			paths:    []string{"/a", "/b"},
			wantBin:  "bcompare",
			wantArgs: []string{"/a", "/b"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o, err := InitOpener(config.OpenerConfig{Name: "t", Cmd: c.cmd, Roles: c.roles}, &fakeExecutor{})
			if err != nil {
				t.Fatalf("InitOpener 失败: %v", err)
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

// ---------- InitOpener cmd 校验 ----------

func TestInitOpenerCmdValidation(t *testing.T) {
	cases := []struct {
		name    string
		cmd     []string
		roles   []string
		wantErr bool
	}{
		{"cmd 缺失报错", nil, nil, true},
		{"cmd 空数组报错", []string{}, nil, true},
		{"cmd 仅可执行文件(无占位符)合法", []string{"code"}, nil, false},
		{"cmd 含合法占位符", []string{"bcompare", "$0", "$1"}, []string{"diff-file"}, false},
		{"cmd 占位符越界报错", []string{"bcompare", "$0", "$1"}, []string{"open-dir"}, true},
		{"cmd 占位符等于slotCount边界合法", []string{"code", "$0"}, []string{"open-dir"}, false},
		{"未知 role 报错", []string{"code"}, []string{"unknown-role"}, true},
		{"slotCount 不一致报错", []string{"code", "$0", "$1"}, []string{"open-dir", "diff-file"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			oc := config.OpenerConfig{Name: "test", Cmd: c.cmd, Roles: c.roles}
			o, err := InitOpener(oc, &fakeExecutor{})
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

// ---------- CanOpenAt glob 匹配 ----------
//
// 注：旧的 CanOpenAt(idx, typ, name) 已重构为 SupportTypeAt(idx, typ)（只看声明类型）
// 与 SupportPathAt(idx, path)（按真实路径 LStat + glob 匹配）。glob 命中/不命中的覆盖
// 已迁移到 filter_test.go 的 TestForPathValue（用真实临时文件验证），此处不再重复。

// ---------- Open 全链路（注入 fakeExecutor）----------

func TestOpenInvokesExecutor(t *testing.T) {
	cases := []struct {
		name     string
		cmd      []string
		roles    []string
		paths    []string
		wantBin  string
		wantArgs []string
	}{
		{
			name:     "单槽占位符",
			cmd:      []string{"code", "$0"},
			roles:    []string{"open-dir"},
			paths:    []string{"/proj"},
			wantBin:  "code",
			wantArgs: []string{"/proj"},
		},
		{
			name:     "无占位符路径追加末尾",
			cmd:      []string{"code"},
			roles:    []string{"open-dir"},
			paths:    []string{"/proj"},
			wantBin:  "code",
			wantArgs: []string{"/proj"},
		},
		{
			name:     "双槽 diff 工具",
			cmd:      []string{"bcompare", "$0", "$1"},
			roles:    []string{"diff-dir"},
			paths:    []string{"/a", "/b"},
			wantBin:  "bcompare",
			wantArgs: []string{"/a", "/b"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &fakeExecutor{}
			o, err := InitOpener(config.OpenerConfig{Name: "t", Cmd: c.cmd, Roles: c.roles}, fake)
			if err != nil {
				t.Fatalf("InitOpener 失败: %v", err)
			}
			if err := o.Open(c.paths...); err != nil {
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
	o, err := InitOpener(config.OpenerConfig{Name: "t", Cmd: []string{"code", "$0"}, Roles: []string{"open-dir"}}, fake)
	if err != nil {
		t.Fatalf("InitOpener 失败: %v", err)
	}
	// slotCount=1 但传 2 个路径，应在 BuildArgs 阶段报错，executor 不被调用
	err = o.Open("/a", "/b")
	if err == nil {
		t.Fatalf("期望 slotCount 不匹配报错，实际 nil")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("期望 executor 未被调用，实际调用 %d 次", len(fake.calls))
	}
}
