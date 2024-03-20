package matcher

import (
	"cmp"
	"go-cube/internal/slicekit"
	"slices"
)

type Matcher[T any] struct {
	targets  []T
	scorer   Scorer
	keywords []string // targets 对应的 keyword 列表
}

func newMatcher[T any](targets []T, keywords []string, scorer Scorer) *Matcher[T] {
	if scorer == nil {
		scorer = defaultScorer
	}

	return &Matcher[T]{
		targets:  targets,
		scorer:   scorer,
		keywords: keywords,
	}
}

func NewKeywordMatcher[T any](targets []T, keywordGetter func(T) string, scorer Scorer) *Matcher[T] {
	keywords := slicekit.Map(targets, keywordGetter)
	return newMatcher(targets, keywords, scorer)
}

func NewStringMatcher(targets []string, scorer Scorer) *Matcher[string] {
	keywords := targets
	return newMatcher(targets, keywords, scorer)
}

func (m *Matcher[T]) Match(query string) []T {
	if query == "" {
		return slices.Clone(m.targets)
	}

	type scoredTarget struct {
		target T
		score  float64
	}

	// 记录匹配的对象和分数
	result := make([]scoredTarget, 0, len(m.targets))
	for i, keyword := range m.keywords {
		score := m.scorer.Score(keyword, query)
		if score > 0 {
			result = append(result, scoredTarget{
				target: m.targets[i],
				score:  score,
			})
		}
	}

	// 按分数排序
	slices.SortFunc(result, func(a, b scoredTarget) int {
		return cmp.Compare(b.score, a.score)
	})

	return slicekit.Map(result, func(t scoredTarget) T {
		return t.target
	})
}
