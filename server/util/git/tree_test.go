package git

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cube/internal/testfixture"
)

// wantBlobSha 测试内独立计算 git blob sha（sha1("blob <len>\0" + 内容)），
// 与被测实现对账。
func wantBlobSha(content string) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// TestFileShasAtRef 递归平铺的路径与 blob sha 均正确（含中文名与子目录）。
func TestFileShasAtRef(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")

	files := map[string]string{
		"a.txt":            "hello\n",
		"nested/dir/b.txt": "中文内容\n",
		"中文名 文件.txt":       "quoted path\n",
	}
	for name, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	directGit(t, dir, "add", "-A")
	directGit(t, dir, "commit", "-m", "c1")

	m, err := FileShasAtRef(dir, "HEAD")
	if err != nil {
		t.Fatalf("FileShasAtRef 出错: %v", err)
	}
	if len(m) != len(files) {
		t.Fatalf("文件数 = %d，期望 %d（全部: %v）", len(m), len(files), m)
	}
	for name, content := range files {
		sha, ok := m[name]
		if !ok {
			t.Errorf("缺少文件 %s（全部: %v）", name, m)
			continue
		}
		if want := wantBlobSha(content); sha != want {
			t.Errorf("%s 的 blob sha = %s，期望 %s", name, sha, want)
		}
	}

	if _, err := FileShasAtRef(dir, "no-such-ref"); err == nil {
		t.Error("ref 不存在时应返回错误")
	}
}

// TestListFilesUnder 子目录视角的全量列举：结果限定在子目录内、绝对路径、
// 仓库根 .gitignore 的忽略链对子目录生效、目录自身被忽略时返回 nil 降级。
func TestListFilesUnder(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")
	ws.WriteFile("repo/.gitignore", []byte("node_modules/\n"))
	ws.WriteFile("repo/docs/a.md", []byte("a"))
	ws.WriteFile("repo/docs/node_modules/b.md", []byte("b"))
	ws.WriteFile("repo/docs/sub/c.md", []byte("c"))
	ws.WriteFile("repo/other/d.md", []byte("d")) // 子目录外，不应出现

	files, err := ListFilesUnder(filepath.Join(repo, "docs"))
	if err != nil {
		t.Fatalf("ListFilesUnder 出错: %v", err)
	}
	got := map[string]bool{}
	for _, f := range files {
		if !filepath.IsAbs(f) {
			t.Errorf("应返回绝对路径: %s", f)
		}
		if strings.Contains(f, "node_modules") {
			t.Errorf("被忽略的 node_modules 不应出现: %s", f)
		}
		if strings.HasSuffix(f, "other/d.md") {
			t.Errorf("子目录外的文件不应出现: %s", f)
		}
		got[f] = true
	}
	if len(got) != 2 {
		t.Errorf("应收录 docs 下 2 个文件, got %v", files)
	}

	// 目录自身被根 .gitignore 忽略 → (nil, nil) 交上层降级
	ws.WriteFile("repo/.gitignore", []byte("node_modules/\ndist/\n"))
	ws.Mkdir("repo/dist")
	files, err = ListFilesUnder(filepath.Join(repo, "dist"))
	if err != nil || files != nil {
		t.Errorf("被忽略目录应返回 (nil, nil), got (%v, %v)", files, err)
	}

	// 仓库外目录 → (nil, nil)
	files, err = ListFilesUnder(ws.Mkdir("plain"))
	if err != nil || files != nil {
		t.Errorf("仓库外目录应返回 (nil, nil), got (%v, %v)", files, err)
	}
}
