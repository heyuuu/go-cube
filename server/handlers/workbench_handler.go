package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/coder/websocket"

	"github.com/danielgtaylor/huma/v2"

	"cube/web"
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

func (h *WorkbenchHandler) Register(api huma.API, mux *http.ServeMux) {
	web.ApiGet(api, "/api/workbench/info", "获取工作台仓库信息", h.info)
	web.ApiGet(api, "/api/workbench/refs", "获取工作台分支与tag列表", h.refs)
	web.ApiGet(api, "/api/workbench/remotes", "获取工作台 remote 列表", h.remotes)
	web.ApiGet(api, "/api/workbench/commits", "拉取工作台 commit 图（分页）", h.commits)
	web.ApiGet(api, "/api/workbench/worktrees", "全部工作副本的状态快照", h.worktrees)
	web.ApiGet(api, "/api/workbench/tree", "列出 TreeSource 下的目录树", h.tree)
	web.ApiGet(api, "/api/workbench/file", "读取 TreeSource 下的文件内容", h.file)
	web.ApiPost(api, "/api/workbench/file/save", "保存工作副本文件（唯一写路径）", h.saveFile)
	web.ApiGet(api, "/api/workbench/diff", "双 TreeSource 目录级对比", h.diff)
	web.ApiGet(api, "/api/workbench/file-diff", "双 TreeSource 单文件 diff", h.fileDiff)
	web.ApiGet(api, "/api/workbench/changes", "列出源相对上一版本的变更文件", h.changes)
	web.ApiPost(api, "/api/workbench/worktree/add", "新增 worktree", h.worktreeAdd)
	web.ApiPost(api, "/api/workbench/worktree/remove", "删除 worktree（非 force 预检拒绝返回 denied+reasons）", h.worktreeRemove)
	web.ApiPost(api, "/api/workbench/branch/delete", "删除本地分支", h.branchDelete)

	// 注册 WebSocket 路由（upgrade 不走 huma）
	mux.HandleFunc("GET /api/workbench/pty", h.ptyWs)
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

func (h *WorkbenchHandler) remotes(input struct {
	Path string `query:"path" required:"true"`
}) ([]workbench.RemoteEntry, error) {
	return h.workbenchService.Remotes(input.Path)
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
// source/base/current 均为 "type://id" 形态（workbench.ParseTreeSource 定义文法）。
func (h *WorkbenchHandler) tree(input struct {
	Path   string `query:"path" required:"true"`
	Source string `query:"source" required:"true"`
}) (*workbench.TreeListResult, error) {
	src, err := workbench.ParseTreeSource(input.Source)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.Tree(input.Path, src)
}

func (h *WorkbenchHandler) file(input struct {
	Path   string `query:"path" required:"true"`
	Source string `query:"source" required:"true"`
	File   string `query:"file" required:"true"`
}) (*workbench.FileResult, error) {
	src, err := workbench.ParseTreeSource(input.Source)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.ReadFile(input.Path, src, input.File)
}

// saveFile 参数全部平铺在 body（POST 动作惯例，同 opener/open）；保存语义已在
// 路径名 file/save 上体现，不用 method 区分读写。
func (h *WorkbenchHandler) saveFile(input struct {
	Body struct {
		Path    string `json:"path"`
		Source  string `json:"source"`
		File    string `json:"file"`
		Content string `json:"content"`
	}
}) (*workbench.FileResult, error) {
	src, err := workbench.ParseTreeSource(input.Body.Source)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.SaveFile(input.Body.Path, src, input.Body.File, input.Body.Content)
}

func (h *WorkbenchHandler) diff(input struct {
	Path    string `query:"path" required:"true"`
	Base    string `query:"base" required:"true"`
	Current string `query:"current" required:"true"`
}) (*workbench.DiffTreesResult, error) {
	base, err := workbench.ParseTreeSource(input.Base)
	if err != nil {
		return nil, err
	}
	current, err := workbench.ParseTreeSource(input.Current)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.DiffTrees(input.Path, base, current)
}

func (h *WorkbenchHandler) fileDiff(input struct {
	Path     string `query:"path" required:"true"`
	Base     string `query:"base"` // 缺省 = 相对基准（worktree vs HEAD、ref/commit vs 父提交）
	Current  string `query:"current" required:"true"`
	File     string `query:"file" required:"true"`
	BaseFile string `query:"baseFile"` // 基准侧路径（rename 条目与当前侧不同；空则同 file）
}) (*workbench.FileDiffResult, error) {
	var base workbench.TreeSource
	if input.Base != "" {
		parsed, err := workbench.ParseTreeSource(input.Base)
		if err != nil {
			return nil, err
		}
		base = parsed
	}
	current, err := workbench.ParseTreeSource(input.Current)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.ReadFileDiff(input.Path, base, current, input.File, input.BaseFile)
}

// worktreeAdd 参数平铺在 body（POST 动作惯例）。branch 支持规范全名或短名；
// targetPath 为空时服务端按 <repoName>.worktrees/<分支名>/ 预填。
func (h *WorkbenchHandler) worktreeAdd(input struct {
	Body struct {
		Path       string `json:"path"`
		Branch     string `json:"branch,omitempty"`     // 留空 = detached
		Commitish  string `json:"commitish,omitempty"`  // 基点（commit/分支/tag），留空 = HEAD
		TargetPath string `json:"targetPath,omitempty"` // 目标目录，留空 = 服务端预填
	}
}) (*workbench.WorktreeCreated, error) {
	return h.workbenchService.WorktreeAdd(input.Body.Path, input.Body.Branch, input.Body.Commitish, input.Body.TargetPath)
}

// worktreeRemoveResult 非 force 预检拒绝时 denied=true，reasons 供 UI 二次确认
// 升级 force；denied=false 表示已删除。
type worktreeRemoveResult struct {
	Denied  bool     `json:"denied"`
	Reasons []string `json:"reasons"`
}

func (h *WorkbenchHandler) worktreeRemove(input struct {
	Body struct {
		Path       string `json:"path"`
		TargetPath string `json:"targetPath"`
		Force      bool   `json:"force,omitempty"`
	}
}) (*worktreeRemoveResult, error) {
	err := h.workbenchService.WorktreeRemove(input.Body.Path, input.Body.TargetPath, input.Body.Force)
	var denied *workbench.WorktreeRemoveDenied
	if errors.As(err, &denied) {
		return &worktreeRemoveResult{Denied: true, Reasons: denied.Reasons}, nil
	}
	if err != nil {
		return nil, err
	}
	return &worktreeRemoveResult{}, nil
}

func (h *WorkbenchHandler) branchDelete(input struct {
	Body struct {
		Path   string `json:"path"`
		Branch string `json:"branch"`
		Force  bool   `json:"force,omitempty"`
	}
}) (map[string]any, error) {
	if err := h.workbenchService.BranchDelete(input.Body.Path, input.Body.Branch, input.Body.Force); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
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
	Path   string `query:"path" required:"true"`
	Source string `query:"source" required:"true"`
}) (*workbench.DiffTreesResult, error) {
	src, err := workbench.ParseTreeSource(input.Source)
	if err != nil {
		return nil, err
	}
	return h.workbenchService.Changes(input.Path, src)
}
