package workbench

import (
	"errors"
	"fmt"
	"strings"

	"cube/util/git"
)

// --- git 面板（info / 分支与 tag / commit 日志 / 工作副本快照）---

// Info 工作台项目基本信息：入口目录规范化的仓库根 + 默认分支。
// 工作副本列表归 /worktrees 快照（含状态，刷新节奏不同）。
type Info struct {
	Root          string `json:"root"`          // 仓库根目录（path 向上探测 .git 的结果，主目录与 worktree 进来得到同一结果）
	DefaultBranch string `json:"defaultBranch"` // 远端默认分支（无 remote 时为空，可接受空值）
}

// Refs 分支与 tag 信息，全部为规范全名（refs/heads/*、refs/remotes/*、refs/tags/*）。
// 全名是写方契约：前端选中 ref 时直接整串作为 TreeSource 的 ref id 写入，零拼装；
// 展示层剥前缀（见前端 refShortName）。
type Refs struct {
	Head    string   `json:"head"`    // HEAD 指向的 ref 全名（detached 时为空）
	Locals  []string `json:"locals"`  // 本地分支（refs/heads/*）
	Remotes []string `json:"remotes"` // 远程分支（refs/remotes/*，不含各 remote 的 HEAD）
	Tags    []string `json:"tags"`    // 全部 tag（refs/tags/*）
}

// RemoteEntry 单个 remote 的展示信息：抓取地址 + 转换出的托管平台网页地址
// （ParseRepoUrl().WebUrl()，自建私服等无法识别的 host 为空串）。
type RemoteEntry struct {
	Name   string `json:"name"`
	Url    string `json:"url"`
	WebUrl string `json:"webUrl"`
}

// CommitsPageResult commit 日志一页数据（纯列表，泳道布局由前端对已持有数据计算）。
// Cursor 用 skip 偏移（依赖 git log 对同一 ref 集合的确定序），前端按 sha 去重兜底翻页边界。
type CommitsPageResult struct {
	List       []git.CommitEntry `json:"list"`
	NextCursor int               `json:"nextCursor"` // 下一页 skip 偏移；HasMore=false 时无意义
	HasMore    bool              `json:"hasMore"`    // 还有没有下一页（后端多取 1 条探测，整倍边界不误报）
}

// WorktreeStatus 单个工作副本的快照：WorktreeList 的身份字段 + LoadRepoStatus 的状态汇总。
// 是工作副本徽标与 commit 图虚拟节点（前端构造）的共同数据源；bare 副本的状态字段为零值。
type WorktreeStatus struct {
	Path      string `json:"path"`      // 工作副本绝对路径
	Head      string `json:"head"`      // 当前 HEAD commit sha（虚拟节点的挂载点）
	Branch    string `json:"branch"`    // 检出分支短名；detached/bare 为空
	Detached  bool   `json:"detached"`  // HEAD 游离
	Bare      bool   `json:"bare"`      // 裸仓库（无工作区，无未提交概念）
	Dirty     bool   `json:"dirty"`     // 任一变更（staged/unstaged/untracked）
	Ahead     int    `json:"ahead"`     // 领先上游的提交数
	Behind    int    `json:"behind"`    // 落后上游的提交数
	Staged    int    `json:"staged"`    // 暂存区变更文件数
	Unstaged  int    `json:"unstaged"`  // 工作区变更文件数（不含 untracked）
	Untracked int    `json:"untracked"` // 未跟踪文件数
}

// --- 文件树 / 文件读写 / diff（代码阅读面板与 diff 面板）---

// SourceType TreeSource 的类型：commit（历史提交）/ ref（分支或 tag）/ worktree（工作副本当前文件状态）。
// 三类目标在工作台里统一被「选中」与「对比」（见总纲提案 1008 的核心抽象）。
type SourceType string

