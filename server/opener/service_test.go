package opener

import (
	"testing"

	"cube/internal/testfixture"
	"cube/settings"
)

// newServiceAt 建一个指向测试专属 settings.json 的 Service（注入 fakeExecutor）。
func newServiceAt(t *testing.T, specs []Spec) (*Service, *fakeExecutor) {
	t.Helper()
	ws := testfixture.NewWorkspace(t)
	file := ws.Join("settings.json")
	if specs != nil {
		if err := settings.SaveSection(file, settingsSection, specs); err != nil {
			t.Fatalf("写入测试 settings.json 失败: %v", err)
		}
	}
	fake := &fakeExecutor{}
	return NewService(file, fake), fake
}

func TestServiceDirectRead(t *testing.T) {
	t.Run("文件不存在返回空列表", func(t *testing.T) {
		s, _ := newServiceAt(t, nil)
		if got := s.AllOpeners(); len(got) != 0 {
			t.Fatalf("期望空列表, got %v", got)
		}
	})

	t.Run("读取全部与按名查找", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{
			{Name: "code", Cmd: []string{"code"}},
			{Name: "bcompare", Cmd: []string{"bcompare", "$0", "$1"}, Roles: []string{"diff-dir"}},
		})
		if got := s.AllOpeners(); len(got) != 2 {
			t.Fatalf("期望 2 个, got %d", len(got))
		}
		if o := s.FindByName("bcompare"); o == nil {
			t.Fatal("FindByName(bcompare) 应命中")
		}
		if o := s.FindByName("nope"); o != nil {
			t.Fatalf("FindByName(nope) 不应命中, got %v", o)
		}
	})

	t.Run("坏条目跳过不阻断", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{
			{Name: "bad", Cmd: nil},                  // 缺 cmd，条目级坏
			{Name: "code", Cmd: []string{"code"}},    // 正常
			{Name: "bad2", Cmd: []string{"c", "$9"}}, // 占位符越界，条目级坏
		})
		got := s.AllOpeners()
		if len(got) != 1 || got[0].Name() != "code" {
			t.Fatalf("期望只剩 code, got %v", got)
		}
	})

	t.Run("RoleOpeners 过滤", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{
			{Name: "code", Cmd: []string{"code"}},
			{Name: "bcompare", Cmd: []string{"bcompare", "$0", "$1"}, Roles: []string{"diff-dir"}},
		})
		if got := s.RoleOpeners(RoleDiffDir); len(got) != 1 || got[0].Name() != "bcompare" {
			t.Fatalf("diff-dir 应只有 bcompare, got %v", got)
		}
	})

	t.Run("SearchAll 模糊匹配", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{
			{Name: "code", Cmd: []string{"code"}},
			{Name: "vscode", Cmd: []string{"vscode"}},
		})
		if got := s.SearchAll("cod"); len(got) != 2 {
			t.Fatalf("cod 应匹配 code+vscode, got %v", got)
		}
	})

	t.Run("保存后无需重建 Service 即生效（直读不缓存）", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{{Name: "code", Cmd: []string{"code"}}})
		if err := settings.SaveSection(s.settingsFile, settingsSection,
			[]Spec{{Name: "newone", Cmd: []string{"newone"}}}); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		if o := s.FindByName("newone"); o == nil {
			t.Fatal("新条目应立即可见（直读）")
		}
		if o := s.FindByName("code"); o != nil {
			t.Fatalf("旧条目应已消失, got %v", o)
		}
	})

	t.Run("其他节不受影响", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{{Name: "code", Cmd: []string{"code"}}})
		if err := settings.SaveSection(s.settingsFile, "other", map[string]int{"a": 1}); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		if got := s.AllOpeners(); len(got) != 1 || got[0].Name() != "code" {
			t.Fatalf("写其他节不应影响 openers, got %v", got)
		}
	})
}

