package workbench

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
