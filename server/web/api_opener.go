package web

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"

	"cube/opener"
	"cube/util/slicekit"
)

// --- dto ---

type OpenerDTO struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`    // exec | web；web 时前端直接路由跳转，不发 open
	Summary string         `json:"summary"` // 展示串：exec 为命令模板，web 为 target
	Roles   []string       `json:"roles"`
	Icon    *OpenerIconDTO `json:"icon,omitempty"` // 缺省无图标（前端 fallback 默认）
}

type OpenerIconDTO struct {
	Type  string `json:"type"`  // lucide | image
	Value string `json:"value"` // lucide 图名 或 base64 PNG
}

func toOpenerDTO(entity opener.Opener) *OpenerDTO {
	if entity == nil {
		return nil
	}

	var icon *OpenerIconDTO
	if i := entity.Icon(); i.Type != "" {
		icon = &OpenerIconDTO{Type: i.Type, Value: i.Value}
	}
	return &OpenerDTO{
		Name:    entity.Name(),
		Type:    entity.Kind(),
		Summary: entity.Summary(),
		Roles: slicekit.Map(entity.Roles(), func(r opener.Role) string {
			return string(r)
		}),
		Icon: icon,
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
	openers := h.service.AllOpeners()
	list := slicekit.Map(openers, toOpenerDTO)
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
		Path   string `json:"path" doc:"文件或目录绝对路径"`
		Opener string `json:"opener" doc:"opener 名称"`
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

	o := h.service.FindByName(input.Body.Opener)
	if o == nil {
		return nil, fmt.Errorf("未找到指定 opener: %s", input.Body.Opener)
	}
	role := opener.RoleOpenFile
	if info.IsDir() {
		role = opener.RoleOpenDir
	}

	// role 是否支持由实现内校验（Open 首步），这里只透传错误
	if err := o.Open(role, input.Body.Path); err != nil {
		return nil, fmt.Errorf("打开失败: %w", err)
	}
	return map[string]any{"ok": true}, nil
}
