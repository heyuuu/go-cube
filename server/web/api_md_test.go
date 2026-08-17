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
	env.ws.WriteFile("notes/plain.txt", []byte("c"))    // 非 md 不收录
	env.ws.WriteFile("notes/.hidden/x.md", []byte("d")) // 隐藏目录跳过

	var got struct {
		Dir   bool     `json:"dir"`
		Files []string `json:"files"`
	}
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
	if len(got.Files) != 2 {
		t.Errorf("应收录 2 个 md 文件, got %v", got.Files)
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
