package web

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

// TestReplaceNilSlices 覆盖 nil→empty 替换的所有边界。
// 替换是构造新值（不改原对象），断言基于 reflect 检查 IsNil/Len。
func TestReplaceNilSlices(t *testing.T) {
	t.Run("顶层 nil 切片", func(t *testing.T) {
		var nilSlice []int
		got := replaceNilSlices(nilSlice)
		rv := reflect.ValueOf(got)
		if rv.Kind() != reflect.Slice || rv.IsNil() || rv.Len() != 0 {
			t.Fatalf("nil 切片应替换为非 nil 空切片, got %#v", got)
		}
	})

	t.Run("struct 内 nil 切片字段", func(t *testing.T) {
		type inner struct {
			Tags []string `json:"tags"`
			Name string   `json:"name"`
		}
		in := inner{Name: "x"} // Tags 为 nil
		got := replaceNilSlices(in).(inner)
		if got.Tags == nil || len(got.Tags) != 0 {
			t.Errorf("Tags 应为非 nil 空切片, got %#v", got.Tags)
		}
		if got.Name != "x" {
			t.Errorf("Name 应保持不变, got %q", got.Name)
		}
	})

	t.Run("嵌套 struct 内 nil 切片（切片元素是 struct）", func(t *testing.T) {
		type item struct {
			Tags []string `json:"tags"`
		}
		type outer struct {
			Items []item `json:"items"`
		}
		// Items 是非 nil 切片含 1 个元素；元素的 Tags 是 nil。
		// 验证递归会进到切片元素内部，把 nil Tags 替换掉。
		in := outer{Items: []item{{Tags: nil}}}
		got := replaceNilSlices(in).(outer)
		if len(got.Items) != 1 {
			t.Fatalf("Items 长度应为 1, got %d", len(got.Items))
		}
		if got.Items[0].Tags == nil || len(got.Items[0].Tags) != 0 {
			t.Errorf("嵌套元素 Tags 应为非 nil 空切片, got %#v", got.Items[0].Tags)
		}
	})

	t.Run("指针包裹的 struct", func(t *testing.T) {
		type s struct {
			Tags []string `json:"tags"`
		}
		in := &s{Tags: nil}
		got := replaceNilSlices(in).(*s)
		if got.Tags == nil || len(got.Tags) != 0 {
			t.Errorf("指针 struct 内 Tags 应为非 nil 空切片, got %#v", got.Tags)
		}
	})

	t.Run("nil map 替换为空 map", func(t *testing.T) {
		type s struct {
			M map[string]int `json:"m"`
		}
		in := s{} // M 为 nil
		got := replaceNilSlices(in).(s)
		if got.M == nil || len(got.M) != 0 {
			t.Errorf("nil map 应替换为非 nil 空 map, got %#v", got.M)
		}
	})

	t.Run("非 nil 空切片保持不变", func(t *testing.T) {
		in := []int{}
		got := replaceNilSlices(in).([]int)
		if got == nil || len(got) != 0 {
			t.Errorf("空切片应保持非 nil, got %#v", got)
		}
	})

	t.Run("非空切片内容保持不变", func(t *testing.T) {
		in := []int{1, 2, 3}
		got := replaceNilSlices(in).([]int)
		if !reflect.DeepEqual(got, in) {
			t.Errorf("非空切片内容应不变, got %#v want %#v", got, in)
		}
	})

	t.Run("unexported 字段被跳过不 panic", func(t *testing.T) {
		type s struct {
			hidden []int // 小写，不可 Set
			Tags   []string
		}
		in := s{Tags: nil}
		// 主要验证不 panic；hidden 字段不替换。
		got := replaceNilSlices(in).(s)
		if got.Tags == nil || len(got.Tags) != 0 {
			t.Errorf("Tags 应被替换, got %#v", got.Tags)
		}
	})

	t.Run("标量/字符串不变", func(t *testing.T) {
		cases := []any{42, "hello", true, 3.14}
		for _, in := range cases {
			got := reflect.ValueOf(replaceNilSlices(in)).Interface()
			if !reflect.DeepEqual(got, in) {
				t.Errorf("标量应不变, got %#v want %#v", got, in)
			}
		}
	})

	t.Run("nil 输入返回 nil", func(t *testing.T) {
		got := replaceNilSlices(nil)
		if got != nil {
			t.Errorf("nil 输入应返回 nil, got %#v", got)
		}
	})
}

// TestNilSliceJSONFormatMarshal 验证自定义 Format 的 Marshal 输出形态：
// nil 切片 → []，nil map → {}，与标准库 Encoder 行为对齐。
func TestNilSliceJSONFormatMarshal(t *testing.T) {
	type s struct {
		Tags []string       `json:"tags"`
		M    map[string]int `json:"m"`
	}

	var buf bytes.Buffer
	if err := nilSliceJSONFormat.Marshal(&buf, s{}); err != nil {
		t.Fatalf("Marshal 失败: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, buf.String())
	}
	if tags, ok := got["tags"].([]any); !ok || tags == nil {
		t.Errorf("tags 应为 [] 非 null, got %#v", got["tags"])
	}
	if m, ok := got["m"].(map[string]any); !ok || m == nil {
		t.Errorf("m 应为 {} 非 null, got %#v", got["m"])
	}
}
