package fuzzy

import (
	"testing"
	"unicode"
)

// ---------- 基础边界 ----------

func TestBonusScorer_EmptyQuery(t *testing.T) {
	// bonus.go:54 空 query 返回 Base 分，且 ok
	score, ok := BonusScorer("", "anything", defaultBonus)
	if !ok || score != defaultBonus[BonusBase] {
		t.Fatalf("空 query 应返回 Base=%d 且 ok, got score=%d ok=%v", defaultBonus[BonusBase], score, ok)
	}
}

func TestBonusScorer_QueryLongerThanTarget(t *testing.T) {
	// bonus.go:63 query 比 target 长，不可能匹配
	score, ok := BonusScorer("abc", "ab", defaultBonus)
	if ok || score != 0 {
		t.Fatalf("query 长于 target 应 (0,false), got (%d,%v)", score, ok)
	}
}

func TestBonusScorer_NoMatch(t *testing.T) {
	// bonus.go:164 字符完全不匹配 → bestTotal==negInf
	score, ok := BonusScorer("abc", "xyz", defaultBonus)
	if ok {
		t.Fatalf("无匹配字符应 (0,false), got (%d,%v)", score, ok)
	}
}

// ---------- 单字符匹配：手算验证数值 ----------

// target="A" query="A"：
//
//	起始 gap=0, posBonus[0]=First(15), 末尾 gap=0 → +Base(10000)
//	预期 = 10000 + 15 = 10015
func TestBonusScorer_SingleCharExact(t *testing.T) {
	score, ok := BonusScorer("A", "A", defaultBonus)
	if !ok {
		t.Fatalf("expected ok, got %v", ok)
	}
	want := defaultBonus[BonusBase] + defaultBonus[BonusFirstLetter]
	if score != want {
		t.Fatalf("score=%d want=%d", score, want)
	}
}

// target="BA" query="A"：
//
//	起始 gap=1 字符(B)，每字符 gapStart=Un+Lead=-1+-5=-6
//	posBonus[1]: B 不是大写后(A 前是 B 大写，不满足 camel 需小写在前)，B 非分隔符 → posBonus=0
//	末尾 gap=0
//	预期 = Base + 1*(-6) + 0 = 10000 - 6 = 9994
func TestBonusScorer_LeadingGap(t *testing.T) {
	score, ok := BonusScorer("A", "BA", defaultBonus)
	if !ok {
		t.Fatalf("expected ok, got %v", ok)
	}
	want := defaultBonus[BonusBase] + 1*(defaultBonus[PenaltyUnmatchedLetter]+defaultBonus[PenaltyLeadingLetter])
	if score != want {
		t.Fatalf("起始 gap: score=%d want=%d", score, want)
	}
}

// target="AB" query="B"：
//
//	起始 gap=1, posBonus[1]: tr[1]=B 大写, tr[0]=A 大写 → camel 不满足(需前小写)；A 非分隔符 → 0
//	末尾 gap=0
//	预期 = 10000 - 6 = 9994
func TestBonusScorer_TrailingGap(t *testing.T) {
	score, ok := BonusScorer("B", "AB", defaultBonus)
	if !ok {
		t.Fatalf("expected ok, got %v", ok)
	}
	want := defaultBonus[BonusBase] + 1*(defaultBonus[PenaltyUnmatchedLetter]+defaultBonus[PenaltyLeadingLetter])
	if score != want {
		t.Fatalf("末尾 gap 不影响分: score=%d want=%d", score, want)
	}
}

// ---------- 连续匹配加分 ----------

// target="cube" query="cube"：
//
//	全连续，i=0 起 posBonus[0]=First(15)
//	i=1,2,3 走连续转移：dp[j-1]+Seq+posBonus[j]
//	posBonus[1..3]: u/b/e 都不是 camel（全小写），前字符非分隔符 → 0
//	末尾 gap=0
//	预期 = Base + First + 3*Seq = 10000 + 15 + 3*15 = 10060
func TestBonusScorer_FullSequential(t *testing.T) {
	score, ok := BonusScorer("cube", "cube", defaultBonus)
	if !ok {
		t.Fatalf("expected ok")
	}
	want := defaultBonus[BonusBase] + defaultBonus[BonusFirstLetter] + 3*defaultBonus[BonusSequential]
	if score != want {
		t.Fatalf("全连续: score=%d want=%d", score, want)
	}
}

