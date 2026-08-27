package project

import (
	"testing"
	"time"

	"cube/internal/testfixture"
)

func TestSortByRecentUsage(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	rule := ScanRule{Group: "g", Path: ws.Dir, MaxDepth: 1}
	projects := []*Project{
		newProject(rule, ws.Join("a"), nil),
		newProject(rule, ws.Join("b"), nil),
		newProject(rule, ws.Join("c"), nil),
	}

	// b 最近、c 次之：两者置顶（倒序），a 保持原序垫底
	base := time.Now()
	latest := map[string]time.Time{
		ws.Join("b"): base.Add(-time.Hour),
		ws.Join("c"): base.Add(-2 * time.Hour),
	}
	SortByRecentUsage(projects, latest, 10)

	got := make([]string, 3)
	for i, p := range projects {
		got[i] = p.Name()
	}
	want := []string{"g:b", "g:c", "g:a"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("排序结果异常: got %v want %v", got, want)
		}
	}
}

func TestSortByRecentUsage_Limit(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	rule := ScanRule{Group: "g", Path: ws.Dir, MaxDepth: 1}
	projects := []*Project{
		newProject(rule, ws.Join("a"), nil),
		newProject(rule, ws.Join("b"), nil),
		newProject(rule, ws.Join("c"), nil),
	}

	// limit=1：只有最新的 b 置顶，c 回落原序
	base := time.Now()
	latest := map[string]time.Time{
		ws.Join("b"): base.Add(-time.Hour),
		ws.Join("c"): base.Add(-2 * time.Hour),
	}
	SortByRecentUsage(projects, latest, 1)

	got := make([]string, 3)
	for i, p := range projects {
		got[i] = p.Name()
	}
	want := []string{"g:b", "g:a", "g:c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("limit 截断后排序异常: got %v want %v", got, want)
		}
	}
}

func TestSortByRecentUsage_Empty(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	rule := ScanRule{Group: "g", Path: ws.Dir, MaxDepth: 1}
	projects := []*Project{newProject(rule, ws.Join("a"), nil)}

	// 无使用记录：原序不动
	SortByRecentUsage(projects, map[string]time.Time{}, 10)
	if projects[0].Name() != "g:a" {
		t.Fatalf("无记录应保持原序, got %v", projects[0].Name())
	}
}
