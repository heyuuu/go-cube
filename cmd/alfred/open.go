package alfred

import (
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
)

// cmd `project open`
var projectOpenCmd = &easycobra.Command{
	Use:   "project-open {project : 项目名} {--app= : 打开项目的App}",
	Short: "打开项目。非交互模式只支持准确项目名，非交互模式下支持模糊搜索",
	Args:  cobra.ExactArgs(1),
	InitRun: func(cmd *cobra.Command) func(cmd *cobra.Command, args []string) {
		// init flags
		var appName string
		cmd.Flags().StringVar(&appName, "app", "", "打开项目的App")

		// run
		return func(cmd *cobra.Command, args []string) {
			projectName := args[0]

			appService := app.Default().ApplicationService()
			projService := app.Default().ProjectService()

			// 匹配项目
			proj := projService.FindByName(projectName)
			if proj == nil {
				log.Fatalln("未找到指定项目: " + projectName)
				return
			}

			// 获取打开项目的app
			openApp := appService.FindByName(appName)
			if openApp == nil {
				log.Fatal("未找到指定app: " + appName)
				return
			}

			// 打开项目
			err := passthruRun(openApp.Bin(), proj.Path())
			if err != nil {
				log.Fatalf("打开失败: " + err.Error())
			}
		}
	},
}

func passthruRun(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
