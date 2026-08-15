package gitcache

import (
	"os"
	"path/filepath"
	"testing"

	"cube/internal/testfixture"
)

// TestLoad_EmptyDir 目录不存在 → 创建并返回空缓存。
func TestLoad_EmptyDir(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.Join("not-exist-yet")

	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 出错: %v", err)
	}
	// 目录应被创建
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("Load 应创建目录，实际未创建: %v", err)
	}
	// 空缓存：Get 任意路径未命中
	if _, ok := c.Get("/any"); ok {
		t.Fatalf("空缓存 Get 应未命中")
	}
}

// TestLoad_FileMissing git.json 不存在 → 空缓存（冷启动降级）。
func TestLoad_FileMissing(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.Mkdir("cache")

	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 出错: %v", err)
	}
	if _, ok := c.Get("/any"); ok {
		t.Fatalf("无 git.json 时应返回空缓存")
	}
}

// TestSaveLoad_RoundTrip Save 后 Load 能读回 entries。
func TestSaveLoad_RoundTrip(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.Mkdir("cache")

	// 写入
	c1, _ := Load(dir)
	c1.entries["/proj/a"] = &Entry{RepoUrl: "git@x:a", CurrentBranch: "main", Ahead: 3}
	if err := c1.Save(); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// 重新 Load
	c2, _ := Load(dir)
	got, ok := c2.Get("/proj/a")
	if !ok {
		t.Fatalf("Save 后 Load 应命中")
	}
	if got.RepoUrl != "git@x:a" || got.CurrentBranch != "main" || got.Ahead != 3 {
		t.Fatalf("读回数据异常: %+v", got)
	}
}

// TestLoad_CorruptFile 损坏的 git.json 应降级为空缓存，且原文件被备份。
func TestLoad_CorruptFile(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.Mkdir("cache")
	ws.WriteFile(filepath.Join("cache", cacheFileName), []byte("{not valid json"))

	c, err := Load(dir)
	if err != nil {
		t.Fatalf("损坏文件 Load 不应报错（降级）: %v", err)
	}
	if _, ok := c.Get("/any"); ok {
		t.Fatalf("损坏文件应返回空缓存")
	}
	// 损坏文件应被备份（git.json.corrupt-*）
	matches, _ := filepath.Glob(filepath.Join(dir, cacheFileName+".corrupt-*"))
	if len(matches) == 0 {
		t.Fatalf("损坏文件应被备份为 .corrupt-*")
	}
}

// TestCache_GetSetMutate 内存态 Get 命中/未命中。
func TestCache_Get(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	c, _ := Load(ws.Mkdir("cache"))

	c.entries["/p"] = &Entry{Ahead: 1}
	if e, ok := c.Get("/p"); !ok || e.Ahead != 1 {
		t.Fatalf("Get 命中异常: %+v ok=%v", e, ok)
	}
	if _, ok := c.Get("/missing"); ok {
		t.Fatalf("未命中 Get 应返回 ok=false")
	}
}

// TestCollectEntry_RealRepo 真实仓库采集能拿到 branch。
func TestCollectEntry_RealRepo(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "main"})

	e, err := collectEntry(repo)
	if err != nil {
		t.Fatalf("collectEntry 不应返回 error: %v", err)
	}
	if e == nil {
		t.Fatalf("collectEntry 不应返回 nil")
	}
	if e.CurrentBranch != "main" {
		t.Fatalf("CurrentBranch = %q，期望 main", e.CurrentBranch)
	}
}

// TestCollectEntry_NonRepo 非仓库目录：git 读函数返回零值，collectEntry 拼出零值 entry。
func TestCollectEntry_NonRepo(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.Mkdir("empty")

	e, err := collectEntry(dir)
	if err != nil {
		t.Fatalf("非仓库 collectEntry 不应返回 error: %v", err)
	}
	if e == nil {
		t.Fatalf("collectEntry 应返回非 nil entry")
	}
	// 字段应为零值
	if e.CurrentBranch != "" {
		t.Fatalf("非仓库 CurrentBranch 应为空，实际 %q", e.CurrentBranch)
	}
}

// TestRefresh_RealRepo Refresh 真实仓库后能读到采集结果。
func TestRefresh_RealRepo(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	cacheDir := ws.Mkdir("cache")
	repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "develop"})

	c, _ := Load(cacheDir)
	if err := c.Refresh([]string{repo}); err != nil {
		t.Fatalf("Refresh 失败: %v", err)
	}
	e, ok := c.Get(repo)
	if !ok {
		t.Fatalf("Refresh 后 Get 应命中")
	}
	if e.CurrentBranch != "develop" {
		t.Fatalf("Refresh 采集 CurrentBranch = %q，期望 develop", e.CurrentBranch)
	}

	// git.json 应已落盘
	if _, err := os.Stat(filepath.Join(cacheDir, cacheFileName)); err != nil {
		t.Fatalf("Refresh 后 git.json 应存在: %v", err)
	}
}

// TestRefresh_EmptyPaths 空路径列表也成功（仅刷 UpdatedAt）。
func TestRefresh_EmptyPaths(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	c, _ := Load(ws.Mkdir("cache"))
	if err := c.Refresh(nil); err != nil {
		t.Fatalf("空 Refresh 不应报错: %v", err)
	}
}
