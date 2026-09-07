package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiffFiles 覆盖 added / modified / deleted / renamed 四种状态与排序。
func TestDiffFiles(t *testing.T) {
	ws := newTestWorkspace(t)
	dir := ws.MakeGitRepo("repo")

	writeAbsFile := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("写文件失败: %v", err)
		}
	}
	writeAbsFile("a.txt", "v1")
	writeAbsFile("del.txt", "bye")
	writeAbsFile("old-name.txt", "same content")
	directGit(t, dir, "add", "-A")
	directGit(t, dir, "commit", "-m", "c1")

	writeAbsFile("a.txt", "v2") // modified
	os.Remove(filepath.Join(dir, "del.txt"))
	directGit(t, dir, "mv", "old-name.txt", "new-name.txt") // renamed（内容不变 → R100）
	writeAbsFile("b.txt", "new")                            // added
	directGit(t, dir, "add", "-A")
	directGit(t, dir, "commit", "-m", "c2")

	files, err := DiffFiles(dir, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("DiffFiles 出错: %v", err)
	}
	byPath := make(map[string]DiffFile, len(files))
	for _, f := range files {
		byPath[f.Path] = f
	}
	if f, ok := byPath["a.txt"]; !ok || f.Code != "M" {
		t.Errorf("a.txt 应为 M，实际 (%q, 存在=%v)", f.Code, ok)
	}
	if f, ok := byPath["b.txt"]; !ok || f.Code != "A" {
		t.Errorf("b.txt 应为 A，实际 (%q, 存在=%v)", f.Code, ok)
	}
	if f, ok := byPath["del.txt"]; !ok || f.Code != "D" {
		t.Errorf("del.txt 应为 D，实际 (%q, 存在=%v)", f.Code, ok)
	}
	f, ok := byPath["new-name.txt"]
	if !ok || !strings.HasPrefix(f.Code, "R") {
		t.Errorf("new-name.txt 应为 R*，实际 (%q, 存在=%v)", f.Code, ok)
	} else if f.OldPath != "old-name.txt" {
		t.Errorf("rename 旧路径 = %q，期望 old-name.txt", f.OldPath)
	}
	if len(files) != 4 {
		t.Errorf("变更数 = %d，期望 4（全部: %v）", len(files), files)
	}
	for i := 1; i < len(files); i++ {
		if files[i-1].Path > files[i].Path {
			t.Errorf("DiffFiles 未按路径排序: %v", files)
			break
		}
	}
}

// TestDiffFiles_RefMissing ref 不存在时报错（不静默返回空）。
func TestDiffFiles_RefMissing(t *testing.T) {
	ws := newTestWorkspace(t)
	dir := ws.MakeGitRepo("repo")
	if _, err := DiffFiles(dir, "no-such-ref", "HEAD"); err == nil {
		t.Error("ref 不存在时应返回错误")
	}
}

// TestDiffNoIndex 有差异时退出码为 1 属正常语义，输出必须完整返回。
func TestDiffNoIndex(t *testing.T) {
	ws := newTestWorkspace(t)
	a := filepath.Join(ws.Dir, "a.txt")
	b := filepath.Join(ws.Dir, "b.txt")
	if err := os.WriteFile(a, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("line1\nlineX\n"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := DiffNoIndex(a, b)
	if err != nil {
		t.Fatalf("有差异是正常语义不应报错: %v", err)
	}
	if !strings.Contains(out, "@@") || !strings.Contains(out, "+lineX") || !strings.Contains(out, "-line2") {
		t.Errorf("unified diff 输出不完整:\n%s", out)
	}

	same := filepath.Join(ws.Dir, "same.txt")
	if err := os.WriteFile(same, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err = DiffNoIndex(a, same)
	if err != nil || out != "" {
		t.Errorf("相同文件应返回空输出，实际 (out=%q, err=%v)", out, err)
	}

	if _, err := DiffNoIndex(a, filepath.Join(ws.Dir, "missing.txt")); err == nil {
		t.Error("路径不存在时应返回错误")
	}
}

// TestNumstat 覆盖 modified 增删计数、rename 新路径入 map、二进制标记、工作区模式。
func TestNumstat(t *testing.T) {
	ws := newTestWorkspace(t)
	dir := ws.MakeGitRepo("repo")

	writeAbsFile := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("写文件失败: %v", err)
		}
	}
	writeAbsFile("a.txt", "l1\nl2\n")
	writeAbsFile("old.txt", "x\n")
	directGit(t, dir, "add", "-A")
	directGit(t, dir, "commit", "-m", "c1")

	writeAbsFile("a.txt", "l1\nl2\nl3\nl4\n") // +2 -0
	directGit(t, dir, "mv", "old.txt", "new.txt")
	if err := os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{'a', 0, 'b'}, 0644); err != nil {
		t.Fatal(err)
	}
	directGit(t, dir, "add", "-A")
	directGit(t, dir, "commit", "-m", "c2")

	stats, err := Numstat(dir, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("Numstat 出错: %v", err)
	}
	if got := stats["a.txt"]; got.Adds != 2 || got.Dels != 0 {
		t.Errorf("a.txt 期望 +2/-0，实际 +%d/-%d", got.Adds, got.Dels)
	}
	if _, ok := stats["new.txt"]; !ok {
		t.Errorf("rename 应以新路径入 map: %v", stats)
	}
	if got := stats["bin.dat"]; !got.Binary {
		t.Errorf("二进制文件应标记 Binary: %+v", got)
	}

	// 工作区模式（right 为空）：改 a.txt 不提交
	writeAbsFile("a.txt", "l1\n")
	wt, err := Numstat(dir, "HEAD", "")
	if err != nil {
		t.Fatalf("Numstat 工作区模式出错: %v", err)
	}
	if got := wt["a.txt"]; got.Adds != 0 || got.Dels != 3 {
		t.Errorf("a.txt 工作区期望 +0/-3，实际 +%d/-%d", got.Adds, got.Dels)
	}
}
