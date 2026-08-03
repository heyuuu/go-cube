package project

import (
	"fmt"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
)

// cmd `project scan-rules`
var scanRulesCmd = &easycobra.Command{
	Use:   "scan-rules",
	Short: "列出 scan 规则",
	Run: func(args []string) error {
		service := app.Default().ProjectService()
		for _, rule := range service.ScanRules() {
			fmt.Println(rule.Group)
		}
		return nil
	},
}
