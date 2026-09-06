package projcache

import (
	"fmt"
	"os"
	"os/exec"
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

// TestLoad_VersionMismatch 版本不符的 git.json 当文件不存在丢弃（不备份、不 lenient 混入）。
// 老数据按新结构反序列化会得到缺字段全零值的部分数据，比空缓存更糟。
func TestLoad_VersionMismatch(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.Mkdir("cache")
	old := `{"version":` + fmt.Sprintf("%d", cacheVersion-1) + `,"entries":{"/p/a":{"repoUrl":"x","currentBranch":"main"}}}`
	ws.WriteFile(filepath.Join("cache", cacheFileName), []byte(old))

	c, err := Load(dir)
	if err != nil {
		t.Fatalf("版本不符 Load 不应报错: %v", err)
	}
	if _, ok := c.Get("/p/a"); ok {
		t.Fatal("旧版本数据应被整体丢弃")
	}
	// 区别于损坏文件：版本不符不备份（文件是合法的，只是过时）
	matches, _ := filepath.Glob(filepath.Join(dir, cacheFileName+".corrupt-*"))
	if len(matches) != 0 {
		t.Fatal("版本不符不应走损坏备份路径")
	}
}

// TestCache_Get 内存态 Get 命中/未命中。
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

// TestCollectEntry_Remotes 全部 remote 进快照（多 remote 项目的 forge 匹配依赖此数据）。
func TestCollectEntry_Remotes(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "main", RemoteUrl: "git@github.com:heyuuu/repo.git"})
	// 补一个非 origin remote（指向另一 host）
	if out, err := exec.Command("git", "-C", repo, "remote", "add", "gitee", "git@gitee.com:heyuuu/repo.git").CombinedOutput(); err != nil {
		t.Fatalf("添加 gitee remote 失败: %v %s", err, out)
	}

	e, err := collectEntry(repo)
	if err != nil || e == nil {
		t.Fatalf("collectEntry 失败: %v", err)
	}
	if e.RepoUrl != "git@github.com:heyuuu/repo.git" {
		t.Fatalf("RepoUrl = %q（origin 口径不变）", e.RepoUrl)
	}
	hosts := make([]string, 0, len(e.Remotes))
	for _, r := range e.Remotes {
		hosts = append(hosts, r.Name+"="+r.Url)
	}
	if len(e.Remotes) != 2 || e.Remotes[0].Name != "gitee" || e.Remotes[1].Name != "origin" {
		t.Fatalf("应含 gitee 与 origin 两条 remote（按名排序）: %v", hosts)
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

// TestCollectEntry_Worktrees 主项目采集时附带枚举 linked worktree（1032 归并）。
func TestCollectEntry_Worktrees(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "main"})
	ws.MakeWorktree(repo, "wt-hot", "hotfix")
	ws.MakeWorktree(repo, "wt-feat", "feat")

	e, err := collectEntry(repo)
	if err != nil {
		t.Fatalf("collectEntry 不应返回 error: %v", err)
	}
	if len(e.Worktrees) != 2 {
		t.Fatalf("应附带 2 个 worktree, got %d (%+v)", len(e.Worktrees), e.Worktrees)
	}
	// 分支名 + 路径落在快照里（主目录自身不含在内）；git 输出路径经符号链接规范化，比较前同样求值
	canonicalWt, err := filepath.EvalSymlinks(ws.Join("wt-hot"))
	if err != nil {
		t.Fatalf("解析真实路径失败: %v", err)
	}
	byBranch := map[string]WorktreeInfo{}
	for _, wt := range e.Worktrees {
		byBranch[wt.Branch] = wt
	}
	if wt, ok := byBranch["hotfix"]; !ok || wt.Path != canonicalWt {
		t.Fatalf("hotfix worktree 字段不符: %+v ok=%v", wt, ok)
	}
	if _, ok := byBranch["feat"]; !ok {
		t.Fatalf("缺少 feat worktree: %+v", e.Worktrees)
	}
}

// TestCollectEntry_WorktreesDegraded 非仓库目录（WorktreeList 失败）降级为空列表而非报错。
func TestCollectEntry_WorktreesDegraded(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.Mkdir("empty")

	e, err := collectEntry(dir)
	if err != nil {
		t.Fatalf("非仓库 collectEntry 不应返回 error: %v", err)
	}
	if e.Worktrees == nil || len(e.Worktrees) != 0 {
		t.Fatalf("非仓库 Worktrees 应为空列表, got %+v", e.Worktrees)
	}
}

// TestCollectEntry_WorktreePruned 目录已删但 git 元数据未 prune 的 worktree 不进快照（幽灵目标）。
func TestCollectEntry_WorktreePruned(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "main"})
	ws.MakeWorktree(repo, "wt-hot", "hotfix")
	ws.MakeWorktree(repo, "wt-gone", "gone")

	// 删除 wt-gone 目录但不跑 git worktree prune——git 仍会列出它
	if err := os.RemoveAll(ws.Join("wt-gone")); err != nil {
		t.Fatalf("删除 worktree 目录失败: %v", err)
	}

	e, err := collectEntry(repo)
	if err != nil {
		t.Fatalf("collectEntry 不应返回 error: %v", err)
	}
	if len(e.Worktrees) != 1 {
		t.Fatalf("失联 worktree 应被过滤，只留 1 个, got %+v", e.Worktrees)
	}
	if e.Worktrees[0].Branch != "hotfix" {
		t.Fatalf("留下的应是 hotfix: %+v", e.Worktrees[0])
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

// RefreshOne 定向刷新单项目：新建 worktree 后即时进快照，不影响其他条目。
func TestRefreshOne(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")
	other := ws.MakeGitRepo("other")
	c, _ := Load(ws.Mkdir("cache"))

	if err := c.RefreshOne(repo); err != nil {
		t.Fatalf("RefreshOne 报错: %v", err)
	}
	e, ok := c.Get(repo)
	if !ok || e.CurrentBranch == "" {
		t.Fatalf("刷新后应有该项目的 entry: %+v", e)
	}
	if _, ok := c.Get(other); ok {
		t.Fatalf("定向刷新不应影响其他项目")
	}

	ws.MakeWorktree(repo, "wt-hot", "hotfix")
	if err := c.RefreshOne(repo); err != nil {
		t.Fatalf("二次刷新报错: %v", err)
	}
	e, _ = c.Get(repo)
	if len(e.Worktrees) != 1 || e.Worktrees[0].Branch != "hotfix" {
		t.Fatalf("worktree 应已进快照: %+v", e.Worktrees)
	}

	// 非 git 目录与整表 Refresh 语义一致：零值 entry 成功写入，不报错
	if err := c.RefreshOne(ws.Join("plain")); err != nil {
		t.Fatalf("非仓库按约定应写入零值 entry: %v", err)
	}
	if e, ok := c.Get(repo); !ok || e.CurrentBranch == "" {
		t.Fatalf("其他条目不受影响: %+v", e)
	}
}
