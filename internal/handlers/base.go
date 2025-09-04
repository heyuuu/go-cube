package handlers

type H map[string]any

// HandleFunc 请求处理函数
type HandleFunc func(params any) (result any, err error)

// Handler 接口
type Handler interface {
	Register(register func(name string, handler HandleFunc))
}

// 返回所有 handlers (用于 wire 生成代码)
func AllHandlers(
	configHandler *ConfigHandler,
	projectHandler *ProjectHandler,
	workspaceHandler *WorkspaceHandler,
) []Handler {
	return []Handler{
		configHandler,
		projectHandler,
		workspaceHandler,
	}
}
