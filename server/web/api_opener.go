package web

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/danielgtaylor/huma/v2"

	"cube/opener"
	"cube/util/slicekit"
)

// --- dto ---

type OpenerDTO struct {
	Name  string   `json:"name"`
	Cmd   []string `json:"cmd"`
	Roles []string `json:"roles"`
}

func toOpenerDTO(entity *opener.Opener) *OpenerDTO {
	if entity == nil {
		return nil
	}

	return &OpenerDTO{
		Name: entity.Name(),
		Cmd:  entity.Cmd(),
		Roles: slicekit.Map(entity.Roles(), func(r opener.Role) string {
			return string(r)
		}),
	}
}

// --- handler ---

type OpenerHandler struct {
	service *opener.Service
}

func NewOpenerHandler(service *opener.Service) *OpenerHandler {
	return &OpenerHandler{
		service: service,
	}
}

func (h *OpenerHandler) Register(api huma.API) {
	apiGet(api, "/api/opener/list", "获取 opener 列表", h.openerList)
	apiGet(api, "/api/opener/info", "获取 opener 详情", h.openerInfo)
	apiPost(api, "/api/opener/open", "用指定 opener 打开任意文件或目录", h.openerOpen)
}

func (h *OpenerHandler) openerList(_ struct{}) (ListResult[*OpenerDTO], error) {
	apps := h.service.AllOpeners()
	list := slicekit.Map(apps, toOpenerDTO)
	return listResult(list), nil
}

func (h *OpenerHandler) openerInfo(input struct {
	Name string `query:"name" required:"true"`
}) (*OpenerDTO, error) {
	o := h.service.FindByName(input.Name)
	return toOpenerDTO(o), nil
}

// OpenerOpenInput open 接口入参。huma 约定：请求体字段须挂在名为 Body 的子结构上。
type OpenerOpenInput struct {
	Body struct {
		Path string `json:"path" doc:"文件或目录绝对路径"`
		App  string `json:"app" doc:"opener 名称"`
	}
}

// openerOpen 用指定 opener 打开任意文件/目录（项目打开也走这里：传项目路径即可）。
// role 按路径实际类型校验：目录须 open-dir、文件须 open-file。
func (h *OpenerHandler) openerOpen(input OpenerOpenInput) (map[string]any, error) {
	if !filepath.IsAbs(input.Body.Path) {
		return nil, errors.New("path 必须是绝对路径: " + input.Body.Path)
	}
	info, err := os.Stat(input.Body.Path)
	if err != nil {
		return nil, fmt.Errorf("读取路径失败: %w", err)
	}

	openApp := h.service.FindByName(input.Body.App)
	if openApp == nil {
		return nil, fmt.Errorf("未找到指定 app: %s", input.Body.App)
	}
	required := opener.RoleOpenFile
	if info.IsDir() {
		required = opener.RoleOpenDir
	}
	if !slices.Contains(openApp.Roles(), required) {
		return nil, fmt.Errorf("opener %s 不支持 %s（该路径需要此 role）", input.Body.App, required)
	}

	if err := openApp.Open(input.Body.Path); err != nil {
		return nil, fmt.Errorf("打开失败: %w", err)
	}
	return map[string]any{"ok": true}, nil
}
