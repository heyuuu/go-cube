package opener

import (
	"runtime"
	"strings"
	"testing"
)

func TestInitWebOpener(t *testing.T) {
	cases := []struct {
		name    string
		spec    Spec
		wantErr bool
	}{
		{"合法 workbench", Spec{Name: "wb", Type: SpecTypeWeb, Target: "workbench", Roles: []string{"open-dir"}}, false},
		{"缺 target 报错", Spec{Name: "wb", Type: SpecTypeWeb}, true},
		{"未知 target 报错", Spec{Name: "wb", Type: SpecTypeWeb, Target: "settings"}, true},
		{"坏 roles 报错", Spec{Name: "wb", Type: SpecTypeWeb, Target: "workbench", Roles: []string{"nope"}}, true},
		{"坏 icon 报错", Spec{Name: "wb", Type: SpecTypeWeb, Target: "workbench", Icon: &Icon{Type: "bad"}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := InitWebOpener(c.spec, "http://localhost:6001", &fakeExecutor{})
			if c.wantErr && err == nil {
				t.Fatal("期望报错")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("意外报错: %v", err)
			}
		})
	}
}

func TestWebOpenerOpen(t *testing.T) {
	makeOpener := func(baseURL string) *webOpener {
		t.Helper()
		w, err := InitWebOpener(Spec{Name: "wb", Type: SpecTypeWeb, Target: "workbench", Roles: []string{"open-dir"}}, baseURL, nil)
		if err != nil {
			t.Fatalf("构造失败: %v", err)
		}
		return w
	}

	t.Run("未声明 role 报错且不触达 executor", func(t *testing.T) {
		fake := &fakeExecutor{}
		w := makeOpener("http://localhost:6001")
		w.executor = fake
		if err := w.Open(RoleDiffDir, "/a"); err == nil {
			t.Fatal("期望 role 报错")
		}
		if len(fake.calls) != 0 {
			t.Fatal("executor 不应被调用")
		}
	})

	t.Run("baseURL 缺失报错", func(t *testing.T) {
		if err := makeOpener("").Open(RoleOpenDir, "/a"); err == nil {
			t.Fatal("期望 baseURL 缺失报错")
		}
	})

	t.Run("缺路径参数报错", func(t *testing.T) {
		if err := makeOpener("http://x").Open(RoleOpenDir); err == nil {
			t.Fatal("期望参数个数报错")
		}
	})

	t.Run("darwin 下打开浏览器 URL 带转义 path", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("仅 darwin 验证浏览器唤起命令")
		}
		fake := &fakeExecutor{}
		w := makeOpener("http://localhost:6001")
		w.executor = fake
		if err := w.Open(RoleOpenDir, "/Users/x/my proj"); err != nil {
			t.Fatalf("Open 失败: %v", err)
		}
		if len(fake.calls) != 1 {
			t.Fatalf("期望 1 次调用, got %d", len(fake.calls))
		}
		call := fake.calls[0]
		if call.bin != "open" {
			t.Fatalf("bin 应为 open, got %q", call.bin)
		}
		if !strings.HasPrefix(call.args[0], "http://localhost:6001/workbench?path=") {
			t.Fatalf("URL 前缀不符: %q", call.args[0])
		}
		if !strings.Contains(call.args[0], "my+proj") {
			t.Fatalf("path 应被 query 转义: %q", call.args[0])
		}
	})
}

func TestServiceDispatchWebSpec(t *testing.T) {
	s, _ := newServiceAt(t, []Spec{
		{Name: "finder", Cmd: []string{"/usr/bin/open", "$0"}},
		{Name: "wb", Type: SpecTypeWeb, Target: "workbench", Roles: []string{"open-dir"}},
		{Name: "badtype", Type: "ftp"},
		{Name: "badwb", Type: SpecTypeWeb, Target: "nope"},
	})
	got := s.AllOpeners()
	if len(got) != 2 {
		t.Fatalf("合法 opener 应 2 个（exec+web）, got %d", len(got))
	}
	wb := s.FindByName("wb")
	if wb == nil || wb.Kind() != SpecTypeWeb || wb.Summary() != "workbench" {
		t.Fatalf("web opener 构造或展示不符: %+v", wb)
	}
	if s.FindByName("badtype") != nil || s.FindByName("badwb") != nil {
		t.Fatal("坏条目应被跳过")
	}
	// web opener 也参与 role 筛选与模糊搜索
	if role := s.RoleOpeners(RoleOpenDir); len(role) != 2 {
		t.Fatalf("open-dir 应含 web opener, got %d", len(role))
	}
}
