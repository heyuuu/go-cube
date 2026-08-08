package fuzzy

import (
	"maps"
	"unicode"
)

// bonus 分数类型
const (
	BonusBase               = iota + 1 // 匹配上的基础分
	BonusSequential                    // 连续匹配
	BonusSeparator                     // 单词首字符匹配
	BonusCamel                         // 驼峰大写字符的加分
	BonusFirstLetter                   // 首个字符匹配
	BonusTrailing                      // 后段加权项：匹配点越靠近 target 末尾加分越高，单位 分/字符距离。
	PenaltyUnmatchedLetter             // 每个未匹配字符的惩罚，单位 分/字符
	PenaltyLeadingLetter               // 距离首个字符距离的惩罚分，单位 分/字符
	PenaltyMaxLeadingLetter            // 距离首个字符距离的总惩罚分上限
)

var defaultBonus map[int]int = map[int]int{
	BonusBase:               10000,
	BonusSequential:         15,
	BonusSeparator:          30,
	BonusCamel:              30,
	BonusFirstLetter:        15,
	PenaltyLeadingLetter:    -5,
	PenaltyUnmatchedLetter:  -1,
	PenaltyMaxLeadingLetter: -15,
}

var enhancedBonus map[int]int

func init() {
	enhancedBonus = maps.Clone(defaultBonus)
	enhancedBonus[BonusSequential] = 60
	enhancedBonus[BonusTrailing] = 80
}

const bonusMaxMatch = 255

func BonusScorer(query string, target string, bonus map[int]int) (int, bool) {
	if len(query) == 0 {
		return bonus[BonusBase], true
	}
	tr := []rune(target)
	if len(tr) > bonusMaxMatch {
		tr = tr[:bonusMaxMatch]
	}
	qr := []rune(query)
	n, m := len(tr), len(qr)
	if m > n {
		return 0, false // query 比 target 长，不可能匹配
	}

	b := bonus
	penUn := b[PenaltyUnmatchedLetter] // 中间/末尾 gap 每字符惩罚 (=-1)
	penLead := b[PenaltyLeadingLetter] // 起始 gap 额外每字符惩罚 (=-5)
	gapStart := penUn + penLead        // 起始 gap 每字符总惩罚 (=-6)
	bonusSeq := b[BonusSequential]
	baseScore := b[BonusBase]

	// posBonus[j]: target[j] 作为匹配位的位置局部分（不含 gap、不含连续分）
	bonusTrail, hasTrail := b[BonusTrailing]
	posBonus := make([]int, n)
	for j := 0; j < n; j++ {
		var pb int
		if j == 0 {
			pb = b[BonusFirstLetter]
		} else {
			if unicode.IsUpper(tr[j]) && unicode.IsLower(tr[j-1]) {
				pb += b[BonusCamel]
			}
			if runeIsSeparator(tr[j-1]) {
				pb += b[BonusSeparator]
			}
		}
		// 后段加权：匹配点距末尾越近分越高。distFromEnd = n-1-j，
		// 取负作为惩罚（靠前负分，靠尾零分），使 basename（尾部）获得相对优势。
		if hasTrail {
			pb += bonusTrail * (j - (n - 1))
		}
		posBonus[j] = pb
	}

	const negInf = -1 << 30
	// dp[j]: query 前 i 个字符匹配完毕、最后匹配落在 target[j] 的最大分（不含末尾 gap）
	dp := make([]int, n)
	for j := range dp {
		dp[j] = negInf
	}

	for i := 0; i < m; i++ {
		ndp := make([]int, n)
		for j := range ndp {
			ndp[j] = negInf
		}
		// 前缀最大优化（penUn<0）：
		//   非连续转移 cost(k→j) = dp[k] + (j-k-1)*penUn + posBonus[j]
		//                        = (j-1)*penUn + posBonus[j] + max_{k<=j-2}(dp[k] - k*penUn)
		//   因 penUn<0，-k*penUn 单调，维护 prefMax = max_{k<t}(dp[k] - k*penUn) 即可。
		var prefMax int = negInf
		for j := i; j <= n-(m-i); j++ { // 剪枝：为剩余 query 字符留足位置
			// 先推进 prefMax：把 j-1 作为潜在前驱加入（无论本位是否匹配）
			if j-1 >= 0 && j-1 >= i-1 && dp[j-1] != negInf {
				cand := dp[j-1] - (j-1)*penUn
				if cand > prefMax {
					prefMax = cand
				}
			}

			if toLowerRune(tr[j]) != toLowerRune(qr[i]) {
				continue
			}

			best := negInf
			if i == 0 {
				// 起始 gap：j 个未匹配字符
				best = j*gapStart + posBonus[j]
			} else {
				// 连续：k = j-1，gap = 0
				if dp[j-1] != negInf {
					cand := dp[j-1] + bonusSeq + posBonus[j]
					if cand > best {
						best = cand
					}
				}
				// 非连续：k <= j-2，gap >= 1
				if prefMax != negInf {
					cand := prefMax + (j-1)*penUn + posBonus[j]
					if cand > best {
						best = cand
					}
				}
			}
			ndp[j] = best
		}
		dp = ndp
	}

	// 收尾：末尾 gap（末匹配位 j 之后到 n-1，每字符 penUn）+ 基础分
	bestTotal := negInf
	for j := m - 1; j < n; j++ {
		if dp[j] == negInf {
			continue
		}
		tail := (n - 1 - j) * penUn
		total := dp[j] + tail
		if total > bestTotal {
			bestTotal = total
		}
	}
	if bestTotal == negInf {
		return 0, false
	}
	return bestTotal + baseScore, true
}

func toLowerRune(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}

func runeIsSeparator(r rune) bool {
	return !unicode.IsLetter(r)
}
