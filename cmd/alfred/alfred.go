package alfred

import (
	"encoding/json"
	"fmt"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
	"log"
)

// alfred root cmd
var AlfredCmd = &easycobra.Command[any]{
	Use: "alfred",
}

func init() {
	easycobra.AddCommand(AlfredCmd, projectSearchCmd)
	easycobra.AddCommand(AlfredCmd, projectOpenCmd)
	easycobra.AddCommand(AlfredCmd, appSearchCmd)
}

// helpers

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
