// 提供了类RPC的服务端功能实现，允许拓展兼容 web api、rpc、wails api 等的调用
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/heyuuu/go-cube/internal/handlers"
	"net/http"
	"strings"
)

// errors
var (
	errMethodNotFound = errors.New("api not found")
)

func IsMethodNotFound(err error) bool {
	return errors.Is(err, errMethodNotFound)
}

// Server 服务器，响应 api 请求
type Server struct {
	handlers map[string]handlers.HandleFunc
}

func NewServer(apiHandlers []handlers.Handler) *Server {
	s := &Server{
		handlers: map[string]handlers.HandleFunc{},
	}

	// init api handlers
	for _, apiHandler := range apiHandlers {
		apiHandler.Register(s.registerHandler)
	}

	return s
}

func (s *Server) registerHandler(name string, handler handlers.HandleFunc) {
	name = strings.Trim(name, "/")
	if name != "" {
		s.handlers[name] = handler
	}
}

func (s *Server) Call(name string, args any) (result any, err error) {
	h, ok := s.handlers[name]
	if !ok {
		return nil, errMethodNotFound
	}

	return h(args)
}

func (s *Server) StartHTTP(addr string) error {
	http.Handle("/api/", s)

	fmt.Printf("Server starting on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}

// 以 http 的方式提供 api 调用
// 限定请求必须以 api/ 为前缀，使用 POST JSON 方式请求
func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// 校验路由名，并从中获取对应 api 名
	apiName, ok := strings.CutPrefix(request.URL.Path, "/api/")
	if !ok {
		s.fastResponse(writer, http.StatusNotFound, "404 api not found")
		return
	}

	// 查找对应 handler
	handler, ok := s.handlers[apiName]
	if !ok {
		s.fastResponse(writer, http.StatusNotFound, "404 api not found")
		return
	}

	// 限定必须为 POST 请求
	if request.Method != http.MethodPost {
		s.fastResponse(writer, http.StatusMethodNotAllowed, "only POST method is allowed")
		return
	}

	// 读取请求参数
	var args any
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&args); err != nil {
		s.fastResponse(writer, http.StatusBadRequest, "failed to parse JSON body")
		return
	}

	// 调用对应的处理函数
	var response *ApiResponse
	if result, err := handler(args); err == nil {
		response = NewApiResponse(true, "", result)
	} else {
		response = NewApiResponse(false, err.Error(), nil)
	}

	// json 化；若失败，返回 500
	content, err := json.Marshal(response)
	if err != nil {
		s.fastResponse(writer, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回正常处理结果
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	writer.Write(content)
}

func (s *Server) fastResponse(writer http.ResponseWriter, code int, message string) {
	writer.WriteHeader(code)
	_, _ = writer.Write([]byte(message))
}
