package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/cmd/util/console"
	"github.com/heyuuu/cube/cmd/util/easycobra"
	"github.com/heyuuu/cube/util/pathkit"
)

// defaultGitignore 是新建 .gitignore 时使用的最小模板
const defaultGitignore = `# macOS
.DS_Store

# IDE
.idea/
.vscode/
*.swp

# 日志与临时文件
*.log
tmp/
`

// cmd `project init`
var projectInitCmd = &easycobra.Command{
	Use:   "init",
	Short: "在指定目录初始化一个项目(本质是初始化 git 仓库)",
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		var projectPath string
		cmd.Flags().StringVarP(&projectPath, "path", "p", ".", "待初始化的项目目录")

		return func(args []string) error {
			// 解析为绝对路径
			absPath, err := filepath.Abs(pathkit.RealPath(projectPath))
			if err != nil {
				return fmt.Errorf("解析路径失败: %w", err)
			}

			// 目录必须存在
			info, err := os.Stat(absPath)
			if err != nil {
				return fmt.Errorf("目录不存在或不可访问: %s", absPath)
			}
			if !info.IsDir() {
				return fmt.Errorf("指定路径不是目录: %s", absPath)
			}

			// 检查：从当前目录向上查找，若任意层级已存在 .git 则报错退出
			if existingGit := findGitRoot(absPath); existingGit != "" {
				return fmt.Errorf("目录 %s 已处于 git 仓库中 (.git 位于 %s)，无法重复初始化",
					pathkit.PrettyPath(absPath), pathkit.PrettyPath(existingGit))
			}

			// 执行 git init
			fmt.Printf("> 在 %s 执行 git init\n", pathkit.PrettyPath(absPath))
			if err = runGit(absPath, "init"); err != nil {
				return fmt.Errorf("git init 失败: %w", err)
			}

			// 若当前目录没有 .gitignore，询问是否创建一个
			gitignorePath := filepath.Join(absPath, ".gitignore")
			if _, err = os.Stat(gitignorePath); os.IsNotExist(err) {
				if console.Confirm("当前目录没有 .gitignore，是否创建一个？") {
					if err = os.WriteFile(gitignorePath, []byte(defaultGitignore), 0644); err != nil {
						return fmt.Errorf("创建 .gitignore 失败: %w", err)
					}
					fmt.Println("> 已创建 .gitignore")
				}
			}

			// 若当前目录有文件，询问是否 git add . 及是否 git commit -m 'init'
			if hasFiles(absPath) {
				if console.Confirm("当前目录存在文件，是否执行 git add . ？") {
					if err = runGit(absPath, "add", "."); err != nil {
						return fmt.Errorf("git add 失败: %w", err)
					}
					fmt.Println("> git add . 完成")

					if console.Confirm("是否执行 git commit -m 'init' ？") {
						if err = runGit(absPath, "commit", "-m", "init"); err != nil {
							return fmt.Errorf("git commit 失败: %w", err)
						}
						fmt.Println("> git commit 完成")
					}
				}
			}

			fmt.Printf("\n> 项目初始化完成: %s\n", pathkit.PrettyPath(absPath))
			return nil
		}
	},
}

// findGitRoot 从 dir 开始向上查找，返回最先出现 .git(文件或目录) 的目录；
// 一路查到根目录都没找到则返回空串。
func findGitRoot(dir string) string {
	for {
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir { // 已到根目录
			return ""
		}
		dir = parent
	}
}

// hasFiles 判断目录下是否存在任何条目（不递归，忽略 .git）。
func hasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		return true
	}
	return false
}

// runGit 在指定工作目录下执行 git 命令，stdout/stderr 透传给当前终端。
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
