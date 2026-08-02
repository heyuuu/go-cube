package slicekit

import (
	"reflect"
	"testing"
)

func TestMap(t *testing.T) {
	// 空切片 → nil
	if got := Map([]int(nil), func(int) int { return 0 }); got != nil {
		t.Fatalf("空切片应返回 nil，实际 %v", got)
	}
	// 正常映射
	got := Map([]int{1, 2, 3}, func(x int) int { return x * 2 })
	want := []int{2, 4, 6}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Map 结果 %v，期望 %v", got, want)
	}
	// 类型转换
	gotStr := Map([]int{1, 2}, func(x int) string { return "n" })
	if len(gotStr) != 2 || gotStr[0] != "n" {
		t.Fatalf("类型转换 Map 异常: %v", gotStr)
	}
}

func TestFilter(t *testing.T) {
	if got := Filter([]int(nil), func(int) bool { return true }); got != nil {
		t.Fatalf("空切片应返回 nil，实际 %v", got)
	}
	got := Filter([]int{1, 2, 3, 4}, func(x int) bool { return x%2 == 0 })
	want := []int{2, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Filter 结果 %v，期望 %v", got, want)
	}
	// 全部过滤掉 → 空 slice（非 nil）
	got = Filter([]int{1, 3}, func(x int) bool { return x%2 == 0 })
	if len(got) != 0 {
		t.Fatalf("全过滤后应 len=0，实际 %v", got)
	}
}

func TestToSet(t *testing.T) {
	m := ToSet([]string{"a", "b", "a", "c", "b"})
	want := map[string]bool{"a": true, "b": true, "c": true}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("ToSet 结果 %v，期望 %v", m, want)
	}
	// 空切片 → 空 map
	if m := ToSet([]int{}); len(m) != 0 {
		t.Fatalf("空切片应返回空 map，实际 %v", m)
	}
}
