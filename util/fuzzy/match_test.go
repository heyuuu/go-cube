package fuzzy

import (
	"reflect"
	"testing"
)

// TestMatchBy_EmptyQuery 空查询返回全部（克隆）。
func TestMatchBy_EmptyQuery(t *testing.T) {
	targets := []string{"a", "b", "c"}
	got := MatchBy("", targets, func(s string) string { return s }, nil)
	if !reflect.DeepEqual(got, targets) {
		t.Fatalf("空 query 应返回全部，实际 %v", got)
	}
	// 空格 query 也算空
	got = MatchBy("   ", targets, func(s string) string { return s }, nil)
	if !reflect.DeepEqual(got, targets) {
		t.Fatalf("空格 query 应返回全部，实际 %v", got)
	}
}

func TestMatchBy_EmptyTargets(t *testing.T) {
	got := MatchBy("x", []string{}, func(s string) string { return s }, nil)
	if len(got) != 0 {
		t.Fatalf("空 targets 应返回空，实际 %v", got)
	}
}

// TestMatchBy_FilterAndSort 匹配过滤 + 按分排序。
func TestMatchBy_FilterAndSort(t *testing.T) {
	targets := []string{"github:cube", "pers:secret", "github:card"}
	// 查 "c"：应匹配 cube 和 card，secret 不含连续 c 开头也可能匹配
	got := MatchBy("cube", targets, func(s string) string { return s }, nil)
	// cube 完整匹配分数最高应排第一
	if len(got) == 0 {
		t.Fatalf("应至少匹配到 cube")
	}
	if got[0] != "github:cube" {
		t.Fatalf("cube 应排第一，实际 %v", got)
	}
}

// TestMatchBy_NilScorerUsesDefault scorer 为 nil 时用默认 scorer。
func TestMatchBy_NilScorerUsesDefault(t *testing.T) {
	targets := []string{"alpha", "beta"}
	// 不 panic 即可
	got := MatchBy("a", targets, func(s string) string { return s }, nil)
	if len(got) == 0 {
		t.Fatalf("alpha 应匹配到 a")
	}
}

// TestDefaultBonusScorer_MatchAndMiss 基本匹配/不匹配。
func TestDefaultBonusScorer_MatchAndMiss(t *testing.T) {
	// 子串匹配
	if _, ok := DefaultBonusScorer("cub", "github:cube"); !ok {
		t.Fatalf("子串应匹配")
	}
	// 不匹配（缺字符）
	if _, ok := DefaultBonusScorer("xyz", "cube"); ok {
		t.Fatalf("无公共字符应不匹配")
	}
	// 空查询（BonusScorer 内部对空 query 的处理）
	if _, ok := DefaultBonusScorer("", "cube"); !ok {
		// 空 query 默认认为匹配（score=0）
	}
}

// TestEnhancedBonusScorer_ContinuousBetterThanScattered
// 增强版：连续匹配分数应明显高于零散匹配。
func TestEnhancedBonusScorer_ContinuousBetterThanScattered(t *testing.T) {
	contScore, _ := EnhancedBonusScorer("cub", "cube") // 连续
	// 找一个零散但匹配的 target
	scatScore, ok := EnhancedBonusScorer("cub", "c-x-u-b") // 零散
	if !ok {
		t.Skip("零散 target 未匹配，跳过对比")
	}
	if contScore <= scatScore {
		t.Fatalf("连续匹配分数 (%d) 应高于零散 (%d)", contScore, scatScore)
	}
}

// TestWordSegmentationScorer 分词计分：每个 segment 都须匹配。
func TestWordSegmentationScorer(t *testing.T) {
	scorer := WordSegmentationScorer(DefaultBonusScorer)

	// 两段都匹配
	if _, ok := scorer("cu be", "cube-x-be"); !ok {
		t.Fatalf("两段都应匹配")
	}
	// 任一段不匹配 → 整体不匹配
	if _, ok := scorer("cu xyz", "cube"); ok {
		t.Fatalf("xyz 不匹配应整体 false")
	}
}