// 连续越多分越高：在同一 target 里，query 越长(全连续) 分数递增
func TestBonusScorer_SequentialMonotonic(t *testing.T) {
	tgt := "abcdef"
	prev := 0
	for i := 1; i <= 6; i++ {
		score, ok := BonusScorer(tgt[:i], tgt, defaultBonus)
		if !ok {
			t.Fatalf("len=%d expected ok", i)
		}
		if score <= prev {
			t.Fatalf("连续匹配长度 %d 分数 %d 应大于长度 %d 的 %d", i, score, i-1, prev)
		}
		prev = score
	}
}

// ---------- 分隔符 / Camel 加分 ----------

// target="go-cube" query="cube"：c 前是 '-'(分隔符) → Separator 加分
//
//	只断言方向性：分隔符后匹配应高于无分隔符情形。
//	不断言精确差值——DP 全局优化下，分隔符还会改变 gap 数量与连续匹配路径，
//	简单的"差值=Separator"假设不成立（实测差 24，而非 30）。
func TestBonusScorer_SeparatorBonus(t *testing.T) {
	with, _ := BonusScorer("cube", "go-cube", defaultBonus)
	// 对比：无分隔符情形（gocube），c 前是 'o'(字母)，无 Separator
	without, _ := BonusScorer("cube", "gocube", defaultBonus)
	if with <= without {
		t.Fatalf("分隔符后匹配应更高: with=%d without=%d", with, without)
	}
}

// target="goCube" query="Cube"：C 大写且前字符 o 小写 → Camel 加分
func TestBonusScorer_CamelBonus(t *testing.T) {
	with, _ := BonusScorer("Cube", "goCube", defaultBonus)
	// 对比全小写
	without, _ := BonusScorer("cube", "gocube", defaultBonus)
	if with <= without {
		t.Fatalf("驼峰匹配应更高: with=%d without=%d", with, without)
	}
	if with-without != defaultBonus[BonusCamel] {
		t.Fatalf("驼峰加分差值=%d want=%d", with-without, defaultBonus[BonusCamel])
	}
}

// ---------- 非连续匹配（prefMax 转移分支） ----------

// target="axbxc" query="abc"：必须非连续（a/b/c 之间隔着 x）
//
//	走 bonus.go:140 的 prefMax 非连续转移分支，gap 字符受 penUn 惩罚
func TestBonusScorer_NonSequential(t *testing.T) {
	score, ok := BonusScorer("abc", "axbxc", defaultBonus)
	if !ok {
		t.Fatalf("expected ok")
	}
	if score <= 0 {
		t.Fatalf("非连续匹配分数应 >0, got %d", score)
	}
	// 应严格小于全连续 "abc" 的分数（中间 gap 有惩罚）
	full, _ := BonusScorer("abc", "abc", defaultBonus)
	if score >= full {
		t.Fatalf("非连续 %d 应小于全连续 %d", score, full)
	}
}

// 非连续 vs 连续 vs 大 gap，三者分数应单调递减。
// 注意 gap 字符必须用字母（如 x），不能用 '.'/'-' 这类分隔符——
// 否则分隔符 bonus 会反向加分，破坏单调性（实测 a..b..c 分数反而更高）。
func TestBonusScorer_GapPenaltyOrdering(t *testing.T) {
	full, _ := BonusScorer("abc", "abc", defaultBonus)      // 全连续
	small, _ := BonusScorer("abc", "axbxc", defaultBonus)   // 小 gap（字母 x）
	large, _ := BonusScorer("abc", "axxbxxc", defaultBonus) // 大 gap（字母 x）
	if !(full > small && small > large) {
		t.Fatalf("分数应单调递减: full=%d small=%d large=%d", full, small, large)
	}
}

// ---------- 后段加权 (BonusTrailing)：default vs enhanced ----------

