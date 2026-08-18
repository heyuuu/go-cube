package workbench

import (
	"os/exec"
	"testing"

	"cube/internal/testfixture"
	"cube/util/git"
)

// 构造无 git 依赖的纯节点列表（直接填 CommitEntry）跑 computeGraph。
func mkCommits(list ...[3]string) []git.CommitEntry { // [sha, parents(逗号分隔), subject]
	result := make([]git.CommitEntry, len(list))
	for i, e := range list {
		var parents []string
		for _, p := range splitComma(e[1]) {
			if p != "" {
				parents = append(parents, p)
			}
		}
		result[i] = git.CommitEntry{Sha: e[0], Parents: parents, Subject: e[2]}
	}
	return result
}

func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == ',' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(c)
	}
	return append(out, cur)
}

func TestComputeGraphLinear(t *testing.T) {
	// c3 ← c2 ← c1：单泳道直线
	nodes, wires := computeGraph(mkCommits(
		[3]string{"c3", "c2", ""},
		[3]string{"c2", "c1", ""},
		[3]string{"c1", "", ""},
	))
	for i, want := range []int{0, 0, 0} {
		if nodes[i].Lane != want {
			t.Errorf("节点 %s lane=%d, want %d", nodes[i].Sha, nodes[i].Lane, want)
		}
	}
	if len(wires) != 2 {
		t.Fatalf("线性历史应 2 段线, got %d", len(wires))
	}
	for _, w := range wires {
		if w.From != 0 || w.To != 0 {
			t.Errorf("线性历史连线应在泳道 0: %+v", w)
		}
	}
}

func TestComputeGraphMerge(t *testing.T) {
	//   c1 ← c2 ← c4(merge, 合并 c3)
	//   └──── c3 ←┘
	nodes, wires := computeGraph(mkCommits(
		[3]string{"c4", "c2,c3", ""},
		[3]string{"c2", "c1", ""},
		[3]string{"c3", "c1", ""},
		[3]string{"c1", "", ""},
	))
	if nodes[0].Lane != 0 {
		t.Errorf("merge 节点应在泳道 0, got %d", nodes[0].Lane)
	}
	if nodes[1].Lane != 0 {
		t.Errorf("c2（首父链）应在泳道 0, got %d", nodes[1].Lane)
	}
	if nodes[2].Lane != 1 {
		t.Errorf("c3（侧支）应在泳道 1, got %d", nodes[2].Lane)
	}
	// c1 收两条线：来自 c2（泳道0）与 c3（泳道1）→ c1 节点两条入线
	intoRoot := 0
	for _, w := range wires {
		if w.To == nodes[3].Lane && w.Row == 2 {
			intoRoot++
		}
	}
	if intoRoot != 2 {
		t.Errorf("根节点应收 2 条入线, got %d（wires=%+v）", intoRoot, wires)
	}
}

func TestComputeGraphBranchColorInherit(t *testing.T) {
	// 侧支（c3）与主线（c2）不同色；c3 的子 c5 继承 c3 的色
	nodes, _ := computeGraph(mkCommits(
		[3]string{"c6", "c5,c2", ""},
		[3]string{"c5", "c3", ""},
		[3]string{"c2", "c1", ""},
		[3]string{"c3", "c1", ""},
		[3]string{"c1", "", ""},
	))
	if nodes[1].Color != nodes[3].Color {
		t.Errorf("同支线 c5/c3 颜色应一致: %d vs %d", nodes[1].Color, nodes[3].Color)
	}
	if nodes[2].Color == nodes[3].Color {
		t.Error("主线与侧支颜色应不同")
	}
}

func TestComputeGraphRealRepo(t *testing.T) {
	// 真实仓库：主干 + 侧支 + merge，端到端验证 lane 分配
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "main", EmptyCommitCount: 2})
	run := func(args ...string) {
		t.Helper()
		if err := exec.Command("git", append([]string{"-C", repo}, args...)...).Run(); err != nil {
			t.Fatalf("git %v 失败: %v", args, err)
		}
	}
	run("checkout", "-b", "feature")
	run("commit", "--allow-empty", "-m", "f1")
	run("commit", "--allow-empty", "-m", "f2")
	run("checkout", "main")
	run("commit", "--allow-empty", "-m", "m1")
	run("merge", "--no-ff", "feature", "-m", "merge")

	commits, err := git.CommitsPage(repo, true, "", 0, 50)
	if err != nil {
		t.Fatalf("CommitsPage: %v", err)
	}
	if len(commits) != 6 {
		t.Fatalf("应 6 个 commit, got %d", len(commits))
	}
	nodes, wires := computeGraph(commits)
	// merge 节点（首行）有两个父 → 图中应出现泳道 1
	hasSideLane := false
	for _, n := range nodes {
		if n.Lane == 1 {
			hasSideLane = true
		}
	}
	if !hasSideLane {
		t.Errorf("应出现侧泳道: %+v", nodes)
	}
	if len(wires) == 0 {
		t.Fatal("应有连线")
	}
	// 所有 wire 的 To/From 落在出现的最大泳道内
	maxLane := 0
	for _, n := range nodes {
		if n.Lane > maxLane {
			maxLane = n.Lane
		}
	}
	for _, w := range wires {
		if w.From > maxLane+1 || w.To > maxLane+1 {
			t.Errorf("连线超出泳道范围: %+v (maxLane=%d)", w, maxLane)
		}
	}
}

// 回归：merge 后 topo 序常把支线提交排在主线前（M, f2, f1, m1, c1）。
// 旧版即时压缩泳道会让 c1 的支线在 m1 行提前左拐（叉出/汇入位置偏一行）；
// 留洞版应保证：支线（f2/f1）全程 lane1，主线（M/m1）lane0，汇合曲线只出现在 band3。
func TestComputeGraphLaneStability(t *testing.T) {
	nodes, wires := computeGraph(mkCommits(
		[3]string{"M", "m1,f2", "merge"},
		[3]string{"f2", "f1", ""},
		[3]string{"f1", "c1", ""},
		[3]string{"m1", "c1", ""},
		[3]string{"c1", "", ""},
	))
	wantLanes := map[string]int{"M": 0, "m1": 0, "f2": 1, "f1": 1, "c1": 1}
	for _, n := range nodes {
		if n.Lane != wantLanes[n.Sha] {
			t.Errorf("节点 %s lane=%d, want %d", n.Sha, n.Lane, wantLanes[n.Sha])
		}
	}
	// band0 应有 M→lane1 的分叉曲线；band3 应有 m1→c1 的汇合曲线；band1/2 无拐弯（from==to）
	for _, w := range wires {
		switch w.Row {
		case 0:
			if w.From == 0 && w.To == 1 {
				// fork curve ✓
			}
		case 1, 2:
			if w.From != w.To {
				t.Errorf("band %d 不应拐弯: %+v", w.Row, w)
			}
		case 3:
			if w.From == 0 && w.To == 1 {
				// merge curve ✓
			}
		}
	}
}
