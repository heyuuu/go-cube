package alfred

import (
	"encoding/json"
	"fmt"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
	"log"
)

type H map[string]any

// see: https://www.alfredapp.com/help/workflows/inputs/script-filter/json/
type Item struct {
	Title    string `json:"title"`
	SubTitle string `json:"subtitle"`
	Arg      string `json:"arg"`
}

func PrintResult(items []Item) {
	result := H{
		"items": items,
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(bytes))
}

func PrintResultFunc[T any](items []T, fn func(item T) Item) {
	listItems := slicekit.Map(items, fn)
	PrintResult(listItems)
}
