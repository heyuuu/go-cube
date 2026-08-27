package project

import (
	"path"
	"strings"
	"testing"

	"cube/internal/testfixture"
)

// newWriteService 构造可写 settings 的 Service（无初始规则）。
func newWriteService(t *testing.T) (*Service, *testfixture.Workspace) {
	t.Helper()
	ws := testfixture.NewWorkspace(t)
	s := newServiceWithRules(t, ws, nil, nil)
	return s, ws
}

// TestSaveScanRule 新增与按键替换：同 path 替换、新 path 追加，保存后项目列表即时反映。
func TestSaveScanRule(t *testing.T) {
	s, ws := newWriteService(t)
	root := ws.Mkdir("root")
	ws.MakeProjectDir(path.Join("root", "p1"))

	if err := s.SaveScanRule(ScanRule{Group: "g", Path: root, MaxDepth: 3}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if rules := s.ScanRules(); len(rules) != 1 || rules[0].Group != "g" {
		t.Fatalf("保存后规则异常: %v", rules)
	}
	// 保存即重扫：无需手动 Refresh，列表立即反映新规则
	if projs := s.Projects(); len(projs) != 1 {
		t.Fatalf("保存后项目列表应即时重扫，实际 %d 个", len(projs))
	}

	// 同 path 替换（改 group）
	if err := s.SaveScanRule(ScanRule{Group: "g2", Path: root, MaxDepth: 3}); err != nil {
		t.Fatalf("替换失败: %v", err)
	}
	if rules := s.ScanRules(); len(rules) != 1 || rules[0].Group != "g2" {
		t.Fatalf("同 path 应替换而非追加: %v", rules)
	}
}

// TestSaveScanRule_Validation 坏数据返回中文错误、落不了文件。
func TestSaveScanRule_Validation(t *testing.T) {
	s, ws := newWriteService(t)
	root := ws.Mkdir("root")
	ws.WriteFile("f.txt", []byte("x"))

	cases := []struct {
		name string
		rule ScanRule
		want string
	}{
		{"group 为空", ScanRule{Path: root, MaxDepth: 3}, "group"},
		{"maxDepth 非正", ScanRule{Group: "g", Path: root, MaxDepth: 0}, "maxDepth"},
		{"路径不存在", ScanRule{Group: "g", Path: "/no/such/dir/xyz", MaxDepth: 3}, "不存在"},
		{"路径是文件", ScanRule{Group: "g", Path: ws.Join("f.txt"), MaxDepth: 3}, "非目录"},
		{"相对路径", ScanRule{Group: "g", Path: "relative/dir", MaxDepth: 3}, "不合法"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.SaveScanRule(c.rule)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("应报含 %q 的中文错误，实际: %v", c.want, err)
			}
			if rules := s.ScanRules(); len(rules) != 0 {
				t.Fatalf("坏数据不应落文件，实际规则: %v", rules)
			}
		})
	}
}

// TestDeleteScanRule 按 path 删除；不存在返回中文错误。
func TestDeleteScanRule(t *testing.T) {
	s, ws := newWriteService(t)
	root := ws.Mkdir("root")
	ws.MakeProjectDir(path.Join("root", "p1"))
	if err := s.SaveScanRule(ScanRule{Group: "g", Path: root, MaxDepth: 3}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	if err := s.DeleteScanRule(root); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if rules := s.ScanRules(); len(rules) != 0 {
		t.Fatalf("删除后规则应为空: %v", rules)
	}
	// 删除即重扫
	if projs := s.Projects(); len(projs) != 0 {
		t.Fatalf("删除后项目列表应即时重扫为空，实际 %d 个", len(projs))
	}

	if err := s.DeleteScanRule(root); err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("删除不存在的规则应报中文错误，实际: %v", err)
	}
}

// TestReorderScanRules 按路径重排；未知/重复路径报错且顺序不变，未列出的排在末尾。
func TestReorderScanRules(t *testing.T) {
	s, ws := newWriteService(t)
	r1, r2, r3 := ws.Mkdir("r1"), ws.Mkdir("r2"), ws.Mkdir("r3")
	for i, r := range []string{r1, r2, r3} {
		if err := s.SaveScanRule(ScanRule{Group: string(rune('a' + i)), Path: r, MaxDepth: 3}); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
	}

	// 只列 r3、r1：r2 未列出，应保持原相对顺序排在末尾
	if err := s.ReorderScanRules([]string{r3, r1}); err != nil {
		t.Fatalf("重排失败: %v", err)
	}
	rules := s.ScanRules()
	if len(rules) != 3 || rules[0].Path != r3 || rules[1].Path != r1 || rules[2].Path != r2 {
		t.Fatalf("重排后顺序不符: %v", rules)
	}

	// 未知路径：报错且不改原顺序
	if err := s.ReorderScanRules([]string{"/no/such"}); err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("未知路径应报中文错误，实际: %v", err)
	}
	// 重复路径：报错
	if err := s.ReorderScanRules([]string{r1, r1}); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("重复路径应报中文错误，实际: %v", err)
	}
}

