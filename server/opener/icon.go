package opener

import "cube/util/iconkit"

// Icon 图标声明，语义与校验收敛在 util/iconkit（跨领域共享）；
// 这里只保留 opener 特有策略：icon 必有值，未配置时按主 role 填默认图。
type Icon = iconkit.Icon

// icon 类型枚举（透传 iconkit）。
const (
	IconTypeLucide = iconkit.IconTypeLucide
	IconTypeImage  = iconkit.IconTypeImage
)

// InitIcon 校验 icon 声明并解析默认值。nil 视为未配置（合法），返回默认 icon——
// Opener 对外永远有 Icon，调用方无需处理缺省；type 非法或 value 为空报中文错误。
func InitIcon(icon *Icon) (Icon, error) {
	if icon == nil {
		return defaultIcon(), nil
	}
	if err := iconkit.ValidateIcon(icon); err != nil {
		return Icon{}, err
	}
	return *icon, nil
}

// defaultIcon 未配置 icon 时按主 role 推导的 lucide 默认图（前端按名渲染）。
func defaultIcon() Icon {
	return Icon{Type: IconTypeLucide, Value: "app-window-mac"}
}
