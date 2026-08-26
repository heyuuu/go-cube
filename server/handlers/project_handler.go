package handlers

import (
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"cube/project"
	"cube/project/gitcache"
	"cube/util/slicekit"
	"cube/web"
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
}

func NewProjectHandler(projectService *project.Service) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
	}
}

func (h *ProjectHandler) Register(api huma.API, mux *http.ServeMux) {
	web.ApiGet(api, "/api/project/list", "获取项目列表", h.projectList)
	web.ApiGet(api, "/api/project/info", "获取项目详情", h.projectInfo)
	web.ApiGet(api, "/api/project/scan-rules", "获取扫描规则", h.scanRules)
	web.ApiGet(api, "/api/project/clone-rules", "获取 clone 规则", h.cloneRules)
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

func (h *ProjectHandler) scanRules(_ struct{}) (web.ListResult[project.ScanRule], error) {
	rules := h.projectService.ScanRules()
	return listResult(rules), nil
}

func (h *ProjectHandler) cloneRules(_ struct{}) (web.ListResult[project.CloneRule], error) {
	rules := h.projectService.CloneRules()
	return listResult(rules), nil
}
