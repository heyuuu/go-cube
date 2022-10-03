package matcher

const ()

// BonusScorer Bonus计分器
// 根据不同匹配方式计算不同的得分(Bonus)，总计得分的计分器. 可以支持首字母、连续匹配等匹配方法得分更高的需求
var BonusScorer StringScorer = func(target string, query string) float64 {
	return 0
}
