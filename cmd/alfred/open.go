package alfred

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/console"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
)

// cmd `project open`
type projectOpenFlags struct {
	app string
}

var projectOpenCmd = &easycobra.Command[projectOpenFlags]{
	Use:   "project-open {project : 项目名} {--app= : 打开项目的App}",
	Short: "打开项目。非交互模式只支持准确项目名，非交互模式下支持模糊搜索",
	Args:  cobra.ExactArgs(1),
	Init: func(cmd *cobra.Command, flags *projectOpenFlags) {
		cmd.Flags().StringVar(&flags.app, "app", "", "打开项目的App")
	},
	Run: func(cmd *cobra.Command, flags *projectOpenFlags, args []string) {
		query := args[0]

		// 获取打开项目的app
		applicationService := app.Default().ApplicationService()
		openApp := applicationService.FindApp(flags.app)
		if openApp == nil {
			log.Fatal("未找到指定app: " + flags.app)
			return
		}

		// 匹配项目
		proj := selectProject(query, "")
		if proj == nil {
			return
		}

		// 打开项目
		err := passthruRun(openApp.Bin(), proj.Path())
		if err != nil {
			log.Fatalf("打开失败: " + err.Error())
		}
	},
}

func selectProject(query string, workspace string) *entities.Project {
	service := app.Default().ProjectService()
	projects := service.SearchInWorkspace(query, workspace)
	switch len(projects) {
	case 0:
		fmt.Println("没有匹配的项目")
		return nil
	case 1:
		return projects[0]
	default:
		proj, ok := console.ChoiceItem("选择项目", projects, (*entities.Project).Name)
		if !ok {
			fmt.Println("选择项目失败")
			return nil
		}
		return proj
	}
}

func passthruRun(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
