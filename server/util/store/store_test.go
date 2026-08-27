package store

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"cube/internal/testfixture"
)

func TestWriteFileAtomic(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	path := ws.Join("sub", "data.json")

	if err := WriteFileAtomic(path, []byte("hello"), 0644); err != nil {
		t.Fatalf("首次原子写失败: %v", err)
	}
	// 深层目录不存在时自动递归创建
	deep := filepath.Join(ws.Dir, "a", "b", "c", "d.json")
	if err := WriteFileAtomic(deep, []byte("x"), 0644); err != nil {
		t.Fatalf("深层目录自动创建失败: %v", err)
	}
	if data, err := os.ReadFile(deep); err != nil || string(data) != "x" {
		t.Fatalf("深层目录写入内容不对: %q err=%v", data, err)
	}

	if err := WriteFileAtomic(path, []byte("world"), 0600); err != nil {
		t.Fatalf("覆盖原子写失败: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回失败: %v", err)
	}
	if string(data) != "world" {
		t.Fatalf("覆盖后内容不对: %q", data)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0600 {
		t.Fatalf("权限未生效: %v", info.Mode().Perm())
	}

	// 目录里不应残留 tmp 文件
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || len(e.Name()) > 0 && e.Name()[0] == '.' {
			t.Fatalf("残留临时文件: %s", e.Name())
		}
	}
}

