package opener

import "fmt"

// Icon 图标声明：只判别「前端怎么渲染」，不管图片来源。
//   - type=lucide：前端按 value 的图名渲染 lucide-react 图标；
//   - type=image：value 是 base64 PNG，前端直接渲染（.app 提取 / 上传图片两个入口
//     都先落成 base64，见 util/iconkit）。
//
// 零值表示未配置，前端 fallback 默认图标。
type Icon struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// icon 类型枚举。
const (
	IconTypeLucide = "lucide"
	IconTypeImage  = "image"
)

// InitIcon 校验 icon 声明并解析默认值。nil 视为未配置（合法），按主 role 推导
// 默认 icon——Opener 对外永远有 Icon，调用方无需处理缺省；type 非法或 value 为空报中文错误。
func InitIcon(icon *Icon) (Icon, error) {
	if icon == nil {
		return defaultIcon(), nil
	}
	switch icon.Type {
	case IconTypeLucide, IconTypeImage:
	default:
		return Icon{}, fmt.Errorf("未知的 icon type %q（合法值：lucide/image）", icon.Type)
	}
	if icon.Value == "" {
		return Icon{}, fmt.Errorf("icon value 不得为空（type=%s）", icon.Type)
	}
	return *icon, nil
}

// defaultIcon 未配置 icon 时按主 role 推导的 lucide 默认图（前端按名渲染）。
func defaultIcon() Icon {
	return Icon{Type: IconTypeLucide, Value: "app-window-mac"}
}
