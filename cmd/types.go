package cmd

type H map[string]any

type AlfredListItem struct {
	Title    string `json:"title"`
	SubTitle string `json:"subtitle"`
	Arg      string `json:"arg"`
}
