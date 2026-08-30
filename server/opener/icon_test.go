package opener

import "testing"

func TestInitIcon(t *testing.T) {
	cases := []struct {
		name    string
		icon    *Icon
		wantErr bool
	}{
		{"lucide 合法", &Icon{Type: IconTypeLucide, Value: "folder-open"}, false},
		{"image 合法", &Icon{Type: IconTypeImage, Value: "aGVsbG8="}, false},
		{"未知 type 报错", &Icon{Type: "svg", Value: "x"}, true},
		{"value 为空报错", &Icon{Type: IconTypeLucide, Value: ""}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := InitIcon(c.icon); c.wantErr != (err != nil) {
				t.Fatalf("wantErr=%v, err=%v", c.wantErr, err)
			}
		})
	}
}

func TestInitIconDefault(t *testing.T) {
	got, err := InitIcon(nil)
	if err != nil {
		t.Fatalf("意外报错: %v", err)
	}
	if got.Type != IconTypeLucide || got.Value != "app-window-mac" {
		t.Fatalf("默认 icon 应为 lucide:app-window-mac, got %+v", got)
	}
}

func TestSpecIconThroughExec(t *testing.T) {
	t.Run("Spec 带 icon 透传到 Opener", func(t *testing.T) {
		o, err := InitExecOpener(Spec{
			Name: "code", Commands: map[Role]string{"open-dir": `code`},
			Icon: &Icon{Type: IconTypeLucide, Value: "app-window"},
		}, nil)
		if err != nil {
			t.Fatalf("构造失败: %v", err)
		}
		if got := o.Icon(); got.Type != IconTypeLucide || got.Value != "app-window" {
			t.Fatalf("icon 透传失败: %+v", got)
		}
	})

	t.Run("无 icon 得默认", func(t *testing.T) {
		o, err := InitExecOpener(Spec{Name: "code", Commands: map[Role]string{"open-dir": `code`}}, nil)
		if err != nil {
			t.Fatalf("构造失败: %v", err)
		}
		if got := o.Icon(); got.Type != IconTypeLucide || got.Value != "app-window-mac" {
			t.Fatalf("应得默认 app-window-mac, got %+v", got)
		}
	})

	t.Run("坏 icon 构造报错（条目级降级由 Service 跳过）", func(t *testing.T) {
		if _, err := InitExecOpener(Spec{
			Name: "code", Commands: map[Role]string{"open-dir": `code`},
			Icon: &Icon{Type: "bad"},
		}, nil); err == nil {
			t.Fatal("坏 icon 应报错")
		}
	})
}

func TestTitleDefault(t *testing.T) {
	o1, _ := InitExecOpener(Spec{Name: "code", Commands: map[Role]string{"open-dir": `code`}}, nil)
	if got := o1.Title(); got != "用 code 打开" {
		t.Fatalf("缺省 title 应由 name 生成, got %q", got)
	}
	o2, _ := InitExecOpener(Spec{Name: "finder", Title: "打开所在目录", Commands: map[Role]string{"open-dir": `open $0`}}, nil)
	if got := o2.Title(); got != "打开所在目录" {
		t.Fatalf("配置 title 应原样生效, got %q", got)
	}
}
