package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// newFile 返回该测试专属的 settings.json 路径（初始不存在）。
func newFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "settings.json")
}

func TestLoadSection(t *testing.T) {
	tests := []struct {
		name    string
		content string // 空 = 文件不存在
		section string
		want    []string
	}{
		{"文件不存在", "", "openers", nil},
		{"文件坏JSON", "{bad", "openers", nil},
		{"节缺失", `{"other": 1}`, "openers", nil},
		{"节格式坏", `{"openers": "not-array"}`, "openers", nil},
		{"节为空数组", `{"openers": []}`, "openers", []string{}},
		{"正常读取", `{"openers": ["a","b"], "other": 1}`, "openers", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := newFile(t)
			if tt.content != "" {
				os.WriteFile(file, []byte(tt.content), 0o644)
			}
			var got []string
			LoadSection(file, tt.section, &got)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSaveSection(t *testing.T) {
	t.Run("新建文件并创建父目录", func(t *testing.T) {
		file := newFile(t)
		if err := SaveSection(file, "openers", []string{"a"}); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		var got []string
		LoadSection(file, "openers", &got)
		if len(got) != 1 || got[0] != "a" {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("其他节语义级透传", func(t *testing.T) {
		file := newFile(t)
		os.WriteFile(file, []byte("{\"other\": {\"z\": 1, \"a\": [1, 2]}}"), 0o644)
		if err := SaveSection(file, "openers", []string{"a"}); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		data, _ := os.ReadFile(file)
		var d map[string]json.RawMessage
		if err := json.Unmarshal(data, &d); err != nil {
			t.Fatalf("落盘文件不是合法 JSON: %v\n%s", err, data)
		}
		// 字节级透传做不到（Unmarshal 落 RawMessage 会压缩空白），断言语义等价
		var other, wantOther map[string]any
		json.Unmarshal(d["other"], &other)
		json.Unmarshal([]byte(`{"z": 1, "a": [1, 2]}`), &wantOther)
		if !reflect.DeepEqual(other, wantOther) {
			t.Fatalf("other 节被改写: %s", d["other"])
		}
		var got []string
		json.Unmarshal(d["openers"], &got)
		if len(got) != 1 || got[0] != "a" {
			t.Fatalf("openers 节内容错误: %v", got)
		}
	})

	t.Run("替换已有节", func(t *testing.T) {
		file := newFile(t)
		SaveSection(file, "openers", []string{"a"})
		if err := SaveSection(file, "openers", []string{"b", "c"}); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		var got []string
		LoadSection(file, "openers", &got)
		if len(got) != 2 || got[0] != "b" || got[1] != "c" {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("坏文件拒写且不被改动", func(t *testing.T) {
		file := newFile(t)
		bad := "{bad json"
		os.WriteFile(file, []byte(bad), 0o644)
		if err := SaveSection(file, "openers", []string{"a"}); err == nil {
			t.Fatal("坏文件应拒绝写入")
		}
		data, _ := os.ReadFile(file)
		if string(data) != bad {
			t.Fatalf("坏文件被改动: %s", data)
		}
	})

	t.Run("落盘两空格缩进", func(t *testing.T) {
		file := newFile(t)
		SaveSection(file, "openers", []string{"a"})
		data, _ := os.ReadFile(file)
		if !strings.Contains(string(data), "\n  \"openers\"") {
			t.Fatalf("非两空格缩进:\n%s", data)
		}
	})
}

func TestSaveSectionConcurrent(t *testing.T) {
	file := newFile(t)
	// 两个 goroutine 各写不同节，验收包级锁：最终两节并存、文件合法（-race 下无撕裂）
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			section := []string{"alpha", "beta"}[i]
			if err := SaveSection(file, section, []string{"v"}); err != nil {
				t.Errorf("并发写入失败: %v", err)
			}
		}(i)
	}
	wg.Wait()
	var alpha, beta []string
	LoadSection(file, "alpha", &alpha)
	LoadSection(file, "beta", &beta)
	if len(alpha) != 1 || alpha[0] != "v" || len(beta) != 1 || beta[0] != "v" {
		t.Fatalf("并发写后两节应并存: alpha=%v beta=%v", alpha, beta)
	}
}
