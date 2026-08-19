package web

import (
	"log/slog"
	"net/http"

	"strconv"

	"github.com/coder/websocket"

	"github.com/danielgtaylor/huma/v2"

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
	apiGet(api, "/api/workbench/worktrees", "全部工作副本的状态快照", h.worktrees)
	apiGet(api, "/api/workbench/tree", "列出 TreeSource 下的目录树", h.tree)
	apiGet(api, "/api/workbench/file", "读取 TreeSource 下的文件内容", h.file)
	apiPost(api, "/api/workbench/file/save", "保存工作副本文件（唯一写路径）", h.saveFile)
	apiGet(api, "/api/workbench/diff", "双 TreeSource 目录级对比", h.diff)
	apiGet(api, "/api/workbench/file-diff", "双 TreeSource 单文件 diff", h.fileDiff)
	apiGet(api, "/api/workbench/changes", "列出源相对上一版本的变更文件", h.changes)
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
	Cursor int    `query:"cursor"` // 分页 skip 偏移
	Limit  int    `query:"limit"`  // 页大小，默认 50，上限 200
}) (*workbench.CommitsPageResult, error) {
	return h.workbenchService.Commits(input.Path, input.Cursor, input.Limit)
}

func (h *WorkbenchHandler) worktrees(input struct {
	Path string `query:"path" required:"true"`
}) ([]workbench.WorktreeStatus, error) {
	return h.workbenchService.WorktreeStatuses(input.Path)
}

// 注意：huma 不展开嵌入 struct 的 query tag，参数一律平铺声明。
func (h *WorkbenchHandler) tree(input struct {
	Path       string `query:"path" required:"true"`
	SourceType string `query:"sourceType" required:"true"`
	SourceId   string `query:"sourceId" required:"true"`
}) (*workbench.TreeListResult, error) {
	src, err := workbench.ParseTreeSource(input.SourceType, input.SourceId)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.Tree(input.Path, src)
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

// saveFile 参数全部平铺在 body（POST 动作惯例，同 opener/open）；保存语义已在
// 路径名 file/save 上体现，不用 method 区分读写。
func (h *WorkbenchHandler) saveFile(input struct {
	Body struct {
		Path       string `json:"path"`
		SourceType string `json:"sourceType"`
		SourceId   string `json:"sourceId"`
		File       string `json:"file"`
		Content    string `json:"content"`
	}
}) (*workbench.FileResult, error) {
	src, err := workbench.ParseTreeSource(input.Body.SourceType, input.Body.SourceId)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.SaveFile(input.Body.Path, src, input.Body.File, input.Body.Content)
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

// RegisterRaw 注册 WebSocket 路由（upgrade 不走 huma）
func (h *WorkbenchHandler) RegisterRaw(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/workbench/pty", h.ptyWs)
}

func (h *WorkbenchHandler) ptyWs(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("path")
	cols, _ := strconv.Atoi(r.URL.Query().Get("cols"))
	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	if err := h.workbenchService.ServePty(r.Context(), conn, dir, cols, rows); err != nil {
		slog.Warn("pty 会话异常结束", "dir", dir, "err", err)
		_ = conn.Close(websocket.StatusInternalError, err.Error())
	}
}

func (h *WorkbenchHandler) changes(input struct {
	Path       string `query:"path" required:"true"`
	SourceType string `query:"sourceType" required:"true"`
	SourceId   string `query:"sourceId" required:"true"`
}) (*workbench.DiffTreesResult, error) {
	src, err := workbench.ParseTreeSource(input.SourceType, input.SourceId)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.Changes(input.Path, src)
}