func TestSaveAndLoadJson(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	path := ws.Join("cfg.json")

	type conf struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}

	// 缺失文件返回哨兵错误
	if _, err := LoadJson[conf](path); !errors.Is(err, ErrFileMissing) {
		t.Fatalf("缺失文件应返回 ErrFileMissing, got %v", err)
	}

	want := conf{Name: "cube", N: 3}
	if err := SaveJson(path, want); err != nil {
		t.Fatalf("SaveJson 失败: %v", err)
	}
	got, err := LoadJson[conf](path)
	if err != nil {
		t.Fatalf("LoadJson 失败: %v", err)
	}
	if got != want {
		t.Fatalf("roundtrip 不一致: %+v vs %+v", got, want)
	}

	// 坏 JSON 返回解析错误（非哨兵）
	if err := os.WriteFile(path, []byte("{oops"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadJson[conf](path); err == nil || errors.Is(err, ErrFileMissing) {
		t.Fatalf("坏 JSON 应返回解析错误, got %v", err)
	}
}

func TestAppendAndLoadJsonl(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	path := ws.Join("usage.jsonl")

	type record struct {
		Path string `json:"path"`
		Seq  int    `json:"seq"`
	}

	if _, err := LoadJsonl[record](path); !errors.Is(err, ErrFileMissing) {
		t.Fatalf("缺失文件应返回 ErrFileMissing, got %v", err)
	}

	want := []record{{Path: "a", Seq: 1}, {Path: "b", Seq: 2}, {Path: "a", Seq: 3}}
	for _, r := range want {
		if err := AppendJsonl(path, r); err != nil {
			t.Fatalf("AppendJsonl 失败: %v", err)
		}
	}

	// 追加一条含换行敏感字符的记录，确认序列化后仍是单行
	if err := AppendJsonl(path, record{Path: "x\ny", Seq: 4}); err != nil {
		t.Fatal(err)
	}

	// 注入坏行：中间坏行被跳过，好行保留
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("\n{broken\n") // 一个空行 + 一个坏行
	f.Close()

	got, err := LoadJsonl[record](path)
	if err != nil {
		t.Fatalf("LoadJsonl 失败: %v", err)
	}
	expectLen := len(want) + 1
	if len(got) != expectLen {
		t.Fatalf("坏行跳过后条数不对: want %d got %d (%+v)", expectLen, len(got), got)
	}
	for i, r := range want {
		if got[i] != r {
			t.Fatalf("第 %d 条不一致: %+v vs %+v", i, got[i], r)
		}
	}
	if got[len(got)-1].Path != "x\ny" {
		t.Fatalf("含换行记录解析错误: %+v", got[len(got)-1])
	}
}

func TestIterJsonl(t *testing.T) {
	ws := testfixture.NewWorkspace(t)

	type record struct {
		Seq int `json:"seq"`
	}

	// 缺失文件：静默结束，不 panic
	n := 0
	for range IterJsonl[record](ws.Join("none.jsonl")) {
		n++
	}
	if n != 0 {
		t.Fatalf("缺失文件应产出 0 条, got %d", n)
	}

	// 坏行占行号、空行占行号
	path := ws.Join("it.jsonl")
	os.WriteFile(path, []byte(`{"seq":1}`+"\n\n"+`{bad`+"\n"+`{"seq":2}`+"\n"), 0644)

	var seqs []int
	var lineNos []int
	for lineNo, r := range IterJsonl[record](path) {
		seqs = append(seqs, r.Seq)
		lineNos = append(lineNos, lineNo)
	}
	if len(seqs) != 2 || seqs[0] != 1 || seqs[1] != 2 {
		t.Fatalf("正向迭代记录不对: %+v", seqs)
	}
	if lineNos[0] != 1 || lineNos[1] != 4 {
		t.Fatalf("物理行号不对: %+v", lineNos)
	}

	// 提前停止：只取第一条
	var firstSeq int
	for _, r := range IterJsonl[record](path) {
		firstSeq = r.Seq
		break
	}
	if firstSeq != 1 {
		t.Fatalf("提前停止取首条失败: %d", firstSeq)
	}
}

func TestIterJsonlReverse(t *testing.T) {
	ws := testfixture.NewWorkspace(t)

	type record struct {
		Seq int `json:"seq"`
	}

	// 缺失文件：静默结束
	n := 0
	for range IterJsonlReverse[record](ws.Join("none.jsonl")) {
		n++
	}
	if n != 0 {
		t.Fatalf("缺失文件应产出 0 条, got %d", n)
	}

	// 多行 + 坏行 + 空行；文件以 '\n' 结尾
	buildLines := func(trailingNL bool) (string, []byte) {
		lines := `{"seq":1}` + "\n" +
			"\n" +
			`{bad` + "\n" +
			`{"seq":2}` + "\n" +
			`{"seq":3}` + "\n"
		if !trailingNL {
			lines = strings.TrimSuffix(lines, "\n")
		}
		return lines, []byte(lines)
	}

	for _, trailingNL := range []bool{true, false} {
		_, data := buildLines(trailingNL)
		path := ws.Join("rev.jsonl")
		os.WriteFile(path, data, 0644)

		var seqs []int
		var lineNos []int
		for lineNo, r := range IterJsonlReverse[record](path) {
			seqs = append(seqs, r.Seq)
			lineNos = append(lineNos, lineNo)
		}
		if len(seqs) != 3 || seqs[0] != 3 || seqs[1] != 2 || seqs[2] != 1 {
			t.Fatalf("逆向迭代记录不对 (trailingNL=%v): %+v", trailingNL, seqs)
		}
		if lineNos[0] != 5 || lineNos[1] != 4 || lineNos[2] != 1 {
			t.Fatalf("物理行号不对 (trailingNL=%v): %+v", trailingNL, lineNos)
		}

		// 提前停止：只取最后一条
		var lastSeq int
		for _, r := range IterJsonlReverse[record](path) {
			lastSeq = r.Seq
			break
		}
		if lastSeq != 3 {
			t.Fatalf("提前停止取末条失败: %d", lastSeq)
		}
	}

	// 超过分块大小（>64KB）：验证跨块拼接
	path := ws.Join("big.jsonl")
	var sb strings.Builder
	want := 2000
	for i := range want {
		sb.WriteString(`{"seq":` + strconv.Itoa(i) + `,"pad":"` + strings.Repeat("x", 64) + `"}` + "\n")
	}
	os.WriteFile(path, []byte(sb.String()), 0644)

	var got []int
	for _, r := range IterJsonlReverse[record](path) {
		got = append(got, r.Seq)
	}
	if len(got) != want {
		t.Fatalf("跨块总条数不对: want %d got %d", want, len(got))
	}
	for i := range want {
		if got[i] != want-1-i {
			t.Fatalf("跨块逆序错位: 位置 %d 应为 %d, got %d", i, want-1-i, got[i])
		}
	}
}

// 统一契约：所有写操作自动递归创建父目录
func TestWriteOpsCreateParentDir(t *testing.T) {
	ws := testfixture.NewWorkspace(t)

	if err := AppendJsonl(ws.Join("a/b/c/usage.jsonl"), map[string]int{"x": 1}); err != nil {
		t.Fatalf("AppendJsonl 应自动建父目录: %v", err)
	}
	if err := SaveJson(ws.Join("d/e/config.json"), map[string]int{"x": 1}); err != nil {
		t.Fatalf("SaveJson 应自动建父目录: %v", err)
	}
	if err := WriteFileAtomic(ws.Join("f/g/raw.bin"), []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic 应自动建父目录: %v", err)
	}
}

func TestWriteJsonl(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	path := ws.Join("a/b/usage.jsonl") // 父目录不存在，验证自动创建

	items := []map[string]int{{"x": 1}, {"x": 2}}
	if err := WriteJsonl(path, items); err != nil {
		t.Fatalf("WriteJsonl 失败: %v", err)
	}
	got, err := LoadJsonl[map[string]int](path)
	if err != nil {
		t.Fatalf("LoadJsonl 失败: %v", err)
	}
	if len(got) != 2 || got[0]["x"] != 1 || got[1]["x"] != 2 {
		t.Fatalf("写读不一致: %v", got)
	}

	// 整体重写语义：旧内容被替换而非追加
	if err := WriteJsonl(path, []map[string]int{{"y": 9}}); err != nil {
		t.Fatalf("WriteJsonl 重写失败: %v", err)
	}
	got, _ = LoadJsonl[map[string]int](path)
	if len(got) != 1 || got[0]["y"] != 9 {
		t.Fatalf("重写后应只剩新内容: %v", got)
	}

	// 空切片：整文件清空（0 行）
	if err := WriteJsonl[map[string]int](path, nil); err != nil {
		t.Fatalf("WriteJsonl 空切片失败: %v", err)
	}
	if got, _ := LoadJsonl[map[string]int](path); len(got) != 0 {
		t.Fatalf("空切片应清空文件, got %v", got)
	}
}
