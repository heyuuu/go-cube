package fuzzy

import "strings"

type Scorer func(query string, target string) (score int, ok bool)

var defaultScorer Scorer = WordSegmentationScorer(DefaultBonusScorer)

// DefaultBonusScorer 默认 fuzzy 算法及分数的匹配器
func DefaultBonusScorer(query string, target string) (score int, ok bool) {
	return BonusScorer(query, target, defaultBonus)
}

// EnhancedBonusScorer 增强配置：在默认配置基础上
//   - 大幅加强连续匹配（BonusSequential 15 → 60），整词连续远超零散拼凑
//   - 新增后段加权（BonusTrailing），让 basename（路径尾部）获得额外加分
//
// 实测对全 PATH 匹配场景效果显著：cube 类查询不再被 pi/monaco-editor 等噪声项目压过。
func EnhancedBonusScorer(query string, target string) (score int, ok bool) {
	return BonusScorer(query, target, enhancedBonus)
}

// WordSegmentationScorer 带分词的计分器
func WordSegmentationScorer(inner Scorer) Scorer {
	return func(query string, target string) (int, bool) {
		segments := strings.Fields(query)
		var totalScore int
		for _, segment := range segments {
			score, ok := inner(segment, target)
			if !ok {
				return 0, false
			}
			totalScore += score
		}
		return totalScore, true
	}
}
