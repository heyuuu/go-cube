package web

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"

	"cube/opener"
	"cube/util/iconkit"
	"cube/util/slicekit"
)

// --- dto ---

type OpenerDTO struct {
	Name    string         `json:"name"`
	Summary string         `json:"summary"` // 命令模板展示串
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
	apiPost(api, "/api/opener/save", "新增或更新 opener（按 name 替换）", h.openerSave)
	apiPost(api, "/api/opener/delete", "按名删除 opener", h.openerDelete)
	apiPost(api, "/api/opener/extract-icon", "从本地 .app 提取图标（64px PNG，base64）", h.openerExtractIcon)
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

// OpenerSaveInput save 接口入参（字段与 opener.Spec 对齐）。
type OpenerSaveInput struct {
	Body struct {
		Name  string         `json:"name" doc:"opener 名称（唯一标识）"`
		Cmd   []string       `json:"cmd,omitempty" doc:"启动命令，$0/$1 占位路径槽位"`
		Roles []string       `json:"roles,omitempty" doc:"业务用途枚举，缺省视为 open-dir"`
		Icon  *OpenerIconDTO `json:"icon,omitempty" doc:"图标声明"`
	}
}

func (h *OpenerHandler) openerSave(input OpenerSaveInput) (map[string]any, error) {
	spec := opener.Spec{
		Name:  input.Body.Name,
		Cmd:   input.Body.Cmd,
		Roles: input.Body.Roles,
	}
	if input.Body.Icon != nil {
		spec.Icon = &opener.Icon{Type: input.Body.Icon.Type, Value: input.Body.Icon.Value}
	}
	// 校验在 Service 写侧（领域构造函数），坏数据返回中文错误、不落文件
	if err := h.service.SaveOpener(spec); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// OpenerDeleteInput delete 接口入参。
type OpenerDeleteInput struct {
	Body struct {
		Name string `json:"name" doc:"opener 名称"`
	}
}

func (h *OpenerHandler) openerDelete(input OpenerDeleteInput) (map[string]any, error) {
	if err := h.service.DeleteOpener(input.Body.Name); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// OpenerExtractIconInput extract-icon 接口入参。
type OpenerExtractIconInput struct {
	Body struct {
		Path string `json:"path" doc:".app 目录绝对路径"`
	}
}

func (h *OpenerHandler) openerExtractIcon(input OpenerExtractIconInput) (map[string]any, error) {
	pngData, err := iconkit.ExtractAppIcon(input.Body.Path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"value": base64.StdEncoding.EncodeToString(pngData)}, nil
}
