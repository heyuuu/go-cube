package opener

import (
	"reflect"
	"testing"

	"github.com/heyuuu/cube/config"
)

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
			o, err := InitOpener(config.OpenerConfig{Name: "t", Cmd: c.cmd, Roles: c.roles})
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
			o, err := InitOpener(oc)
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
// 与 SupportPathAt(idx, path)（按真实路径 Lstat + glob 匹配）。glob 命中/不命中的覆盖
// 已迁移到 filter_test.go 的 TestForPathValue（用真实临时文件验证），此处不再重复。
