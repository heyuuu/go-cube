/**
 * 校验 src/components/ui 下的组件与 shadcn 官方 registry 输出是否一致。
 *
 * 流程：确认工作区干净 -> 逐组件重新下载（--overwrite）-> oxfmt 按项目规则格式化
 * -> git diff。无变动说明本地组件与官方一致；有变动则展示 diff 并报错退出，
 * 可用 `git checkout -- src/components/ui` 恢复。
 */
import { spawnSync } from "node:child_process"
import { readdirSync } from "node:fs"
import path from "node:path"
import process from "node:process"

const webDir = path.resolve(import.meta.dirname, "..")
const uiDir = path.join(webDir, "src/components/ui")

function run(cmd: string, args: string[], opts?: { cwd?: string }) {
  const result = spawnSync(cmd, args, {
    cwd: opts?.cwd ?? webDir,
    stdio: "pipe",
    encoding: "utf8",
  })
  if (result.error) throw result.error
  return result
}

function fail(message: string): never {
  console.error(`✗ ${message}`)
  process.exit(1)
}

// 组件清单从 ui 目录扫描得出，后续新增组件无需改本脚本
const components = readdirSync(uiDir)
  .filter((f) => f.endsWith(".tsx"))
  .map((f) => path.basename(f, ".tsx"))
if (components.length === 0) fail("src/components/ui 下没有找到任何组件")

// 前置：工作区必须干净，否则重新下载产生的 diff 会和未提交改动混在一起
run("git", ["rev-parse", "HEAD"])
const status = run("git", ["status", "--porcelain"])
if (status.status !== 0) fail("无法读取 git 状态，请确认在 git 仓库内")
if (status.stdout.trim() !== "") {
  console.error(status.stdout)
  fail("工作区存在未提交内容，请先提交或暂存（git stash）后再校验")
}
console.log(`✓ 工作区干净，基线 commit: ${run("git", ["rev-parse", "--short", "HEAD"]).stdout.trim()}`)

for (const component of components) {
  console.log(`↓ 重新下载 ${component} ...`)
  const result = run("pnpm", ["dlx", "shadcn@latest", "add", component, "--overwrite"])
  if (result.status !== 0) fail(`下载 ${component} 失败：\n${result.stderr}`)
}

// registry 输出经 CLI 内置 prettier 格式化，与项目 oxfmt 规则有换行/排序差异，
// 统一格式化后再 diff，避免纯格式差异误报
const fmt = run("pnpm", ["exec", "oxfmt", "src/components/ui"])
if (fmt.status !== 0) fail(`oxfmt 格式化失败：\n${fmt.stderr}`)
console.log("✓ 已按项目规则格式化")

const diff = run("git", ["diff", "--", "src/components/ui"])
if (diff.stdout.trim() === "") {
  console.log(`✓ ${components.length} 个组件与官方 registry 输出一致`)
  process.exit(0)
}

console.error("以下组件与官方 registry 输出存在差异：\n")
console.error(diff.stdout)
fail("组件校验失败。如需恢复：git checkout -- src/components/ui")
