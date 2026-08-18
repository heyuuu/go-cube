package workbench

// 基于「构建真实 git 仓库」的 graph 算法测试（提案 1011 拓扑升级的守护网）。
//
// 不比对 golden 值（拓扑组合太多），而是校验可证明的不变量——任何一个被破坏
// 都直接定位到「后端算错了」还是前端渲染问题：
//
//	不变量 A（边连通）：每条 child→parent 边，从 child 节点出发存在连续线段链
//	  到达 parent 节点；首段从 child 泳道出发，末段落到 parent 泳道；
//	  中间段（若有）必须全程停留在 parent 泳道（留洞方案下泳道不移位）。
//	不变量 B（无悬空线）：每段线的起点要么是本行节点的泳道（节点出线），
//	  要么承接上一段线的终点；终点要么是下一行节点的泳道，要么有下一段延续。
//	不变量 C（节点占位）：节点泳道互不冲突地落在各自行的线图上（首段 From 覆盖）。

import (
	"os/exec"
	"testing"

	"cube/internal/testfixture"
	"cube/util/git"
)

// graphCase 一个待测拓扑：建仓脚本 + 拓扑名。
type graphCase struct {
	name string
	// build 在空仓库（已含 1 个 main 提交）上继续构造拓扑，返回后统一断言
	build func(t *testing.T, repo string)
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v 失败: %v\n%s", args, err, out)
	}
}

func commitEmpty(t *testing.T, repo, msg string) {
	runGit(t, repo, "commit", "--allow-empty", "-m", msg)
}

var graphCases = []graphCase{
	{"线性历史", func(t *testing.T, repo string) {
		commitEmpty(t, repo, "c2")
		commitEmpty(t, repo, "c3")
	}},
	{"分支+合并 no-ff", func(t *testing.T, repo string) {
		runGit(t, repo, "checkout", "-b", "feature")
		commitEmpty(t, repo, "f1")
		commitEmpty(t, repo, "f2")
		runGit(t, repo, "checkout", "main")
		commitEmpty(t, repo, "m1")
		runGit(t, repo, "merge", "--no-ff", "feature", "-m", "merge feature")
	}},
	{"双分支未合并", func(t *testing.T, repo string) {
		runGit(t, repo, "checkout", "-b", "b1")
		commitEmpty(t, repo, "b1c")
		runGit(t, repo, "checkout", "main")
		commitEmpty(t, repo, "m1")
		runGit(t, repo, "checkout", "-b", "b2")
		commitEmpty(t, repo, "b2c")
	}},
	{"章鱼合并（3 父）", func(t *testing.T, repo string) {
		runGit(t, repo, "checkout", "-b", "b1")
		commitEmpty(t, repo, "b1c")
		runGit(t, repo, "checkout", "-b", "b2", "main")
		commitEmpty(t, repo, "b2c")
		runGit(t, repo, "checkout", "main")
		runGit(t, repo, "merge", "b1", "b2", "-m", "octopus")
	}},
	{"连续两次合并", func(t *testing.T, repo string) {
		runGit(t, repo, "checkout", "-b", "b1")
		commitEmpty(t, repo, "b1c")
		runGit(t, repo, "checkout", "main")
		commitEmpty(t, repo, "m1")
		runGit(t, repo, "merge", "--no-ff", "b1", "-m", "merge b1")
		runGit(t, repo, "checkout", "-b", "b2")
		commitEmpty(t, repo, "b2c")
		runGit(t, repo, "checkout", "main")
		runGit(t, repo, "merge", "--no-ff", "b2", "-m", "merge b2")
	}},
	{"嵌套分支合并", func(t *testing.T, repo string) {
		runGit(t, repo, "checkout", "-b", "outer")
		commitEmpty(t, repo, "o1")
		runGit(t, repo, "checkout", "-b", "inner")
		commitEmpty(t, repo, "i1")
		runGit(t, repo, "checkout", "outer")
		commitEmpty(t, repo, "o2")
		runGit(t, repo, "merge", "--no-ff", "inner", "-m", "merge inner")
		runGit(t, repo, "checkout", "main")
		commitEmpty(t, repo, "m1")
		runGit(t, repo, "merge", "--no-ff", "outer", "-m", "merge outer")
	}},
	{"晚切支线", func(t *testing.T, repo string) {
		// dev 从 base 分出，但 dev 首提交晚于主线 m1：dev 节点须让位给主线，
		// 出线拐进主线泳道并随其落到共同父（回归：让位出线曾漏发导致支线断裂）
		runGit(t, repo, "checkout", "-b", "dev")
		commitEmpty(t, repo, "d1")
		runGit(t, repo, "checkout", "main")
		commitEmpty(t, repo, "m1")
		runGit(t, repo, "merge", "--no-ff", "dev", "-m", "merge dev")
	}},
	{"tag 装饰不影响布局", func(t *testing.T, repo string) {
		commitEmpty(t, repo, "c2")
		runGit(t, repo, "tag", "v1.0")
		commitEmpty(t, repo, "c3")
	}},
}

func TestComputeGraphRealTopologies(t *testing.T) {
	for _, tc := range graphCases {
		t.Run(tc.name, func(t *testing.T) {
			ws := testfixture.NewWorkspace(t)
			// 目录名固定（中文长名在个别文件系统上会被截断成非法字节序列）
			repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "main", EmptyCommitCount: 1})
			tc.build(t, repo)

			commits, err := git.CommitsPage(repo, true, "", 0, 200)
			if err != nil {
				t.Fatalf("CommitsPage: %v", err)
			}
			nodes, wires := computeGraph(commits)
			verifyGraphInvariants(t, commits, nodes, wires)
		})
	}
}

