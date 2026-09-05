package handlers

import (
	"encoding/base64"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"cube/util/iconkit"
	"cube/web"
)

// IconHandler icon 声明的通用辅助出口（提取图标）。icon 语义属 util/iconkit
// 跨域共享（opener / scanRules / forge 的 IconField 共用此端点），不挂任何单一 domain。
type IconHandler struct{}

func NewIconHandler() *IconHandler {
	return &IconHandler{}
}

func (h *IconHandler) Register(api huma.API, mux *http.ServeMux) {
	web.ApiPost(api, "/api/icon/extract", "从本地路径（.app 目录或图片文件）或 http(s) URL 提取图标（64px PNG，base64）", h.iconExtract)
}

// IconExtractInput icon/extract 接口入参。source 由后端按形态分发：
// http(s):// 拉取（favicon.ico / png / jpg / gif）、.app 目录走 icns、文件按图片解码。
type IconExtractInput struct {
	Body struct {
		Source string `json:"source" doc:"图标来源：本地路径（.app 目录或图片文件）或 http(s) URL（如 https://gitee.com/favicon.ico）"`
	}
}

func (h *IconHandler) iconExtract(input IconExtractInput) (map[string]any, error) {
	pngData, err := iconkit.ExtractFromSource(input.Body.Source)
	if err != nil {
		return nil, err
	}
	return map[string]any{"value": base64.StdEncoding.EncodeToString(pngData)}, nil
}
