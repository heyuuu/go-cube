package alfred

import (
	"encoding/json"
	"fmt"

	"cube/util/slicekit"
)

type H map[string]any

// see: https://www.alfredapp.com/help/workflows/inputs/script-filter/json/
type Item struct {
	Title    string `json:"title"`
	SubTitle string `json:"subtitle"`
	Arg      string `json:"arg"`
}

func PrintResult[T any](items []T, fn func(item T) Item) error {
	return PrintItems(slicekit.Map(items, fn))
}

// PrintItems 直接输出已构造好的 Item 列表（条目不与单一源列表一一对应时用，如项目平铺展开为多目标）。
func PrintItems(items []Item) error {
	result := H{
		"items": items,
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		return err
	}

	fmt.Println(string(bytes))
	return nil
}