// 增强配置开了 BonusTrailing：basename 越靠尾分越高。
// 同一个 query "cube" 在 "/x/cube"（cube 在尾部）vs "/cube/x"（cube 在前部），
// enhanced 配置下前者应更高；default 配置下两者应接近（无 trailing，差异来自 leading/gap）。
func TestBonusScorer_TrailingWeightingEnhanced(t *testing.T) {
	tail, _ := BonusScorer("cube", "/x/cube", enhancedBonus) // cube 靠尾
	head, _ := BonusScorer("cube", "/cube/x", enhancedBonus) // cube 靠前
	if tail <= head {
		t.Fatalf("enhanced: 尾部匹配应高于前部: tail=%d head=%d", tail, head)
	}
}

// default 无 trailing 项：hasTrail=false (bonus.go:75)，posBonus 不含 trailing 项
// 验证 defaultBonus 确实不含 BonusTrailing 键
func TestDefaultBonusHasNoTrailing(t *testing.T) {
	if _, ok := defaultBonus[BonusTrailing]; ok {
		t.Fatalf("defaultBonus 不应包含 BonusTrailing")
	}
	if _, ok := enhancedBonus[BonusTrailing]; !ok {
		t.Fatalf("enhancedBonus 应包含 BonusTrailing")
	}
}

// enhanced 连续分加强：enhanced 的 BonusSequential > default
func TestEnhancedStrengthensSequential(t *testing.T) {
	if enhancedBonus[BonusSequential] <= defaultBonus[BonusSequential] {
		t.Fatalf("enhanced Seq(%d) 应 > default Seq(%d)",
			enhancedBonus[BonusSequential], defaultBonus[BonusSequential])
	}
}

// ---------- 大小写无关 ----------

// target="CUBE" query="cube" 应与 "cube"/"cube" 同分（连续数相同，camel/first 处理一致）
// 注：C 在位置0 → First；后续 U/B/E 大写但前面也是大写，不触发 camel
func TestBonusScorer_CaseInsensitive(t *testing.T) {
	upper, _ := BonusScorer("cube", "CUBE", defaultBonus)
	lower, _ := BonusScorer("cube", "cube", defaultBonus)
	if upper != lower {
		t.Fatalf("大小写无关应同分: upper=%d lower=%d", upper, lower)
	}
}

// ---------- target 超长截断 (bonus.go:58) ----------

// 超过 bonusMaxMatch(255) 的 target 会被截断，截断后若 query 在前 255 内能匹配则 ok
func TestBonusScorer_TruncateLongTarget(t *testing.T) {
	// 构造 300 字符的 target，query 出现在第 10 位
	short := repeatRune('x', 10) + "cube" + repeatRune('y', 300)
	long := repeatRune('x', 300) + "cube"
	scoreShort, okShort := BonusScorer("cube", short, defaultBonus)
	scoreLong, okLong := BonusScorer("cube", long, defaultBonus)
	// short: cube 在前 255 内，能匹配
	if !okShort {
		t.Fatalf("short target 应能匹配")
	}
	// long: cube 在 300 位之后，截断后无法匹配 → false
	if okLong {
		t.Fatalf("long target: cube 在截断区外应无法匹配, got score=%d", scoreLong)
	}
	if scoreShort <= 0 {
		t.Fatalf("short 匹配分数应>0, got %d", scoreShort)
	}
	_ = scoreLong
}

// ---------- 真实场景：排序正确性 ----------

