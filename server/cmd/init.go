package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/util/git"
	"cube/util/pathkit"
	"cube/util/tui"
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

// cmd `cube init`
func newInitCmd(a *app.App) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "在指定目录初始化一个项目(本质是初始化 git 仓库)",
		Long: `在指定目录初始化一个新项目，本质是初始化 git 仓库。

目录必须已存在，且从该目录向上探测不得已有 .git
（不允许在已有仓库内重复初始化）。

初始化过程按需交互确认：
  - 目录下没有 .gitignore 时，询问是否用内置模板创建一个。
  - 目录下已有文件时，询问是否 git add . 并提交 'init'。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 解析为绝对路径
			absPath, err := pathkit.AbsPath(projectPath)
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
			if existingGit, ok := git.FindGitRoot(absPath); ok {
				return fmt.Errorf("目录 %s 已处于 git 仓库中 (.git 位于 %s)，无法重复初始化",
					pathkit.PrettyPath(absPath), pathkit.PrettyPath(existingGit))
			}

			// 执行 git init
			fmt.Printf("> 在 %s 执行 git init\n", pathkit.PrettyPath(absPath))
			if err = git.Init(absPath); err != nil {
				return fmt.Errorf("git init 失败: %w", err)
			}

			// 若当前目录没有 .gitignore，询问是否创建一个
			gitignorePath := filepath.Join(absPath, ".gitignore")
			if _, err = os.Stat(gitignorePath); os.IsNotExist(err) {
				ok, err := tui.Confirm("当前目录没有 .gitignore，是否创建一个？")
				if err != nil {
					return err
				}
				if ok {
					if err = os.WriteFile(gitignorePath, []byte(defaultGitignore), 0644); err != nil {
						return fmt.Errorf("创建 .gitignore 失败: %w", err)
					}
					fmt.Println("> 已创建 .gitignore")
				}
			}

			// 若当前目录有文件，询问是否 git add . 及是否 git commit -m 'init'
			if hasFiles(absPath) {
				ok, err := tui.Confirm("当前目录存在文件，是否执行 git add . ？")
				if err != nil {
					return err
				}
				if ok {
					if err = git.Add(absPath, "."); err != nil {
						return fmt.Errorf("git add 失败: %w", err)
					}
					fmt.Println("> git add . 完成")

					ok, err := tui.Confirm("是否执行 git commit -m 'init' ？")
					if err != nil {
						return err
					}
					if ok {
						if err = git.Commit(absPath, "init"); err != nil {
							return fmt.Errorf("git commit 失败: %w", err)
						}
						fmt.Println("> git commit 完成")
					}
				}
			}

			fmt.Printf("\n> 项目初始化完成: %s\n", pathkit.PrettyPath(absPath))
			return nil
		},
	}
	cmd.Flags().StringVarP(&projectPath, "path", "p", ".", "待初始化的项目目录")
	return cmd
}

// findGitRoot / runGit 等 git 操作已合并入 util/git 包
// （对应 git.FindGitRoot / git.Run / git.Init / git.Add / git.Commit）。

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
