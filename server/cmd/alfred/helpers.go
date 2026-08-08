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
	result := H{
		"items": slicekit.Map(items, fn),
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		return err
	}

	fmt.Println(string(bytes))
	return nil
}
