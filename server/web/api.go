package web

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"unicode"

	"github.com/danielgtaylor/huma/v2"
)

// --- 数据类型 ---

// ApiOutput api 标准输出结构
type ApiOutput[T any] struct {
	Body struct {
		Ok      bool   `json:"ok"`
		Message string `json:"message"`
		Data    *T     `json:"data"`
	}
}

// ListResult 列表请求通用结果
type ListResult[T any] struct {
	List []T `json:"list"`
}

func listResult[T any](list []T) ListResult[T] {
	return ListResult[T]{List: list}
}

// --- api register --

type humaHandler[I, O any] = func(context.Context, *I) (*O, error)

// api 注册的统一入口
func apiRegister[I, O any](api huma.API, op huma.Operation, handler humaHandler[I, O]) {
	if op.Method == "" {
		panic("operation 必须指定 method")
	}
	if op.Path == "" {
		panic("operation 必须指定 path")
	} else if !strings.HasPrefix(op.Path, "/api/") {
		panic("api path 必须以 /api/ 开头")
	}

	group, operationId := parseInfoFromPath(op.Path)

	// 默认将分组作为 tag
	if !slices.Contains(op.Tags, group) {
		op.Tags = slices.Concat(op.Tags, []string{group})
	}

	// 默认 operation-id，影响使用方的代码生成
	if op.OperationID == "" {
		op.OperationID = operationId
	}

	// 默认 summary (huma.{Get/Post}() 默认逻辑)
	if op.Summary == "" {
		var o *O
		op.Summary = huma.GenerateSummary(op.Method, op.Path, o)
	}

	huma.Register(api, op, handler)
}

func parseInfoFromPath(p string) (group string, operationId string) {
	p = strings.Trim(strings.TrimPrefix(p, "/api"), " /")
	if p == "" {
		return "index", "index.index"
	}

	if idx := strings.IndexByte(p, '/'); idx < 0 {
		return p, p + ".index"
	} else {
		group = p[:idx]
		rest := toCamelPath(p[idx+1:])
		operationId = group + "." + rest
		return
	}
}

// toCamelPath 把路径段转驼峰："/clone-rules" → "cloneRules"（"-"后字母大写，其余原样）
func toCamelPath(p string) string {
	var b strings.Builder
	upper := false
	for _, r := range p {
		switch {
		case r == '-' || r == '/':
			upper = true
		case upper:
			b.WriteRune(unicode.ToUpper(r))
			upper = false
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// --- handlers ---

func jsonHandler[I, O any](h func(I) (O, error)) humaHandler[I, ApiOutput[O]] {
	return func(ctx context.Context, input *I) (*ApiOutput[O], error) {
		data, err := h(*input)

		var output ApiOutput[O]
		if err != nil {
			output.Body.Ok = false
			output.Body.Message = err.Error()
			output.Body.Data = nil
		} else {
			output.Body.Ok = true
			output.Body.Message = ""
			output.Body.Data = &data
		}
		return &output, nil
	}
}

// --- api helpers 快捷方法 ---

func apiGet[I, O any](api huma.API, path string, summary string, handler func(I) (O, error)) {
	apiRegister[I, ApiOutput[O]](api, huma.Operation{
		Method:  http.MethodGet,
		Path:    path,
		Summary: summary,
	}, jsonHandler(handler))
}

func apiPost[I, O any](api huma.API, path string, summary string, handler func(I) (O, error)) {
	apiRegister[I, ApiOutput[O]](api, huma.Operation{
		Method:  http.MethodPost,
		Path:    path,
		Summary: summary,
	}, jsonHandler(handler))
}
