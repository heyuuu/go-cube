package matcher

import "sort"

type Keyword struct {
	String string
	Weight float64
}

type Matcher[T any] struct {
	targets  []T
	scorer   Scorer
	keywords [][]Keyword // targets 对应的 keywords 列表
}

func NewMatcher[T any](targets []T, keywordGetter func(T) []Keyword, scorer Scorer) *Matcher[T] {
	var targetKeywords = make([][]Keyword, len(targets))
	for index, target := range targets {
		targetKeywords[index] = keywordGetter(target)
	}
	return &Matcher[T]{targets: targets, scorer: scorer, keywords: targetKeywords}
}

func NewStringMatcher(targets []string, scorer Scorer) *Matcher[string] {
	return NewMatcher(targets, func(t string) []Keyword {
		return []Keyword{{String: t, Weight: 1.0}}
	}, scorer)
}

func (m *Matcher[T]) Match(query string) []T {
	// 记录匹配的对象和分数
	var result []T
	var scores []float64
	for i, keyword := range m.keywords {
		score := m.scorer.Score(keyword, query)
		if score > 0 {
			result = append(result, m.targets[i])
			scores = append(scores, score)
		}
	}

	// 按分数排序
	sort.Slice(result, func(i, j int) bool {
		return scores[i] > scores[j]
	})

	return result
}