// TestSaveCloneRule 新增与按 host+prefix 替换。
func TestSaveCloneRule(t *testing.T) {
	s, ws := newWriteService(t)

	if err := s.SaveCloneRule(CloneRule{RepoHost: "github.com", LocalPath: ws.Dir}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	// 同 host 不同 prefix 是新规则
	if err := s.SaveCloneRule(CloneRule{RepoHost: "github.com", RepoPrefix: "/heyuuu", LocalPath: ws.Dir}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	// 同 host+prefix 替换 localPath
	if err := s.SaveCloneRule(CloneRule{RepoHost: "github.com", RepoPrefix: "/heyuuu", LocalPath: ws.Mkdir("gh")}); err != nil {
		t.Fatalf("替换失败: %v", err)
	}

	rules := s.CloneRules()
	if len(rules) != 2 {
		t.Fatalf("应保留 2 条规则，实际 %d: %v", len(rules), rules)
	}
	for _, r := range rules {
		if r.RepoHost == "github.com" && r.RepoPrefix == "/heyuuu" && r.LocalPath != ws.Join("gh") {
			t.Fatalf("替换未生效: %+v", r)
		}
	}
}

// TestSaveCloneRule_Validation 坏数据返回中文错误、落不了文件。
func TestSaveCloneRule_Validation(t *testing.T) {
	s, ws := newWriteService(t)

	cases := []struct {
		name string
		rule CloneRule
		want string
	}{
		{"host 为空", CloneRule{LocalPath: ws.Dir}, "repoHost"},
		{"prefix 不以 / 开头", CloneRule{RepoHost: "github.com", RepoPrefix: "heyuuu", LocalPath: ws.Dir}, "repoPrefix"},
		{"localPath 相对路径", CloneRule{RepoHost: "github.com", LocalPath: "relative"}, "localPath"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.SaveCloneRule(c.rule)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("应报含 %q 的中文错误，实际: %v", c.want, err)
			}
			if rules := s.CloneRules(); len(rules) != 0 {
				t.Fatalf("坏数据不应落文件，实际规则: %v", rules)
			}
		})
	}
}

// TestDeleteCloneRule 按 host+prefix 删除；不存在返回中文错误。
func TestDeleteCloneRule(t *testing.T) {
	s, ws := newWriteService(t)
	if err := s.SaveCloneRule(CloneRule{RepoHost: "github.com", RepoPrefix: "/heyuuu", LocalPath: ws.Dir}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	key := CloneRuleKey{RepoHost: "github.com", RepoPrefix: "/heyuuu"}
	if err := s.DeleteCloneRule(key); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if rules := s.CloneRules(); len(rules) != 0 {
		t.Fatalf("删除后规则应为空: %v", rules)
	}
	if err := s.DeleteCloneRule(key); err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("删除不存在的规则应报中文错误，实际: %v", err)
	}
}

// TestReorderCloneRules 按键重排；语义约束同 scan 规则重排。
func TestReorderCloneRules(t *testing.T) {
	s, ws := newWriteService(t)
	save := func(host, prefix string) {
		t.Helper()
		if err := s.SaveCloneRule(CloneRule{RepoHost: host, RepoPrefix: prefix, LocalPath: ws.Dir}); err != nil {
			t.Fatalf("保存失败: %v", err)
		}
	}
	save("github.com", "/heyuuu")
	save("github.com", "")
	save("gitee.com", "")

	if err := s.ReorderCloneRules([]CloneRuleKey{
		{RepoHost: "gitee.com"},
		{RepoHost: "github.com", RepoPrefix: "/heyuuu"},
	}); err != nil {
		t.Fatalf("重排失败: %v", err)
	}
	rules := s.CloneRules()
	if len(rules) != 3 || rules[0].RepoHost != "gitee.com" || rules[1].RepoPrefix != "/heyuuu" || rules[2].RepoPrefix != "" {
		t.Fatalf("重排后顺序不符: %v", rules)
	}

	// 未知键：报中文错误
	bad := CloneRuleKey{RepoHost: "gitlab.com"}
	if err := s.ReorderCloneRules([]CloneRuleKey{bad}); err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("未知键应报中文错误，实际: %v", err)
	}
}
