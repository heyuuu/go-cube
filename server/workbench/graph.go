package workbench

// commit graph 的 lane 布局（提案 1011 的拓扑升级；选型结论见 zcode-topics
// 260730 调研：后端产出带 lane 坐标的节点 + 行间连线，前端 SVG 纯渲染）。
//
// 算法：经典 active-lanes 扫描（pvigier / DoltHub 博客的做法）。
// 按拓扑序遍历 commit（API 层用 --topo-order 保证）：
//   - lanes 是「等待出现的 commit sha」占位列表，索引即泳道；
//   - 当前 commit 出现在某泳道 → 节点画在该泳道并腾出位置；
//     不在任何泳道（根提交等）→ 右侧开新泳道；
//   - 首父占据原泳道（直线延续、继承泳道色）；首父已被其它子提交占位则
//     合并过去、原泳道删除（列压缩，右侧泳道左移）；
//   - 其余父各开新泳道（合并曲线的来源）。
// 连线推导（第二遍，纯查表）：第 i 行到 i+1 行的线段 = lanesAfter[i] 的每个
// 条目映射到 lanesAfter[i+1] 中的新位置；条目正是 i+1 行的 commit 时映射到
// 该节点的泳道（线进入节点）。分段表达使前端无需建模跨行整线。

import "cube/util/git"

// GraphWire 一行到下一行的一条连线段。From/To 为两端泳道索引，Color 为调色板索引。
type GraphWire struct {
	Row   int `json:"row"`   // 段的起始行（连到 row+1）
	From  int `json:"from"`  // 起始泳道
	To    int `json:"to"`    // 结束泳道
	Color int `json:"color"` // 调色板索引（前端映射具体颜色）
}

// GraphCommit 节点 = git.CommitEntry + 泳道坐标（row = 在结果数组中的下标）。
type GraphCommit struct {
	git.CommitEntry
	Lane  int `json:"lane"`  // 节点所在泳道（列）
	Color int `json:"color"` // 调色板索引
}

type graphLane struct {
	sha   string
	color int
}

func computeGraph(commits []git.CommitEntry) (nodes []GraphCommit, wires []GraphWire) {
	nodes = make([]GraphCommit, len(commits))
	// 每行处理完的 lanes 快照：既用于下一行定位，也用于第二遍连线推导
	snapshots := make([][]graphLane, len(commits))

	var lanes []graphLane
	laneIndexOf := func(sha string) int {
		for i, l := range lanes {
			if l.sha == sha {
				return i
			}
		}
		return -1
	}

	colorSeq := 0
	for i, c := range commits {
		lane := laneIndexOf(c.Sha)
		if lane < 0 {
			colorSeq++
			lanes = append(lanes, graphLane{sha: c.Sha, color: colorSeq})
			lane = len(lanes) - 1
		}
		color := lanes[lane].color
		nodes[i] = GraphCommit{CommitEntry: c, Lane: lane, Color: color}

		if len(c.Parents) > 0 {
			if laneIndexOf(c.Parents[0]) >= 0 {
				// 首父已被其它子提交占位 → 合并到那条泳道，当前泳道删除
				lanes = append(lanes[:lane], lanes[lane+1:]...)
			} else {
				lanes[lane] = graphLane{sha: c.Parents[0], color: color}
			}
			// 其余父各开新泳道（若未占位）
			for _, p := range c.Parents[1:] {
				if laneIndexOf(p) < 0 {
					colorSeq++
					lanes = append(lanes, graphLane{sha: p, color: colorSeq})
				}
			}
		} else {
			lanes = append(lanes[:lane], lanes[lane+1:]...)
		}

		snapshots[i] = make([]graphLane, len(lanes))
		copy(snapshots[i], lanes)
	}

	// 连线分两类（行 i → i+1）：
	//  1. 快照条目线：snapshots[i] 的每个条目落到 snapshots[i+1]（或 i+1 行节点）的新位置；
	//     条目正是 i+1 行的 commit 时，线进入该节点。
	//  2. 节点合并线：行 i 节点的首父已占其它泳道时，该节点自身的泳道被删除、
	//     不再出现在快照里——需要补一段从节点到首父位置的折线（典型合并拐弯）。
	for i := 0; i+1 < len(commits); i++ {
		next := commits[i+1]
		for from, entry := range snapshots[i] {
			to := -1
			if entry.sha == next.Sha {
				to = nodes[i+1].Lane
			} else {
				for j, l := range snapshots[i+1] {
					if l.sha == entry.sha {
						to = j
						break
					}
				}
			}
			if to >= 0 {
				wires = append(wires, GraphWire{Row: i, From: from, To: to, Color: entry.color})
			}
		}

		// 补节点合并线（存在快照没覆盖的「节点 → 首父」连线时）
		if parents := commits[i].Parents; len(parents) > 0 {
			parentPos := -1
			for j, l := range snapshots[i] {
				if l.sha == parents[0] {
					parentPos = j
					break
				}
			}
			covered := parentPos == nodes[i].Lane // 首父占位恰为节点泳道 = 首类线已覆盖
			if parentPos >= 0 && !covered {
				wires = append(wires, GraphWire{Row: i, From: nodes[i].Lane, To: parentPos, Color: nodes[i].Color})
			}
		}
	}

	return nodes, wires
}