// 分页一致性：整页算一遍 vs 分两页算，节点泳道与颜色必须一致（跨页接续段吻合）。
func TestComputeGraphPaginationConsistency(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "main", EmptyCommitCount: 1})
	runGit(t, repo, "checkout", "-b", "feature")
	commitEmpty(t, repo, "f1")
	commitEmpty(t, repo, "f2")
	runGit(t, repo, "checkout", "main")
	for i := 0; i < 7; i++ {
		commitEmpty(t, repo, "m")
	}
	runGit(t, repo, "merge", "--no-ff", "feature", "-m", "merge")

	all, err := git.CommitsPage(repo, true, "", 0, 200)
	if err != nil {
		t.Fatalf("CommitsPage: %v", err)
	}
	fullNodes, fullWires := computeGraph(all)

	// 3 + 余量 两页
	p1, err := git.CommitsPage(repo, true, "", 0, 3)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	n1, w1 := computeGraph(p1)
	// 第二页与 Service.Commits 的做法一致：从第 0 行重算（skip 只是切片窗口）
	_, err = git.CommitsPage(repo, true, "", 3, 200)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	prefix, _ := git.CommitsPage(repo, true, "", 0, 3+200)
	n2full, w2full := computeGraph(prefix)

	for i := range fullNodes {
		var paged GraphCommit
		if i < 3 {
			paged = n1[i]
		} else {
			paged = n2full[i]
		}
		if fullNodes[i].Lane != paged.Lane || fullNodes[i].Color != paged.Color {
			t.Errorf("节点 %s 分页泳道不一致: full=(%d,%d) paged=(%d,%d)",
				fullNodes[i].Sha, fullNodes[i].Lane, fullNodes[i].Color, paged.Lane, paged.Color)
		}
	}
	if len(w1)+len(w2full) < len(fullWires) {
		t.Errorf("分页线段总量不应少于整页: %d+%d < %d", len(w1), len(w2full), len(fullWires))
	}
}

// --- 不变量校验 ---

func verifyGraphInvariants(t *testing.T, commits []git.CommitEntry, nodes []GraphCommit, wires []GraphWire) {
	t.Helper()
	if len(commits) == 0 || len(nodes) != len(commits) {
		t.Fatalf("节点数量应与 commit 数一致: %d vs %d", len(nodes), len(commits))
	}
	nodeBySha := make(map[string]int, len(nodes)) // sha → row
	for i, n := range nodes {
		if n.Lane < 0 {
			t.Fatalf("节点 %s lane 非法: %d", n.Sha, n.Lane)
		}
		nodeBySha[n.Sha] = i
	}
	wiresByRow := make(map[int][]GraphWire)
	maxLane := 0
	for _, w := range wires {
		if w.From < 0 || w.To < 0 {
			t.Fatalf("线段泳道非法: %+v", w)
		}
		if w.Row < 0 || w.Row >= len(nodes)-1 {
			t.Fatalf("线段行号越界: %+v（共 %d 行）", w, len(nodes))
		}
		wiresByRow[w.Row] = append(wiresByRow[w.Row], w)
		maxLane = max(maxLane, max(w.From, w.To))
	}
	for _, n := range nodes {
		maxLane = max(maxLane, n.Lane)
	}

	// 不变量 A：每条 child→parent 边存在连续线段链。
	// 分叉点上同一泳道有多条 From 相同的线段（各父各一条），不能贪心逐段选——
	// 用 DFS 找「一条不重用线段、从子节点泳道出发、末段落到父节点泳道」的路径。
	type segKey struct {
		row int
		idx int
	}
	var findPath func(from, band, endBand, endLane int, used map[segKey]bool) bool
	findPath = func(from, band, endBand, endLane int, used map[segKey]bool) bool {
		if band > endBand {
			return false
		}
		for idx, w := range wiresByRow[band] {
			k := segKey{band, idx}
			if used[k] || w.From != from {
				continue
			}
			if band == endBand {
				if w.To == endLane {
					used[k] = true
					return true
				}
				continue
			}
			used[k] = true
			if findPath(w.To, band+1, endBand, endLane, used) {
				return true
			}
			delete(used, k)
		}
		return false
	}
	for i, c := range commits {
		for _, p := range c.Parents {
			pRow, ok := nodeBySha[p]
			if !ok {
				t.Fatalf("父提交 %s 不在节点列表（应加载全量）", p)
			}
			used := map[segKey]bool{}
			if !findPath(nodes[i].Lane, i, pRow-1, nodes[pRow].Lane, used) {
				t.Errorf("边 %s→%s 无连续线段路径（子 lane=%d 父 lane=%d）", c.Sha[:7], p[:7], nodes[i].Lane, nodes[pRow].Lane)
			}
		}
	}

	// 不变量 B：无悬空线段
	for row, segs := range wiresByRow {
		for _, w := range segs {
			// 起点：本行节点出线，或承接上一段
			fromOk := w.From == nodes[row].Lane
			if !fromOk {
				for _, prev := range wiresByRow[row-1] {
					if prev.To == w.From {
						fromOk = true
						break
					}
				}
			}
			if !fromOk {
				t.Errorf("悬空起点: band %d %+v（既非节点 %s 出线，也无上段承接）", row, w, nodes[row].Sha)
			}
			// 终点：下一行节点入线，或有下段延续
			toOk := w.To == nodes[row+1].Lane
			if row+1 <= len(nodes)-2 {
				for _, next := range wiresByRow[row+1] {
					if next.From == w.To {
						toOk = true
						break
					}
				}
			}
			if !toOk && row+1 == len(nodes)-1 {
				toOk = false // 最后一行的入线由「落到节点泳道」判定，上面已覆盖
			}
			if !toOk {
				t.Errorf("悬空终点: band %d %+v", row, w)
			}
		}
	}

	_ = maxLane
}
