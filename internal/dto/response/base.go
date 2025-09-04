package response

// API 响应的基础结构
type ApiResponse struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func NewApiResponse(ok bool, message string, data any) *ApiResponse {
	return &ApiResponse{ok, message, data}
}
