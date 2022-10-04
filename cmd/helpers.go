package cmd

import (
	"encoding/json"
	"fmt"
	"log"
)

func showTable(table [][]string) {

}

func alfredSearchResult(items []any) {
	result := struct {
		Items []any `json:"items"`
	}{items}

	bytes, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(bytes))
}
