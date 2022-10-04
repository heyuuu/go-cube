package matcher

import (
	"unicode"
)

// bonus 分数类型
const (
	BonusBase               = iota // 匹配上的基础分
	BonusSequential                // 连续匹配
	BonusSeparator                 // 单词首字符匹配
	BonusCamel                     // 驼峰大写字符的加分
	BonusFirstLetter               // 首个字符匹配
	PenaltyUnmatchedLetter         // 每个未匹配字符的惩罚，单位 分/字符
	PenaltyLeadingLetter           // 距离首个字符距离的惩罚分，单位 分/字符
	PenaltyMaxLeadingLetter        // 距离首个字符距离的总惩罚分上限
)

var defaultBonusScorer StringScorer = BonusStringScorer{
	maxMatch:          255,
	maxRecursiveLimit: 10,
	bonus: map[int]int{
		BonusBase:               10000,
		BonusSequential:         15,
		BonusSeparator:          30,
		BonusCamel:              30,
		BonusFirstLetter:        15,
		PenaltyLeadingLetter:    -5,
		PenaltyUnmatchedLetter:  -1,
		PenaltyMaxLeadingLetter: -15,
	},
}

// BonusStringScorer Bonus计分器
// 根据不同匹配方式计算不同的得分(Bonus)，总计得分的计分器. 可以支持首字母、连续匹配等匹配方法得分更高的需求
type BonusStringScorer struct {
	maxMatch          int         // 最长匹配字符数，太长影响性能
	maxRecursiveLimit int         // 最大递归深度，避免爆栈
	bonus             map[int]int // 各加分项对应分值
}

func (b BonusStringScorer) StringScore(target string, query string) float64 {
	if len(query) == 0 {
		return float64(b.bonus[BonusBase])
	}

	queryRunes := []rune(query)
	targetRunes := []rune(target)
	if len(targetRunes) > b.maxMatch {
		targetRunes = targetRunes[:b.maxMatch]
	}

	_, bestScore := b.matchRecursive(targetRunes, queryRunes, 0, 0, nil, 0)

	return float64(bestScore)
}

func (b BonusStringScorer) matchRecursive(targetRunes []rune, queryRunes []rune, targetIndex int, queryIndex int, matches []int, recursiveCount int) ([]int, int) {
	// 终止条件
	if queryIndex == len(queryRunes) {
		// 全部query匹配完，范围当前匹配
		return matches, b.calcScore(targetRunes, matches)
	} else if targetIndex >= len(targetRunes) {
		// 全部target使用完，返回未匹配
		return nil, 0
	} else if recursiveCount >= b.maxRecursiveLimit {
		// 超过递归层级，返回未匹配
		return nil, 0
	}

	var bestScore = 0
	var bestMatches []int
	for targetIndex < len(targetRunes) && queryIndex < len(queryRunes) {
		targetRune := unicode.ToLower(targetRunes[targetIndex])
		queryRune := unicode.ToLower(queryRunes[queryIndex])
		if targetRune == queryRune {
			// 获取当前未匹配时的最优结果
			aMatches, aScore := b.matchRecursive(targetRunes, queryRunes, targetIndex+1, queryIndex, matches, recursiveCount+1)
			if aScore > bestScore {
				bestMatches, bestScore = aMatches, aScore
			}

			// 当前匹配循环的步进
			matches = append(matches, targetIndex)

			targetIndex++
			queryIndex++
		} else {
			targetIndex++
		}
	}

	if queryIndex == len(queryRunes) {
		score := b.calcScore(targetRunes, matches)
		if score > bestScore {
			bestMatches, bestScore = matches, score
		}
	}

	return bestMatches, bestScore
}

func (b BonusStringScorer) calcScore(targetRunes []rune, matches []int) int {
	bonusItems := make(map[int]int)

	// 基础得分
	bonusItems[BonusBase] = 1

	// 首个匹配距离的惩罚分
	bonusItems[PenaltyLeadingLetter] = matches[0]

	// 每个未匹配字符的惩罚分
	bonusItems[PenaltyUnmatchedLetter] = len(targetRunes) - len(matches)

	// 逐匹配判断分数
	for queryIndex, targetIndex := range matches {
		// 连续匹配加分
		if queryIndex > 0 && targetIndex == matches[queryIndex-1]+1 {
			bonusItems[BonusSequential] += 1
		}

		if targetIndex == 0 {
			// 首字符匹配的加分
			bonusItems[BonusFirstLetter] = 1
		} else {
			// 驼峰大写字母匹配的加分
			if unicode.IsUpper(targetRunes[targetIndex]) && unicode.IsLower(targetRunes[targetIndex-1]) {
				bonusItems[BonusCamel] += 1
			}

			// 分隔符后首字符匹配的加分
			if runeIsSeparator(targetRunes[targetIndex-1]) {
				bonusItems[BonusSeparator] += 1
			}
		}
	}

	// 计算分数
	bonus := 0
	for bonusType, bonusCount := range bonusItems {
		bonus += b.bonus[bonusType] * bonusCount
	}

	return bonus
}

func runeIsSeparator(r rune) bool {
	return !unicode.IsLetter(r)
}
