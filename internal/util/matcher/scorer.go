package matcher

import "strings"

type Scorer interface {
	Score(target string, query string) float64
}

const (
	ScoreFailed  = 0
	ScoreSuccess = 1
)

var (
	defaultScorer Scorer = WordSegmentationScorer(defaultBonusScorer)
)

type ScorerFunc func(target, query string) float64

func (s ScorerFunc) Score(target string, query string) float64 {
	return s(target, query)
}

// SimpleScorer 简单计分器
// 仅当目标字符串有查询字符串所有字符的顺序出现(不要求连续)即为匹配，否则为不匹配；没有具体分数，无法表达匹配程度
var simpleScorer = ScorerFunc(func(target, query string) float64 {
	if len(query) == 0 {
		return ScoreSuccess
	}
	if len(target) == 0 {
		return ScoreFailed
	}

	targetRunes := []rune(strings.ToLower(target))
	queryRunes := []rune(strings.ToLower(query))

	queryIdx := 0
	for _, r := range targetRunes {
		if r == queryRunes[queryIdx] {
			queryIdx++
			if queryIdx == len(queryRunes) {
				// 匹配所有 query 字符，返回成功
				return ScoreSuccess
			}
		}
	}

	// 存在query未被匹配的字符，返回失败
	return ScoreFailed
})

// WordSegmentationScorer 带分词的计分器
func WordSegmentationScorer(inner Scorer) Scorer {
	return ScorerFunc(func(target, query string) float64 {
		if query == "" {
			return ScoreSuccess
		}

		querySegments := strings.Split(query, " ")

		var score float64 = 1
		for _, segment := range querySegments {
			segment = strings.TrimSpace(segment)
			score *= inner.Score(target, segment)
			if score == 0 {
				return ScoreFailed
			}
		}
		return score
	})
}
