package web

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"cube/util/git"
	"cube/workbench"
)

// 工作台基座（提案 docs/proposals/1010-workbench基座）。
// 本文件先落 info / refs 两个接口；tree / file / diff / file-diff / commits
// 的契约在提案中定死，由 1011-1013 各自注册实现。

type WorkbenchHandler struct {
	workbenchService *workbench.Service
}

func NewWorkbenchHandler(workbenchService *workbench.Service) *WorkbenchHandler {
	return &WorkbenchHandler{workbenchService: workbenchService}
}

func (h *WorkbenchHandler) Register(api huma.API) {
	apiGet(api, "/api/workbench/info", "获取工作台项目信息", h.info)
	apiGet(api, "/api/workbench/refs", "获取工作台分支与tag列表", h.refs)
	apiGet(api, "/api/workbench/commits", "拉取工作台 commit 图（分页）", h.commits)
	apiGet(api, "/api/workbench/status", "获取工作副本状态", h.worktreeStatus)
	apiGet(api, "/api/workbench/tree", "列出 TreeSource 下的目录树", h.tree)
	apiGet(api, "/api/workbench/file", "读取 TreeSource 下的文件内容", h.file)
	// 同 path 不同 method：显式 operationId 避免与 GET 的 workbench.file 重复
	apiRegisterOp(api, http.MethodPut, "/api/workbench/file", "保存工作副本文件（唯一写路径）", "workbench.saveFile", h.saveFile)
	apiGet(api, "/api/workbench/diff", "双 TreeSource 目录级对比", h.diff)
	apiGet(api, "/api/workbench/file-diff", "双 TreeSource 单文件 diff", h.fileDiff)
}

func (h *WorkbenchHandler) info(input struct {
	Path string `query:"path" required:"true"`
}) (*workbench.Info, error) {
	return h.workbenchService.Info(input.Path)
}

func (h *WorkbenchHandler) refs(input struct {
	Path string `query:"path" required:"true"`
}) (*workbench.Refs, error) {
	return h.workbenchService.Refs(input.Path)
}

func (h *WorkbenchHandler) commits(input struct {
	Path   string `query:"path" required:"true"`
	Scope  string `query:"scope"`  // all（默认，全部分支拓扑）| ref（单线历史）
	Ref    string `query:"ref"`    // scope=ref 时的起点 ref；空 = 当前 HEAD
	Cursor int    `query:"cursor"` // 分页 skip 偏移
	Limit  int    `query:"limit"`  // 页大小，默认 50，上限 200
}) (*workbench.CommitsPageResult, error) {
	return h.workbenchService.Commits(input.Path, input.Scope, input.Ref, input.Cursor, input.Limit)
}

func (h *WorkbenchHandler) worktreeStatus(input struct {
	Path string `query:"path" required:"true"`
	Dir  string `query:"dir"` // 工作副本目录（主目录或 worktree），空 = 仓库根
}) (*git.RepoStatus, error) {
	return h.workbenchService.WorktreeStatus(input.Path, input.Dir)
}

// 注意：huma 不展开嵌入 struct 的 query tag，参数一律平铺声明。
func (h *WorkbenchHandler) tree(input struct {
	Path        string `query:"path" required:"true"`
	SourceType  string `query:"sourceType" required:"true"`
	SourceId    string `query:"sourceId" required:"true"`
	Dir         string `query:"dir"`         // 相对该源根的子目录，空 = 根
	ShowIgnored bool   `query:"showIgnored"` // 仅 worktree 源生效
}) ([]workbench.TreeEntry, error) {
	src, err := workbench.ParseTreeSource(input.SourceType, input.SourceId)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.Tree(input.Path, src, input.Dir, input.ShowIgnored)
}

func (h *WorkbenchHandler) file(input struct {
	Path       string `query:"path" required:"true"`
	SourceType string `query:"sourceType" required:"true"`
	SourceId   string `query:"sourceId" required:"true"`
	File       string `query:"file" required:"true"`
}) (*workbench.FileResult, error) {
	src, err := workbench.ParseTreeSource(input.SourceType, input.SourceId)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.ReadFile(input.Path, src, input.File)
}

func (h *WorkbenchHandler) saveFile(input struct {
	Path       string `query:"path" required:"true"`
	SourceType string `query:"sourceType" required:"true"`
	SourceId   string `query:"sourceId" required:"true"`
	File       string `query:"file" required:"true"`
	Body       struct {
		Content string `json:"content"`
	}
}) (*workbench.FileResult, error) {
	src, err := workbench.ParseTreeSource(input.SourceType, input.SourceId)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.SaveFile(input.Path, src, input.File, input.Body.Content)
}

func (h *WorkbenchHandler) diff(input struct {
	Path          string `query:"path" required:"true"`
	LeftType      string `query:"leftType" required:"true"`
	LeftId        string `query:"leftId" required:"true"`
	RightType     string `query:"rightType" required:"true"`
	RightId       string `query:"rightId" required:"true"`
	ShowIgnored   bool   `query:"showIgnored"`
	ShowUntracked bool   `query:"showUntracked"`
	StatusFilter  string `query:"statusFilter"`
	PathPrefix    string `query:"pathPrefix"`
}) (*workbench.DiffTreesResult, error) {
	left, err := workbench.ParseTreeSource(input.LeftType, input.LeftId)
	if err != nil {
		return nil, err
	}
	right, err := workbench.ParseTreeSource(input.RightType, input.RightId)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.DiffTrees(input.Path, left, right,
		input.ShowIgnored, input.ShowUntracked, input.StatusFilter, input.PathPrefix)
}

func (h *WorkbenchHandler) fileDiff(input struct {
	Path      string `query:"path" required:"true"`
	LeftType  string `query:"leftType" required:"true"`
	LeftId    string `query:"leftId" required:"true"`
	RightType string `query:"rightType" required:"true"`
	RightId   string `query:"rightId" required:"true"`
	File      string `query:"file" required:"true"`
}) (*workbench.FileDiffResult, error) {
	left, err := workbench.ParseTreeSource(input.LeftType, input.LeftId)
	if err != nil {
		return nil, err
	}
	right, err := workbench.ParseTreeSource(input.RightType, input.RightId)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.ReadFileDiff(input.Path, left, right, input.File)
}
