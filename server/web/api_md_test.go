package web

import (
	"strings"
	"testing"
)

func TestMdContent(t *testing.T) {
	env := newTestEnv(t)
	md := env.ws.Join("notes/readme.md")
	env.ws.WriteFile("notes/readme.md", []byte("# 标题\n\n正文"))

	var got struct {
		Content string `json:"content"`
	}
	decodeData(t, getJSON(t, env.url("/api/md/content?path="+md)), &got)
	if got.Content != "# 标题\n\n正文" {
		t.Errorf("content 应返回原文, got %q", got.Content)
	}

	// 相对路径拒绝
	bad := getJSON(t, env.url("/api/md/content?path=relative/x.md"))
	if bad.Ok {
		t.Errorf("相对路径应报错")
	}
}

func TestMdList(t *testing.T) {
	env := newTestEnv(t)
	dir := env.ws.Dir
	env.ws.WriteFile("notes/readme.md", []byte("a"))
	env.ws.WriteFile("notes/sub/deep.md", []byte("b"))
	env.ws.WriteFile("notes/plain.txt", []byte("c"))    // 全量返回，不限扩展名（.md 过滤归前端）
	env.ws.WriteFile("notes/.hidden/x.md", []byte("d")) // 降级遍历时隐藏目录跳过

	var got struct {
		Dir   bool     `json:"dir"`
		Files []string `json:"files"`
	}
	// workspace 落在 cube 仓库被 gitignore 的 runtime/ 下，走「目录自身被忽略」降级分支
	decodeData(t, getJSON(t, env.url("/api/md/list?path="+dir+"/notes")), &got)
	if !got.Dir {
		t.Fatalf("目录路径 dir 应为 true")
	}
	// .hidden/x.md 不应出现
	for _, f := range got.Files {
		if strings.Contains(f, ".hidden") {
			t.Errorf("隐藏目录不应被收录: %s", f)
		}
	}
	if len(got.Files) != 3 {
		t.Errorf("应收录 3 个文件（含非 md）, got %v", got.Files)
	}

	// 单文件模式
	var gotFile struct {
		Dir   bool     `json:"dir"`
		Files []string `json:"files"`
	}
	decodeData(t, getJSON(t, env.url("/api/md/list?path="+dir+"/notes/readme.md")), &gotFile)
	if gotFile.Dir || len(gotFile.Files) != 1 {
		t.Errorf("文件路径应为单文件模式, got dir=%v files=%v", gotFile.Dir, gotFile.Files)
	}
}

// git 仓库内：.gitignore 过滤链生效——打开仓库子目录时，仓库根的 .gitignore
// 同样过滤（node_modules 等被忽略目录不进入结果）
func TestMdListGitIgnore(t *testing.T) {
	env := newTestEnv(t)
	repo := env.ws.MakeGitRepo("docrepo")
	env.ws.WriteFile("docrepo/.gitignore", []byte("node_modules/\n"))
	env.ws.WriteFile("docrepo/docs/readme.md", []byte("a"))
	env.ws.WriteFile("docrepo/docs/node_modules/pkg/x.md", []byte("b")) // 根 .gitignore 忽略

	var got struct {
		Dir   bool     `json:"dir"`
		Files []string `json:"files"`
	}
	// 打开的是仓库子目录 docs，不是仓库根
	decodeData(t, getJSON(t, env.url("/api/md/list?path="+repo+"/docs")), &got)
	if !got.Dir {
		t.Fatalf("目录路径 dir 应为 true")
	}
	for _, f := range got.Files {
		if strings.Contains(f, "node_modules") {
			t.Errorf("被 .gitignore 忽略的文件不应被收录: %s", f)
		}
	}
	if len(got.Files) != 1 || !strings.HasSuffix(got.Files[0], "docs/readme.md") {
		t.Errorf("应只收录 readme.md, got %v", got.Files)
	}
}