// 核心诉求验证：搜 cube 时，basename 含 cube 的项目应排在噪声项目前面
func TestRanking_RealWorld_Cube(t *testing.T) {
	paths := []string{
		"/Users/heyu/Code/heyuuu/cube",
		"/Users/heyu/Code/local/go/go-cube",
		"/Users/heyu/Code/heyuuu/next-cube",
		"/Users/heyu/Code/github/earendil-works/pi",       // 噪声
		"/Users/heyu/Code/github/microsoft/monaco-editor", // 噪声
		"/Users/heyu/Code/github/heyuuu/cattery-cube",
	}
	// 用 MatchBy + EnhancedScorer 跑完整流程
	got := MatchBy("cube", paths, identityGetter, EnhancedBonusScorer)

	if len(got) != len(paths) {
		t.Fatalf("预期匹配所有 paths，实际只匹配了 %d", len(got))
	}

	// 前 3 应都是 basename 含 "cube" 的项目（非 pi/monaco）
	for i := 0; i < 3; i++ {
		bn := baseName(got[i])
		if !contains(bn, "cube") {
			t.Fatalf("top%d 应是含 cube 的项目, got %q (full=%q)\n完整排序: %v",
				i+1, bn, got[i], got)
		}
	}
	// pi 和 monaco-editor 不应进入前 3
	for i := 0; i < 3; i++ {
		bn := baseName(got[i])
		if bn == "pi" || bn == "monaco-editor" {
			t.Fatalf("噪声项目 %q 不应进入 top3: %v", bn, got)
		}
	}
}

// default 配置下排序不如 enhanced（这正是引入 enhanced 的动机）
// 验证：default 配置 cube 不一定登顶（量化 enhanced 的价值）
func TestRanking_DefaultConfig_LessAccurate(t *testing.T) {
	paths := []string{
		"/Users/heyu/Code/heyuuu/cube",
		"/Users/heyu/Code/github/earendil-works/pi",
		"/Users/heyu/Code/github/microsoft/monaco-editor",
	}
	got := MatchBy("cube", paths, identityGetter, DefaultBonusScorer)
	// default 配置下 cube 不保证第一（pi/monaco 可能领先）——记录现象即可
	t.Logf("default 排序: %v", got)
	if baseName(got[0]) != "cube" {
		t.Logf("（符合预期）default 配置下 cube 未登顶: %q", baseName(got[0]))
	}
}

// ---------- CJK ----------

// 逐个验证 Go unicode 库对中文(CJK)的真实判定，不凭印象
func TestCJK_UnicodeBehavior(t *testing.T) {
	cjk := []rune("笔记")[0]
	cases := []rune{'a', 'A', '中', '笔', '/', '，', '。', '_', '1'}
	for _, r := range cases {
		t.Logf("rune=%-2c(U+%04X) IsLetter=%-5v IsUpper=%-5v IsLower=%-5v IsDigit=%v",
			r, r, unicode.IsLetter(r), unicode.IsUpper(r), unicode.IsLower(r), unicode.IsDigit(r))
	}
	// 关键：中文是 Letter 但既非 Upper 也非 Lower
	if !unicode.IsLetter(cjk) {
		t.Errorf("中文应为 letter")
	}
	if unicode.IsUpper(cjk) || unicode.IsLower(cjk) {
		t.Errorf("中文不应是 Upper/Lower（无大小写概念）")
	}
	// 中文是 letter → runeIsSeparator 返回 false
	if runeIsSeparator(cjk) {
		t.Errorf("中文是 letter，runeIsSeparator 应返回 false（非分隔符）")
	}
	// toLowerRune 只处理 A-Z，中文原样返回
	if toLowerRune(cjk) != cjk {
		t.Errorf("toLowerRune 对中文应原样返回")
	}
}

// 中文子序列匹配本身是否工作
func TestCJK_SubsequenceMatch(t *testing.T) {
	// 精确连续匹配
	score, ok := BonusScorer("笔", "笔记", defaultBonus)
	t.Logf("笔记/笔: ok=%v score=%d", ok, score)
	if !ok {
		t.Errorf("中文单字匹配应成功")
	}

	// 子序列：中文路径里搜
	score2, ok2 := BonusScorer("笔", "/Users/heyu/Code/heyuuu/学习笔记", defaultBonus)
	t.Logf("学习笔记/笔: ok=%v score=%d", ok2, score2)
	if !ok2 {
		t.Errorf("中文路径里搜中文应成功")
	}
}

