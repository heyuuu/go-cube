package settings

import (
	"strings"
	"testing"
)

func TestUpsertKeyed(t *testing.T) {
	type item struct {
		k string
		v int
	}
	keyOf := func(x item) string { return x.k }
	list := []item{{"a", 1}, {"b", 2}}

	got := UpsertKeyed(list, keyOf, item{"b", 9})
	if len(got) != 2 || got[1].v != 9 {
		t.Fatalf("命中键应原位替换: %+v", got)
	}
	got = UpsertKeyed(list, keyOf, item{"c", 3})
	if len(got) != 3 || got[2].k != "c" {
		t.Fatalf("未命中键应追加末尾: %+v", got)
	}
}

func TestRemoveKeyed(t *testing.T) {
	type item struct{ k string }
	keyOf := func(x item) string { return x.k }
	list := []item{{"a"}, {"b"}, {"a"}}

	rest, removed := RemoveKeyed(list, keyOf, "a")
	if !removed || len(rest) != 1 || rest[0].k != "b" {
		t.Fatalf("应删除全部命中项: rest=%+v removed=%v", rest, removed)
	}
	rest, removed = RemoveKeyed(list, keyOf, "zz")
	if removed || len(rest) != len(list) {
		t.Fatalf("未命中应原样返回且 removed=false: %+v %v", rest, removed)
	}
}

func TestReorderKeyed(t *testing.T) {
	type item struct{ k string }
	keyOf := func(x item) string { return x.k }
	list := []item{{"a"}, {"b"}, {"c"}, {"d"}}

	got, err := ReorderKeyed(list, keyOf, []string{"c", "a"}, "项", func(k string) string { return k })
	if err != nil {
		t.Fatalf("合法重排不应报错: %v", err)
	}
	if len(got) != 4 || got[0].k != "c" || got[1].k != "a" || got[2].k != "b" || got[3].k != "d" {
		t.Fatalf("未列出条目应原序殿后: %+v", got)
	}

	if _, err = ReorderKeyed(list, keyOf, []string{"zz"}, "项", func(k string) string { return k }); err == nil || !strings.Contains(err.Error(), "未找到指定") {
		t.Fatalf("未知键应报中文错误, got %v", err)
	}
	if _, err = ReorderKeyed(list, keyOf, []string{"a", "a"}, "项", func(k string) string { return k }); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("重复键应报中文错误, got %v", err)
	}
	if _, err = ReorderKeyed([]item{{"a"}, {"a"}}, keyOf, []string{"a"}, "项", func(k string) string { return k }); err == nil || !strings.Contains(err.Error(), "重复键") {
		t.Fatalf("存储清单键重复应报中文错误, got %v", err)
	}
}
