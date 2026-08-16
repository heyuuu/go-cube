package web

import (
	"errors"
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"cube/opener"
	"cube/project"
	"cube/project/gitcache"
	"cube/util/slicekit"
)

// --- dto ---
type ProjectDTO struct {
	Name    string          `json:"name"`
	Group   string          `json:"group"`
	Path    string          `json:"path"`
	Tags    []string        `json:"tags"`
	GitInfo *gitcache.Entry `json:"gitInfo"` // git 状态快照，可能为 nil（未采集）
}

// ProjectListResult 列表接口返回结构：含项目列表 + 两类刷新时间。
type ProjectListResult struct {
	List          []*ProjectDTO `json:"list"`
	ScanUpdatedAt time.Time     `json:"scanUpdatedAt"` // 项目列表最近一次重扫完成时间（零值=未刷新过）
	GitUpdatedAt  time.Time     `json:"gitUpdatedAt"`  // git 缓存最近一次落盘时间（零值=无缓存）
}

// --- handler ---

type ProjectHandler struct {
	projectService *project.Service
	openerService  *opener.Service
}

func NewProjectHandler(projectService *project.Service, openerService *opener.Service) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
		openerService:  openerService,
	}
}

func (h *ProjectHandler) Register(api huma.API) {
	apiGet(api, "/api/project/list", "获取项目列表", h.projectList)
	apiGet(api, "/api/project/info", "获取项目详情", h.projectInfo)
	apiGet(api, "/api/project/scan-rules", "获取扫描规则", h.scanRules)
	apiGet(api, "/api/project/clone-rules", "获取 clone 规则", h.cloneRules)
	apiGet(api, "/api/project/tree", "获取项目目录树", h.projectTree)
	apiPost(api, "/api/project/open", "用指定 opener 打开项目", h.projectOpen)
}

func (h *ProjectHandler) projectList(_ struct{}) (ProjectListResult, error) {
	projects := h.projectService.Projects()
	list := slicekit.Map(projects, h.toProjectDTO)
	return ProjectListResult{
		List:          list,
		ScanUpdatedAt: h.projectService.ScanUpdatedAt(),
		GitUpdatedAt:  h.projectService.GitUpdatedAt(),
	}, nil
}

// ProjectInfoResult 详情接口返回结构：含项目 DTO + 两类刷新时间。
type ProjectInfoResult struct {
	Project       *ProjectDTO `json:"project"`
	ScanUpdatedAt time.Time   `json:"scanUpdatedAt"`
	GitUpdatedAt  time.Time   `json:"gitUpdatedAt"`
}

func (h *ProjectHandler) projectInfo(input struct {
	Name string `query:"name" required:"true"`
}) (ProjectInfoResult, error) {
	proj := h.projectService.FindByName(input.Name)
	return ProjectInfoResult{
		Project:       h.toProjectDTO(proj),
		ScanUpdatedAt: h.projectService.ScanUpdatedAt(),
		GitUpdatedAt:  h.projectService.GitUpdatedAt(),
	}, nil
}

func (h *ProjectHandler) toProjectDTO(entity *project.Project) *ProjectDTO {
	if entity == nil {
		return nil
	}

	info, _ := h.projectService.GitInfo(entity.Path())
	return &ProjectDTO{
		Name:    entity.Name(),
		Group:   entity.Group(),
		Path:    entity.Path(),
		Tags:    entity.Tags(),
		GitInfo: info,
	}
}

func (h *ProjectHandler) scanRules(_ struct{}) (ListResult[project.ScanRule], error) {
	rules := h.projectService.ScanRules()
	return listResult(rules), nil
}

func (h *ProjectHandler) cloneRules(_ struct{}) (ListResult[project.CloneRule], error) {
	rules := h.projectService.CloneRules()
	return listResult(rules), nil
}

// TreeNodeDTO 树节点 web 输出。把 Style 枚举转成字符串便于前端判断。
type TreeNodeDTO struct {
	Name     string        `json:"name"`
	Path     string        `json:"path"`
	Kind     string        `json:"kind"` // dir（含项目的目录）/ project（项目）/ other（普通目录）
	Children []TreeNodeDTO `json:"children"`
}

func toTreeNodeDTO(n project.TreeNode) TreeNodeDTO {
	kind := "other"
	switch n.Style {
	case project.TreeNodeStyleProject:
		kind = "project"
	case project.TreeNodeStyleDir:
		kind = "dir"
	}
	children := make([]TreeNodeDTO, 0, len(n.Children))
	for _, c := range n.Children {
		children = append(children, toTreeNodeDTO(c))
	}
	return TreeNodeDTO{Name: n.Name, Path: n.Path, Kind: kind, Children: children}
}

// ProjectTreeInput tree 接口入参：root 可选（空 = 项目公共前缀）。
type ProjectTreeInput struct {
	Root string `query:"root"`
}

func (h *ProjectHandler) projectTree(input ProjectTreeInput) (TreeNodeDTO, error) {
	root, err := h.projectService.BuildTree(input.Root)
	if err != nil {
		return TreeNodeDTO{}, err
	}
	return toTreeNodeDTO(root), nil
}

// ProjectOpenInput open 接口入参。huma 约定：请求体字段须挂在名为 Body 的子结构上。
type ProjectOpenInput struct {
	Body struct {
		Path string `json:"path" doc:"项目绝对路径"`
		App  string `json:"app" doc:"opener 名称（finder / vscode / idea ...）"`
	}
}

func (h *ProjectHandler) projectOpen(input ProjectOpenInput) (map[string]any, error) {
	// 校验 opener
	openApp := h.openerService.FindByName(input.Body.App)
	if openApp == nil {
		return nil, fmt.Errorf("未找到指定 app: %s", input.Body.App)
	}

	// 校验项目
	proj := h.projectService.FindByPath(input.Body.Path)
	if proj == nil {
		return nil, errors.New("未找到指定项目: " + input.Body.Path)
	}

	// 打开（opener 是 fire-and-forget，启动子进程后立即返回）
	if err := openApp.Open(proj.Path()); err != nil {
		return nil, fmt.Errorf("打开失败: %w", err)
	}

	return map[string]any{"ok": true}, nil
}
