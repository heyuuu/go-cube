package web

import (
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
