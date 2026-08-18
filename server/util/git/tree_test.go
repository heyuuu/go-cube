package git

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
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
