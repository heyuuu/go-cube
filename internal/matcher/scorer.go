package matcher

import "strings"

type Scorer interface {
	Score(target []Keyword, query string) float64
}

type StringScorer interface {
	StringScore(target string, query string) float64
}

const (
	scoreFailed  float64 = 0.0
	scoreSuccess float64 = 1.0
)

var (
	DefaultScorer Scorer = &baseScorer{stringScorer: defaultBonusScorer, isSplitQuery: true}
)

// baseScorer Scorer 的默认实现
type baseScorer struct {
	stringScorer StringScorer
	isSplitQuery bool
}

func NewBaseScorer(stringScorer StringScorer, isSplitQuery bool) Scorer {
	return &baseScorer{stringScorer: stringScorer, isSplitQuery: isSplitQuery}
}

func (d *baseScorer) Score(keywords []Keyword, query string) float64 {
	queryWords := d.splitQuery(query)

	if len(queryWords) == 0 {
		return scoreSuccess
	}

	var score float64 = 1
	for _, word := range queryWords {
		var wordScore float64 = 0
		for _, keyword := range keywords {
			wordScore += keyword.Weight * d.stringScorer.StringScore(keyword.String, word)
		}
		score *= wordScore
		if score == 0 {
			break
		}
	}
	return score
}

func (d *baseScorer) splitQuery(query string) []string {
	var queryWords []string
	if !d.isSplitQuery {
		word := strings.TrimSpace(query)
		if len(word) > 0 {
			queryWords = append(queryWords, word)
		}
	} else {
		for _, word := range strings.Split(query, " ") {
			word = strings.TrimSpace(word)
			if len(word) > 0 {
				queryWords = append(queryWords, word)
			}
		}
	}
	return queryWords
}

// SimpleStringScorer 简单计分器
// 仅当目标字符串有查询字符串所有字符的顺序出现(不要求连续)即为匹配，否则为不匹配；没有具体分数，无法表达匹配程度
type SimpleStringScorer struct{}

func (s SimpleStringScorer) StringScore(target string, query string) float64 {
	if len(query) == 0 {
		return scoreSuccess
	}
	if len(target) == 0 {
		return scoreFailed
	}

	targetRunes := []rune(strings.ToLower(target))
	queryRunes := []rune(strings.ToLower(query))

	var targetIndex, queryIndex = 0, 0
	for targetIndex < len(targetRunes) && queryIndex < len(queryRunes) {
		if targetRunes[targetIndex] == queryRunes[queryIndex] {
			targetIndex++
			queryIndex++
		} else {
			targetIndex++
		}
	}

	if queryIndex >= len(queryRunes) {
		return scoreSuccess
	} else {
		return scoreFailed
	}
}