func TestServiceIconEndToEnd(t *testing.T) {
	// icon 随 Spec 存进 settings.json，读出后透传到 Opener 接口
	s, _ := newServiceAt(t, []Spec{
		{Name: "code", Cmd: []string{"code"}, Icon: &Icon{Type: IconTypeLucide, Value: "app-window"}},
		{Name: "plain", Cmd: []string{"plain"}},
	})
	o := s.FindByName("code")
	if o == nil || o.Icon().Value != "app-window" {
		t.Fatalf("icon 未透传: %+v", o)
	}
	if p := s.FindByName("plain"); p.Icon().Value != "app-window-mac" {
		t.Fatalf("无 icon 应得默认 app-window-mac, got %+v", p.Icon())
	}
}

func TestServiceSaveDelete(t *testing.T) {
	t.Run("新增后可读回", func(t *testing.T) {
		s, _ := newServiceAt(t, nil)
		if err := s.SaveOpener(Spec{Name: "code", Cmd: []string{"code"}}); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
		if o := s.FindByName("code"); o == nil {
			t.Fatal("保存后应可读回")
		}
	})

	t.Run("按名替换不重复", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{{Name: "code", Cmd: []string{"code"}}})
		if err := s.SaveOpener(Spec{Name: "code", Cmd: []string{"code", "$0"}}); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
		if got := s.AllOpeners(); len(got) != 1 {
			t.Fatalf("应仍为 1 条, got %d", len(got))
		}
		if o := s.FindByName("code"); o.Summary() != "code $0" {
			t.Fatalf("内容应被替换, got %q", o.Summary())
		}
	})

	t.Run("坏数据返回中文错误且不落文件", func(t *testing.T) {
		s, _ := newServiceAt(t, nil)
		if err := s.SaveOpener(Spec{Name: "bad", Cmd: nil}); err == nil {
			t.Fatal("缺 cmd 应报错")
		}
		if err := s.SaveOpener(Spec{Name: "", Cmd: []string{"x"}}); err == nil {
			t.Fatal("空 name 应报错")
		}
		if got := s.AllOpeners(); len(got) != 0 {
			t.Fatalf("坏数据不应落文件, got %d", len(got))
		}
	})

	t.Run("删除与不存在报错", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{{Name: "code", Cmd: []string{"code"}}})
		if err := s.DeleteOpener("code"); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if got := s.AllOpeners(); len(got) != 0 {
			t.Fatalf("删除后应为空, got %d", len(got))
		}
		if err := s.DeleteOpener("nope"); err == nil {
			t.Fatal("删除不存在项应报错")
		}
	})
}

func TestServiceReorder(t *testing.T) {
	newSpecs := []Spec{
		{Name: "finder", Cmd: []string{"open", "-a", "Finder"}},
		{Name: "code", Cmd: []string{"code"}},
		{Name: "stree", Cmd: []string{"stree"}},
	}

	t.Run("按名单重排", func(t *testing.T) {
		s, _ := newServiceAt(t, newSpecs)
		if err := s.ReorderOpeners([]string{"stree", "code", "finder"}); err != nil {
			t.Fatalf("重排失败: %v", err)
		}
		got := s.AllOpeners()
		if len(got) != 3 || got[0].Name() != "stree" || got[1].Name() != "code" || got[2].Name() != "finder" {
			t.Fatalf("顺序不符: %v", got)
		}
	})

	t.Run("未列名条目保持原序排在末尾", func(t *testing.T) {
		s, _ := newServiceAt(t, newSpecs)
		if err := s.ReorderOpeners([]string{"code"}); err != nil {
			t.Fatalf("重排失败: %v", err)
		}
		got := s.AllOpeners()
		if len(got) != 3 || got[0].Name() != "code" || got[1].Name() != "finder" || got[2].Name() != "stree" {
			t.Fatalf("未列名条目应原序殿后: %v", got)
		}
	})

	t.Run("未知名与重复名报错且不落文件", func(t *testing.T) {
		s, _ := newServiceAt(t, newSpecs)
		if err := s.ReorderOpeners([]string{"code", "nope"}); err == nil {
			t.Fatal("未知名应报错")
		}
		if err := s.ReorderOpeners([]string{"code", "code"}); err == nil {
			t.Fatal("重复名应报错")
		}
		got := s.AllOpeners()
		if len(got) != 3 || got[0].Name() != "finder" {
			t.Fatalf("失败重排不应改动文件: %v", got)
		}
	})
}
