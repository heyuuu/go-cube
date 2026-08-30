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
	return NewService(file, fake, ""), fake
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
			{Name: "code", Actions: map[Role]string{"open-dir": `exec: code`}},
			{Name: "bcompare", Actions: map[Role]string{"diff-dir": `exec: bcompare $0 $1`}},
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
			{Name: "bad", Actions: map[Role]string{"open-dir": `exec:`}},       // 缺 cmd，条目级坏
			{Name: "code", Actions: map[Role]string{"open-dir": `exec: code`}}, // 正常
			{Name: "bad2", Actions: map[Role]string{"open-dir": `exec: c $9`}}, // 占位符越界，条目级坏
		})
		got := s.AllOpeners()
		if len(got) != 1 || got[0].Name() != "code" {
			t.Fatalf("期望只剩 code, got %v", got)
		}
	})

	t.Run("RoleOpeners 过滤", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{
			{Name: "code", Actions: map[Role]string{"open-dir": `exec: code`}},
			{Name: "bcompare", Actions: map[Role]string{"diff-dir": `exec: bcompare $0 $1`}},
		})
		if got := s.RoleOpeners(RoleDiffDir); len(got) != 1 || got[0].Name() != "bcompare" {
			t.Fatalf("diff-dir 应只有 bcompare, got %v", got)
		}
	})

	t.Run("SearchAll 模糊匹配", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{
			{Name: "code", Actions: map[Role]string{"open-dir": `exec: code`}},
			{Name: "vscode", Actions: map[Role]string{"open-dir": `exec: vscode`}},
		})
		if got := s.SearchAll("cod"); len(got) != 2 {
			t.Fatalf("cod 应匹配 code+vscode, got %v", got)
		}
	})

	t.Run("保存后无需重建 Service 即生效（直读不缓存）", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{{Name: "code", Actions: map[Role]string{"open-dir": `exec: code`}}})
		if err := settings.SaveSection(s.settingsFile, settingsSection,
			[]Spec{{Name: "newone", Actions: map[Role]string{"open-dir": `exec: newone`}}}); err != nil {
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
		s, _ := newServiceAt(t, []Spec{{Name: "code", Actions: map[Role]string{"open-dir": `exec: code`}}})
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
		{Name: "code", Actions: map[Role]string{"open-dir": `exec: code`}, Icon: &Icon{Type: IconTypeLucide, Value: "app-window"}},
		{Name: "plain", Actions: map[Role]string{"open-dir": `exec: plain`}},
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
		if err := s.SaveOpener(Spec{Name: "code", Actions: map[Role]string{"open-dir": `exec: code`}}); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
		if o := s.FindByName("code"); o == nil {
			t.Fatal("保存后应可读回")
		}
	})

	t.Run("按名替换不重复", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{{Name: "code", Actions: map[Role]string{"open-dir": `exec: code`}}})
		if err := s.SaveOpener(Spec{Name: "code", Actions: map[Role]string{"open-dir": `exec: code $0`}}); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
		if got := s.AllOpeners(); len(got) != 1 {
			t.Fatalf("应仍为 1 条, got %d", len(got))
		}
		if o := s.FindByName("code"); o.Summary() != "open-dir:exec: code $0" {
			t.Fatalf("内容应被替换, got %q", o.Summary())
		}
	})

	t.Run("坏数据返回中文错误且不落文件", func(t *testing.T) {
		s, _ := newServiceAt(t, nil)
		if err := s.SaveOpener(Spec{Name: "bad", Actions: map[Role]string{"open-dir": `exec:`}}); err == nil {
			t.Fatal("缺 cmd 应报错")
		}
		if err := s.SaveOpener(Spec{Name: "", Actions: map[Role]string{"open-dir": `exec: x`}}); err == nil {
			t.Fatal("空 name 应报错")
		}
		if got := s.AllOpeners(); len(got) != 0 {
			t.Fatalf("坏数据不应落文件, got %d", len(got))
		}
	})

	t.Run("删除与不存在报错", func(t *testing.T) {
		s, _ := newServiceAt(t, []Spec{{Name: "code", Actions: map[Role]string{"open-dir": `exec: code`}}})
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
		{Name: "finder", Actions: map[Role]string{"open-dir": `exec: open -a Finder`}},
		{Name: "code", Actions: map[Role]string{"open-dir": `exec: code`}},
		{Name: "stree", Actions: map[Role]string{"open-dir": `exec: stree`}},
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

// ---------- intents（openerIntents 节，1038） ----------

// writeIntents 写测试 openerIntents 节。
func writeIntents(t *testing.T, file string, intents map[Intent]IntentSpec) {
	t.Helper()
	if err := settings.SaveSection(file, intentsSection, intents); err != nil {
		t.Fatalf("写入测试 openerIntents 失败: %v", err)
	}
}

func mkSpecs(pairs ...any) []Spec {
	specs := make([]Spec, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		specs = append(specs, Spec{Name: pairs[i].(string), Actions: pairs[i+1].(map[Role]string)})
	}
	return specs
}

func TestIntentsSynthesis(t *testing.T) {
	dirOnly := mkSpecs("finder", mkmk(RoleOpenDir, "finder"), "code", mkmk(RoleOpenDir, "code $0", RoleOpenFile, "code $0"))
	s, _ := newServiceAt(t, dirOnly)
	writeIntents(t, s.settingsFile, map[Intent]IntentSpec{
		IntentDir:       {DefaultOpener: "finder"},
		IntentGit:       {DefaultOpener: "stree"},               // opener 不存在 → 读侧忽略
		IntentDoc:       {DefaultOpener: "finder"},              // role 不符（doc→open-file）→ 读侧忽略
		IntentTerminal:  {Openers: []string{"code", "ghostty"}}, // ghostty 不存在 → 跳过，保留 code
		Intent("bogus"): {DefaultOpener: "finder"},              // 未知 intent 键 → 跳过
	})

	got := map[Intent]IntentInfo{}
	for _, info := range s.Intents() {
		got[info.Intent] = info
	}

	if d := got[IntentDir]; d.DefaultOpener != "finder" {
		t.Fatalf("dir 默认应为 finder, got %+v", d)
	}
	if d := got[IntentGit]; d.DefaultOpener != "" {
		t.Fatalf("不存在的默认 opener 应被忽略, got %+v", d)
	}
	if d := got[IntentDoc]; d.DefaultOpener != "" {
		t.Fatalf("role 不符的默认 opener 应被忽略, got %+v", d)
	}
	if d := got[IntentTerminal]; len(d.Openers) != 1 || d.Openers[0] != "code" {
		t.Fatalf("terminal 候选应只剩 code, got %+v", d)
	}
	// 缺省候选 = 声明对应 role 的全部 opener（openers 节顺序）
	if d := got[IntentFile]; len(d.Openers) != 1 || d.Openers[0] != "code" {
		t.Fatalf("file 缺省候选应只有 code, got %+v", d)
	}
}

func TestSaveDeleteIntentDefault(t *testing.T) {
	s, _ := newServiceAt(t, mkSpecs("finder", mkmk(RoleOpenDir, "finder"), "code", mkmk(RoleOpenDir, "code $0", RoleOpenFile, "code $0")))

	// 写侧校验：role 不符 / 不存在 报错不落盘
	if err := s.SaveIntentDefault(IntentDoc, "finder"); err == nil {
		t.Fatal("finder 无 open-file，应报错")
	}
	if err := s.SaveIntentDefault(IntentDir, "nope"); err == nil {
		t.Fatal("不存在的 opener 应报错")
	}
	if _, err := s.DefaultOpener(IntentDir); err == nil {
		t.Fatal("未配置默认应报错")
	}

	if err := s.SaveIntentDefault(IntentDir, "code"); err != nil {
		t.Fatalf("保存默认失败: %v", err)
	}
	o, err := s.DefaultOpener(IntentDir)
	if err != nil || o.Name() != "code" {
		t.Fatalf("DefaultOpener(dir) = (%v,%v)", o, err)
	}

	if err := s.DeleteIntentDefault(IntentDir); err != nil {
		t.Fatalf("删除默认失败: %v", err)
	}
	if _, err := s.DefaultOpener(IntentDir); err == nil {
		t.Fatal("删除后应视为未配置")
	}
	if err := s.DeleteIntentDefault(IntentDir); err == nil {
		t.Fatal("重复删除应报错")
	}
}

// DeleteOpener 连带清理 intents 引用（默认 + 候选）
func TestDeleteOpenerCleansIntents(t *testing.T) {
	s, _ := newServiceAt(t, mkSpecs("finder", mkmk(RoleOpenDir, "finder"), "code", mkmk(RoleOpenDir, "code $0")))
	writeIntents(t, s.settingsFile, map[Intent]IntentSpec{
		IntentDir:      {DefaultOpener: "finder", Openers: []string{"finder", "code"}},
		IntentTerminal: {Openers: []string{"finder"}},
	})

	if err := s.DeleteOpener("finder"); err != nil {
		t.Fatalf("删除 opener 失败: %v", err)
	}
	specs := s.loadIntents()
	if d, ok := specs[IntentDir]; !ok || d.DefaultOpener != "" || len(d.Openers) != 1 || d.Openers[0] != "code" {
		t.Fatalf("dir 引用应清理为 {无默认, [code]}, got %+v", d)
	}
	// terminal 只剩 finder 引用，清空后整键删除
	if _, ok := specs[IntentTerminal]; ok {
		t.Fatalf("terminal 应整键删除, got %+v", specs[IntentTerminal])
	}
}
