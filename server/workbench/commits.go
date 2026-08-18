package workbench

import (
	"fmt"

	"cube/util/git"
)

// CommitsPageResult commit 图一页数据（提案 1011 + 拓扑升级）。
// Cursor 用 skip 偏移（依赖 git log 对同一 ref 集合的确定序），前端按 sha 去重兜底翻页边界。
// lane/连线每次请求从第 0 行重算（无状态、跨页确定一致），只返回本页行段的 wires。
type CommitsPageResult struct {
	List       []GraphCommit `json:"list"`
	Wires      []GraphWire   `json:"wires"`      // 本页涉及的行间连线（含与上一页末行的接续段），Row 为绝对行号
	NextCursor int           `json:"nextCursor"` // 下一页 skip 偏移；HasMore=false 时无意义
	HasMore    bool          `json:"hasMore"`    // 本页拉满 limit 即认为还有更多
}

// Commits 拉取 commit 图一页。scope=all 走全部分支（--all，首屏拓扑全景），
// scope=ref 时按 ref 单线历史（大仓库首屏降级路径）。
func (s *Service) Commits(path string, scope string, ref string, cursor int, limit int) (*CommitsPageResult, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	if scope == "" {
		scope = "all"
	}
	if scope != "all" && scope != "ref" {
		return nil, fmt.Errorf("未知的 scope: %q（合法值 all/ref）", scope)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if cursor < 0 {
		cursor = 0
	}
	if scope == "ref" && ref == "" {
		// 单线模式没给 ref：退化为当前 HEAD（与 all 的区别仍是不带 --all）
		ref = "HEAD"
	}

	// 从头拉 cursor+limit 条再整体算 lane（保证跨页泳道一致），只返回本页切片
	all, err := git.CommitsPage(root, scope == "all", ref, 0, cursor+limit)
	if err != nil {
		return nil, err
	}
	nodes, wires := computeGraph(all)
	end := cursor + limit
	if end > len(nodes) {
		end = len(nodes)
	}
	page := nodes[cursor:end]

	// 本页 wires：上一页末行 → 本页首行的接续段（cursor-1 起）+ 本页内部各行段
	var pageWires []GraphWire
	fromRow := cursor - 1
	if fromRow < 0 {
		fromRow = 0
	}
	for _, w := range wires {
		if w.Row >= fromRow && w.Row < end-1 {
			pageWires = append(pageWires, w)
		}
	}

	return &CommitsPageResult{
		List:       page,
		Wires:      pageWires,
		NextCursor: cursor + limit,
		HasMore:    len(all) == cursor+limit,
	}, nil
}

// WorktreeStatus 单个工作副本的状态（提案 1011 状态区）。dir 为该工作副本目录
// （主目录或 linked worktree），不传时取仓库根。
func (s *Service) WorktreeStatus(path string, dir string) (*git.RepoStatus, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	if dir == "" {
		dir = root
	}
	st, err := git.LoadRepoStatus(dir)
	if err != nil {
		return nil, fmt.Errorf("读取工作副本状态失败: dir=%s: %w", dir, err)
	}
	return st, nil
}