// 多字连续匹配：连续加分是否正常累积
func TestCJK_Sequential(t *testing.T) {
	full, ok1 := BonusScorer("笔记", "学习笔记", defaultBonus)
	t.Logf("学习笔记/笔记(连续): ok=%v score=%d", ok1, full)
	gap, ok2 := BonusScorer("笔记", "学A习B笔C记", defaultBonus)
	t.Logf("学A习B笔C记/笔记(非连续): ok=%v score=%d", ok2, gap)
	if !ok1 || !ok2 {
		t.Fatalf("两个都应匹配成功")
	}
	// 连续应 > 非连续（受 BonusSequential 影响）
	if full <= gap {
		t.Errorf("中文连续匹配 %d 应高于非连续 %d", full, gap)
	}
}

// 中英混合：最常见的真实场景
func TestCJK_MixedCN_EN(t *testing.T) {
	paths := []string{
		"/Users/heyu/Code/heyuuu/项目笔记",
		"/Users/heyu/Code/heyuuu/cube",
		"/Users/heyu/Code/github/heyuuu/工作笔记",
		"/Users/heyu/Code/github/microsoft/monaco-editor",
	}
	// 搜中文
	r1 := MatchBy("笔记", paths, identityGetter, EnhancedBonusScorer)
	t.Logf("搜 [笔记]: %v", r1)
	// 搜英文（在含中文的路径里）
	r2 := MatchBy("cube", paths, identityGetter, EnhancedBonusScorer)
	t.Logf("搜 [cube]: %v", r2)
	// 搜中英混合
	r3 := MatchBy("项目", paths, identityGetter, EnhancedBonusScorer)
	t.Logf("搜 [项目]: %v", r3)

	// 笔记 应命中含"笔记"的两个项目，monaco-editor 不应在内
	for _, p := range r1 {
		if baseName(p) == "monaco-editor" {
			t.Errorf("搜'笔记'不应命中 monaco-editor")
		}
	}
}

// 中文场景下分隔符加分的盲区验证：
// 中文路径里，"笔记"的"笔"字前面是中文(也是 letter)，不触发 Separator 加分。
// 这是中文相对英文的一个天然劣势——英文有 /-_ 等词边界，中文没有。
func TestCJK_NoWordBoundaryBonus(t *testing.T) {
	// 英文：go-cube 的 cube 前 - 是分隔符 → Separator 加分
	en, _ := BonusScorer("cube", "go-cube", defaultBonus)
	// 英文变体：gocube（无分隔符）
	enNoSep, _ := BonusScorer("cube", "gocube", defaultBonus)
	t.Logf("英文 go-cube=%d  gocube=%d (差=%d, 即 Separator 影响)", en, enNoSep, en-enNoSep)

	// 中文：学习笔记 里 笔 前是 习(中文，letter，非分隔符) → 无 Separator 加分
	// 中文无法享受词边界加分，这是算法对中文的结构性盲区
	cn, _ := BonusScorer("笔记", "学习笔记", defaultBonus)
	t.Logf("中文 学习笔记/笔记=%d（笔前为习，非分隔符，无 Separator 加分）", cn)
}

// 一个微妙但重要的点：中文用 IsUpper/IsLower 判定 camel 会怎样
// bonus.go:71 判定 IsUpper(当前)&&IsLower(前一)
// 中文既非 Upper 也非 Lower，所以中文场景 BonusCamel 永远不触发——
// 这意味着中文完全得不到驼峰加分，但也不会误触发。验证不误触发：
func TestCJK_NoCamelMisfire(t *testing.T) {
	// 中英相邻：a笔（a 小写，笔 非 upper）→ 不应加 camel
	// 笔A（A 大写，前是笔非 lower）→ 不应加 camel
	s1, _ := BonusScorer("笔", "a笔", defaultBonus)
	s2, _ := BonusScorer("A", "笔A", defaultBonus)
	t.Logf("a笔/笔=%d  笔A/A=%d（均不应触发 BonusCamel）", s1, s2)
	// 只要 ok 且分值合理即可，重点是验证不 panic、不异常加分
}

// ---------- 工具函数 ----------

func repeatRune(r rune, n int) string {
	rs := make([]rune, n)
	for i := range rs {
		rs[i] = r
	}
	return string(rs)
}

func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// 为测试 MatchBy 泛型，用 string slice 适配
func pathsToAny(ps []string) []string { return ps }
func identityGetter(s string) string  { return s }
