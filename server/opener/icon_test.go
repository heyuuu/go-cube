package opener

import "testing"

func TestInitIcon(t *testing.T) {
	cases := []struct {
		name    string
		icon    *Icon
		wantErr bool
	}{
		{"nil 视为未配置", nil, false},
		{"lucide 合法", &Icon{Type: IconTypeLucide, Value: "folder-open"}, false},
		{"image 合法", &Icon{Type: IconTypeImage, Value: "aGVsbG8="}, false},
		{"未知 type 报错", &Icon{Type: "svg", Value: "x"}, true},
		{"value 为空报错", &Icon{Type: IconTypeLucide, Value: ""}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := InitIcon(c.icon)
			if c.wantErr {
				if err == nil {
					t.Fatalf("期望报错, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("意外报错: %v", err)
			}
			if c.icon == nil && got.Type != "" {
				t.Fatalf("nil 应得零值, got %+v", got)
			}
		})
	}
}

func TestSpecIconThroughExec(t *testing.T) {
	t.Run("Spec 带 icon 透传到 Opener", func(t *testing.T) {
		o, err := InitExecOpener(Spec{
			Name: "code", Cmd: []string{"code"},
			Icon: &Icon{Type: IconTypeLucide, Value: "app-window"},
		}, nil)
		if err != nil {
			t.Fatalf("构造失败: %v", err)
		}
		if got := o.Icon(); got.Type != IconTypeLucide || got.Value != "app-window" {
			t.Fatalf("icon 透传失败: %+v", got)
		}
	})

	t.Run("无 icon 得零值", func(t *testing.T) {
		o, err := InitExecOpener(Spec{Name: "code", Cmd: []string{"code"}}, nil)
		if err != nil {
			t.Fatalf("构造失败: %v", err)
		}
		if got := o.Icon(); got != (Icon{}) {
			t.Fatalf("应得零值, got %+v", got)
		}
	})

	t.Run("坏 icon 构造报错（条目级降级由 Service 跳过）", func(t *testing.T) {
		if _, err := InitExecOpener(Spec{
			Name: "code", Cmd: []string{"code"},
			Icon: &Icon{Type: "bad"},
		}, nil); err == nil {
			t.Fatal("坏 icon 应报错")
		}
	})
}
