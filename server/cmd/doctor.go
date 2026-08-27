package cmd

import (
	"errors"
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
	// fix 可选：--fix 模式下的安全修复（幂等且无损）。约定：只允许执行不破坏
	// 数据的清理类操作；修复后重跑 run 复核，剩余问题照常输出。
	fix func(service *project.Service) error
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
		desc: "git 元数据里目录已失联的 worktree（目录被删/移动/重命名，git worktree list 仍列出；--fix 可自动 prune）",
		run:  doctorCheckWorktreeLost,
		fix:  doctorFixWorktreeLost,
	},
}

// cmd `cube doctor`
func newDoctorCmd(a *app.App) *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "doctor [items...]",
		Short: "体检 cube 管理的环境，找出异常损坏的项目",
		Long: `检查环境层面的异常与损坏，与 check 的分工：
  check 找的是正常业务状态（开发中、未推送等需要关注的项目），
  doctor 找的是异常损坏（git 环境坏了、数据残缺等非正常状态的问题）。

目前支持的检查项：
  - git-repo-broken：被收录为项目、但 git 已无法正常读写的仓库
    （损坏的 .git 等）。
  - worktree-lost：主仓库 git 元数据里目录已失联的 worktree（目录被删/
    移动/重命名）。1032 归并后 worktree 不是独立项目，悬空目录不再被
    收录，但 git worktree list 在 prune 前仍会列出、采集时需按存在性
    过滤。检出后按主项目给出 prune 建议命令。

--fix：对支持修复的检查项执行安全修复（当前仅 worktree-lost：对涉及的
  主仓库跑 git worktree prune，幂等且无损——只删失效记录，不碰现存
  worktree）。修复后重跑检查复核，剩余问题照常输出。

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
				// --fix：先修复再复核（修复后重跑 run，剩余问题照常列出）
				if fix && len(findings) > 0 && c.fix != nil {
					if err := c.fix(service); err != nil {
						fmt.Printf("> [%s] 修复失败: %v\n\n", c.name, err)
						total += len(findings)
						printDoctorFindings(c, findings)
						continue
					}
					fmt.Printf("> [%s] 已执行修复，复核剩余问题...\n", c.name)
					findings = c.run(service)
				}
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
	cmd.Flags().BoolVar(&fix, "fix", false, "对支持修复的检查项执行安全修复（如 worktree-lost 自动 git worktree prune）")
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

// doctorCheckWorktreeLost 找出主仓库 git 元数据里目录已失联的 worktree（1032）。
// 现场跑 git.WorktreeList 探测（doctor 本就实时读 git，不受快照采集滞后影响）：
// 目录被删/移动/重命名后，git worktree list 在 prune 前仍会列出这些失效记录，
// 采集侧需按存在性过滤；此检查把它们连同名下的 prune 建议命令一起指出。
func doctorCheckWorktreeLost(service *project.Service) []doctorFinding {
	var findings []doctorFinding
	for _, p := range service.Projects() {
		wts, err := git.WorktreeList(p.Path())
		if err != nil {
			continue // 主仓库 git 不可读的场合归 git-repo-broken 报告
		}
		for _, wt := range wts[1:] { // 第 1 项是主目录自身
			if _, err := os.Stat(wt.Path); err != nil {
				findings = append(findings, doctorFinding{
					path:    wt.Path,
					problem: "worktree 目录已失联（所属项目 " + p.Path() + "）",
					detail:  "建议: git -C " + p.Path() + " worktree prune",
				})
			}
		}
	}
	return findings
}

// doctorFixWorktreeLost 对全部主仓库执行 git worktree prune（幂等且无损）。
// 逐仓库执行，单仓库失败不中断其余。
func doctorFixWorktreeLost(service *project.Service) error {
	var errs []error
	for _, p := range service.Projects() {
		if err := git.WorktreePrune(p.Path()); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Path(), err))
		}
	}
	return errors.Join(errs...)
}
