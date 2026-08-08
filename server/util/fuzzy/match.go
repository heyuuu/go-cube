package fuzzy

import (
	"slices"
	"strings"

	"cube/util/slicekit"
)

func MatchBy[T any](query string, targets []T, keywordGetter func(T) string, scorer Scorer) []T {
	if scorer == nil {
		scorer = defaultScorer
	}

	query = strings.TrimSpace(query)
	if len(targets) == 0 || query == "" {
		return slices.Clone(targets)
	}

	// 遍历并打分
	type match struct {
		score int
		index int
	}
	var matches []match
	for i, target := range targets {
		score, ok := scorer(query, keywordGetter(target))
		if ok {
			matches = append(matches, match{score: score, index: i})
		}
	}

	// 按分数排序
	slices.SortStableFunc(matches, func(a, b match) int {
		return b.score - a.score
	})

	// 取顺序取原对象并返回
	return slicekit.Map(matches, func(m match) T {
		return targets[m.index]
	})
}
