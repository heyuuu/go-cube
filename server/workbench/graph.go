package workbench

// commit graph 的 lane 布局（提案 1011 的拓扑升级；选型结论见 zcode-topics
// 260730 调研：后端产出带 lane 坐标的节点 + 行间连线，前端 SVG 纯渲染）。
//
// 算法：经典 active-lanes 扫描（pvigier / DoltHub 博客的做法）。
// 按拓扑序遍历 commit（API 层用 --topo-order 保证）：
//   - lanes 是「等待出现的 commit sha」占位列表，索引即泳道；
//   - 当前 commit 出现在某泳道 → 节点画在该泳道并腾出位置；
//     不在任何泳道（根提交等）→ 复用最靠左的空洞，无洞则右侧新开；
//   - 首父占据原泳道（直线延续、继承泳道色）；首父已被其它子提交占位则
//     合并过去、原泳道留洞；其余父各占新位（合并曲线的来源）；
//   - **关键：泳道不即时压缩**——空位留洞，只裁剪尾部连续空洞。
//     即时删除会让右侧泳道整体左移，支线被迫在错误的行提前拐弯
//     （叉出/汇入位置偏一行）；留洞保证位置稳定，拐弯只出现在
//     真正 fork/merge 的段上。
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
	sha   string // 空 sha = 空洞（已腾出待复用的泳道位）
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
	// newLane：新支线复用最靠左的空洞，没有空洞才在右侧展开（控制总宽度）
	newLane := func(entry graphLane) int {
		for i, l := range lanes {
			if l.sha == "" {
				lanes[i] = entry
				return i
			}
		}
		lanes = append(lanes, entry)
		return len(lanes) - 1
	}
	// trimTrailingHoles：裁掉尾部连续空洞（不移动任何占用泳道的位置）
	trimTrailingHoles := func() {
		for len(lanes) > 0 && lanes[len(lanes)-1].sha == "" {
			lanes = lanes[:len(lanes)-1]
		}
	}

	// fpChildren[p] = 把 p 作为首父的子提交行号列表。
	// 首父认领泳道时做「低泳道优先」：若本节点的首父还有其它首父子提交
	// 正等待在更靠左（更靠近主线）的泳道上，本节点不认领、让位给那条泳道，
	// 自身按合并处理——否则后切出的支线会把母分支的祖先链拖到右侧泳道，
	// 母分支反而画成拐弯的侧枝（嵌套分支场景的主视觉 bug）。
	fpChildren := make(map[string][]int)
	for i, c := range commits {
		if len(c.Parents) > 0 {
			fpChildren[c.Parents[0]] = append(fpChildren[c.Parents[0]], i)
		}
	}

	deferred := make(map[int]int) // 行号 → 让位目标泳道（出线拐弯的落点）
	colorSeq := 0
	for i, c := range commits {
		lane := laneIndexOf(c.Sha)
		if lane < 0 {
			colorSeq++
			lane = newLane(graphLane{sha: c.Sha, color: colorSeq})
		}
		color := lanes[lane].color
		nodes[i] = GraphCommit{CommitEntry: c, Lane: lane, Color: color}

		if len(c.Parents) > 0 {
			// 让位判定：存在更靠左的首父兄弟等待条目 → 让出本泳道，出线拐向兄弟泳道
			deferTo := -1
			for _, j := range fpChildren[c.Parents[0]] {
				if j == i {
					continue
				}
				if jl := laneIndexOf(commits[j].Sha); jl >= 0 && jl < lane && (deferTo < 0 || jl < deferTo) {
					deferTo = jl
				}
			}
			if deferTo >= 0 || laneIndexOf(c.Parents[0]) >= 0 {
				// 首父已被其它子提交占位（或让位）→ 合并过去，原泳道留洞（不左移）
				lanes[lane] = graphLane{}
				if deferTo >= 0 {
					deferred[i] = deferTo
				}
			} else {
				lanes[lane] = graphLane{sha: c.Parents[0], color: color}
			}
			// 其余父各占新位（若未占位）
			for _, p := range c.Parents[1:] {
				if laneIndexOf(p) < 0 {
					colorSeq++
					newLane(graphLane{sha: p, color: colorSeq})
				}
			}
		} else {
			lanes[lane] = graphLane{}
		}
		trimTrailingHoles()

		snapshots[i] = make([]graphLane, len(lanes))
		copy(snapshots[i], lanes)
	}

	// 连线（行 i → i+1）。每段两类线，按条目归属判断：
	//   节点出线：条目是本行 commit 的父提交 → 从节点泳道出发（首父同位时即竖线，
	//             额外父/合并占位时为拐弯曲线）；
	//   穿越线：条目在上一行快照已存在（上方有子提交的支线路过本段）→ 从自身位置出发；
	//           泳道压缩时 to 落在新位置，表现为折线。
	// 两类可并存（同一父被多条支线共享时，节点出线 + 穿越线同时存在）；
	// 首父接管原泳道时二者重合，只发一条。
	for i := 0; i+1 < len(commits); i++ {
		next := commits[i+1]
		parentSet := make(map[string]bool, len(commits[i].Parents))
		for _, p := range commits[i].Parents {
			parentSet[p] = true
		}
		target := func(entry graphLane) int {
			if entry.sha == next.Sha {
				return nodes[i+1].Lane
			}
			for j, l := range snapshots[i+1] {
				if l.sha == entry.sha {
					return j
				}
			}
			return -1
		}
		for pos, entry := range snapshots[i] {
			if entry.sha == "" {
				continue // 空洞
			}
			to := target(entry)
			if to < 0 {
				continue
			}
			isParent := parentSet[entry.sha]
			// 上方是否有来线：条目由更早的行放置（首行上方无快照，必为 false）
			incoming := func() bool {
				if i == 0 {
					return false
				}
				for _, l := range snapshots[i-1] {
					if l.sha == entry.sha {
						return true
					}
				}
				return false
			}()
			// 节点出线（父提交）。颜色约定：本行新开的支线（分叉）用新泳道色；
			// 已存在的支线汇入主线（合并）用子节点自己的颜色——即支线全程同色，
			// 只有真正的主线竖线用主线色
			if isParent {
				outColor := entry.color
				if incoming {
					outColor = nodes[i].Color
				}
				wires = append(wires, GraphWire{Row: i, From: nodes[i].Lane, To: to, Color: outColor})
			}
			// 穿越线（上方支线延续），首父接管原泳道时与节点出线重合，跳过
			if incoming && !(isParent && pos == nodes[i].Lane) {
				wires = append(wires, GraphWire{Row: i, From: pos, To: to, Color: entry.color})
			}
		}
	}

	return nodes, wires
}
