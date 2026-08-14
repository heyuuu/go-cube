package project

import (
	"os"
	"path"
	"testing"

	"cube/internal/testfixture"
)

// TestSearchByPath_SearchByPath 验证 SearchByPath 的入参契约与路径搜索行为。
// path 只接受绝对路径或 ~ 前缀（StaticAbsPath 归一化）；相对路径按未找到处理
// ——cwd 解析是出口层（cmd）职责，见 cmd 包的 searchProjects 测试。
func TestSearchByPath_SearchByPath(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	root := ws.Mkdir("root")
	repo := ws.MakeProjectDir(path.Join("root", "proj"))

	s := newServiceAt(t, root, "g1", 5)

	// 绝对路径：命中该目录下的项目
	projs := s.SearchByPath(repo, false)
	if len(projs) != 1 || projs[0].Path() != repo {
		t.Fatalf("SearchByPath(绝对路径) 应命中 %s，实际 %v", repo, projs)
	}

	// 绝对路径 + up：在项目子目录向上标定
	sub := ws.Mkdir(path.Join("root", "proj", "sub"))
	projs = s.SearchByPath(sub, true)
	if len(projs) != 1 || projs[0].Path() != repo {
		t.Fatalf("SearchByPath(子目录, up=true) 应向上命中 %s，实际 %v", repo, projs)
	}
	if projs = s.SearchByPath(sub, false); len(projs) != 0 {
		t.Fatalf("SearchByPath(子目录, up=false) 不应命中，实际 %v", projs)
	}

	// 相对路径：拒绝（出口层职责，到这里即调用方传错）
	if projs = s.SearchByPath(".", true); len(projs) != 0 {
		t.Fatalf("SearchByPath(\".\") 应按未找到处理，实际 %v", projs)
	}
}

// TestSearchByPath_HomePrefix 验证 ~ 前缀归一化（兼容 web 直接传 ~/xxx 的场景）。
func TestSearchByPath_HomePrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// 在 home 下建 scan root + 项目，使项目路径形如 home/root/proj
	ws := testfixture.NewWorkspace(t)
	repoDir := home + "/root/proj"
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	testfixture.BuildGitRepo(ws.TB, repoDir, testfixture.GitRepoSpec{})

	cfg := newServiceAt(t, home+"/root", "g1", 5)
	projs := cfg.SearchByPath("~/root", false)
	if len(projs) != 1 || projs[0].Path() != repoDir {
		t.Fatalf("SearchByPath(~/root) 应命中 %s，实际 %v", repoDir, projs)
	}
}
