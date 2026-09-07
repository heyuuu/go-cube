package usage

import "time"

// Record 一条项目使用记录 = 一次 opener 打开事件。
// 行序与 time 单调一致，承担原 sqlite 自增 id 的排序职责。
// 字段演进约束：只增不改（新字段加 omitempty），读取方必须容忍缺失字段。
type Record struct {
	Time    time.Time `json:"time"`
	Project string    `json:"project"`          // 项目绝对路径（project 的唯一标识，排序 join 用它）
	Opener  string    `json:"opener,omitempty"` // opener 名（settings openers 节的 key）
	Dir     string    `json:"dir,omitempty"`    // 实际打开的目标目录绝对路径（主项目根打开时省略；worktree / monorepo workspace 子目录时为其绝对路径，见 1032/1030）
}

// PathUsage 去重后的一条最近路径（RecentPaths 输出条目）。
type PathUsage struct {
	Path string    `json:"path"`
	Time time.Time `json:"time"` // 该路径最近一次使用时间
}