const (
	SourceTypeCommit   SourceType = "commit"
	SourceTypeRef      SourceType = "ref"
	SourceTypeWorktree SourceType = "worktree"
)

// TreeSource 工作台统一的目标抽象，序列化（URL 参数 / API 参数）统一为 "type://id" 形态（见 String）：
//   - commit:   Id = commit sha
//   - ref:      Id = 规范全名（refs/heads/master，写方契约）或短名（master、HEAD，人类手输兜底）
//   - worktree: Id = 工作副本目录绝对路径（含未提交改动的当前状态）
type TreeSource struct {
	Type SourceType `json:"type"`
	Id   string     `json:"id"`
}

// String 序列化为 "type://id"，与 ParseTreeSource 成对；URL 参数、API 参数、前端 query key 共用此格式。
func (t TreeSource) String() string { return string(t.Type) + "://" + t.Id }

// ParseTreeSource 解析 "scheme://id" 形态的 TreeSource。只做 envelope 层校验：
// 已知 scheme 精确前缀匹配、余部原样取出——不做通用 URI 解析（不认 host、不 percent-decode，
// 编码只发生在 HTTP 层），worktree 路径里出现 "://" 也不歧义（scheme 在第一个 "://" 前已确定）。
// ref 是否存在等语义校验推迟到物化树时。
func ParseTreeSource(s string) (TreeSource, error) {
	scheme, id, ok := strings.Cut(s, "://")
	if !ok {
		return TreeSource{}, fmt.Errorf("TreeSource 缺少 scheme:// 前缀: %q（合法值 commit/ref/worktree）", s)
	}
	if id == "" {
		return TreeSource{}, errors.New("scheme 后的 id 不能为空")
	}

	typ := SourceType(scheme)
	switch typ {
	case SourceTypeCommit:
		if !isCommitSha(id) {
			return TreeSource{}, fmt.Errorf("commit id 必须是 40/64 位十六进制: %q", id)
		}
	case SourceTypeRef:
		// refs/ 开头视为规范全名原样放行（含 refs/pull/* 等开放子树）；短名按 refname 规则校验
		if !strings.HasPrefix(id, "refs/") && !isRefShortName(id) {
			return TreeSource{}, fmt.Errorf("ref 名不合法: %q（不接受 HEAD~2、@{u} 等 rev 表达式）", id)
		}
	case SourceTypeWorktree:
	default:
		return TreeSource{}, fmt.Errorf("未知的 scheme: %q（合法值 commit/ref/worktree）", scheme)
	}
	return TreeSource{Type: typ, Id: id}, nil
}

// isCommitSha 校验完整 commit sha（sha1 40 位 / sha256 64 位，大小写均可）。
func isCommitSha(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// isRefShortName 按 git check-ref-format 的核心字符规则校验 ref 短名
// （master、HEAD、origin/dev、heads/master 等各级形态）。目的是挡住 rev 表达式
// （HEAD~2、@{u}、v1.0-2-gabc）——它们是寻址语法而非 ref 名，作为选中目标
// 会随仓库演进含义漂移；完整规则由物化时的 rev-parse 兜底。
func isRefShortName(s string) bool {
	if s == "" || strings.HasPrefix(s, "/") || strings.HasPrefix(s, ".") ||
		strings.HasSuffix(s, "/") || strings.HasSuffix(s, ".") || strings.HasSuffix(s, ".lock") ||
		strings.Contains(s, "..") || strings.Contains(s, "//") || strings.Contains(s, "@{") {
		return false
	}
	for _, r := range s {
		if r <= 0x20 || r == 0x7f || strings.ContainsRune("~^:?*[\\", r) {
			return false
		}
	}
	return true
}

// TreeListResult 全量文件清单：扁平相对路径（前端用 lib/tree 组树）。
// 统一只含 git 管理的文件——worktree 源含未跟踪未忽略项，被忽略项在两种源下都不返回。
type TreeListResult struct {
	List []string `json:"list"`
}
