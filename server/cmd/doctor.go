package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/project"
	"cube/util/git"
	"cube/util/pathkit"
	"cube/util/tui"
)

// doctorFinding 一条体检发现：某个项目/路径存在需要处理的问题。
type doctorFinding struct {
	path    string // 问题路径
	problem string // 问题描述
	detail  string // 排查线索（git 报错原文等）
}

// doctorCheck 一个体检项：对全部已收录项目跑一遍，返回问题清单。
type doctorCheck struct {
	name string // 检查项名，用于 help 与输出
	desc string // 检查项说明
	run  func(service *project.Service) []doctorFinding
}

// allDoctorChecks 所有体检项。新问题检查统一加在这里，cube doctor 自动带上。
var allDoctorChecks = []doctorCheck{
	{
		name: "git-repo-broken",
		desc: "被收录为项目、但 git 已无法正常读写的仓库（损坏的 .git 等）",
		run:  doctorCheckGitRepoBroken,
	},
	{
		name: "worktree-lost",
		desc: "快照内 worktree 路径已失联（目录被删/移动/重命名，等下次采集自然淘汰）",
		run:  doctorCheckWorktreeLost,
	},
}

// cmd `cube doctor`
func newDoctorCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor [items...]",
		Short: "体检 cube 管理的环境，找出异常损坏的项目",
		Long: `检查环境层面的异常与损坏，与 check 的分工：
  check 找的是正常业务状态（开发中、未推送等需要关注的项目），
  doctor 找的是异常损坏（git 环境坏了、数据残缺等非正常状态的问题）。

目前支持的检查项：
  - git-repo-broken：被收录为项目、但 git 已无法正常读写的仓库
    （损坏的 .git 等）。
  - worktree-lost：主项目快照里的 worktree 路径已失联（目录被删/移动/
    重命名）。1032 归并后 worktree 不是独立项目、悬空目录不再被收录，
    该检查面向快照数据而非扫描列表；失联条目等下次采集自然淘汰。

不传 items 时，依次执行全部检查项。后续新增的损坏类检查统一收到本命令下。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := make([]doctorCheck, 0, len(allDoctorChecks))
			if len(args) == 0 {
				checks = allDoctorChecks
			} else {
				for _, name := range args {
					match := false
					for _, c := range allDoctorChecks {
						if c.name == name {
							checks = append(checks, c)
							match = true
							break
						}
					}
					if !match {
						return fmt.Errorf("未支持的检查项: %s", name)
					}
				}
			}

			service := a.ProjectService()
			total := 0
			for _, c := range checks {
				findings := c.run(service)
				total += len(findings)
				printDoctorFindings(c, findings)
			}

			if total == 0 {
				fmt.Println("\n> 环境健康，未发现问题")
			}
			fmt.Print("\n")
			return nil
		},
	}
	return cmd
}

func printDoctorFindings(c doctorCheck, findings []doctorFinding) {
	if len(findings) == 0 {
		fmt.Printf("> [%s] 未发现问题\n", c.name)
		return
	}

	fmt.Printf("> [%s] 发现 %d 个问题:\n", c.name, len(findings))
	headers := []string{"Path", "问题", "详情"}
	rows := make([][]string, 0, len(findings))
	for _, f := range findings {
		rows = append(rows, []string{
			pathkit.PrettyPath(f.path),
			f.problem,
			f.detail,
		})
	}
	tui.PrintTable(headers, rows)
}

// doctorCheckGitRepoBroken 找出 git 已无法正常读写的收录项目。
// 判定信号：git.Refs 对病态仓库返回 error（正常仓库/空仓库都返回零值+nil）。
func doctorCheckGitRepoBroken(service *project.Service) []doctorFinding {
	var findings []doctorFinding
	for _, p := range service.Projects() {
		if _, err := git.Refs(p.Path()); err != nil {
			findings = append(findings, doctorFinding{
				path:    p.Path(),
				problem: "git 无法读取该仓库",
				detail:  err.Error(),
			})
		}
	}
	return findings
}

// doctorCheckWorktreeLost 找出主项目快照里已失联的 worktree 路径（1032）。
// worktree 归并为项目打开目标后其可见性完全来自快照枚举——目录被删/移动/重命名时
// 快照条目残留到下次采集，此检查在窗口期内把失联路径指出来。
func doctorCheckWorktreeLost(service *project.Service) []doctorFinding {
	var findings []doctorFinding
	for _, p := range service.Projects() {
		info, ok := service.GitInfo(p.Path())
		if !ok {
			continue
		}
		for _, wt := range info.Worktrees {
			if _, err := os.Stat(wt.Path); err != nil {
				findings = append(findings, doctorFinding{
					path:    wt.Path,
					problem: "快照内 worktree 路径已失联（所属项目 " + p.Path() + "）",
					detail:  err.Error(),
				})
			}
		}
	}
	return findings
}
