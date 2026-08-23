# 工作台 diff 面板：双 TreeSource 目录/文件对比（Beyond Compare 级）

> **状态**：✅ 已实现（2026-08-18）
>
> **所属**：[`1008-workspace工作台` 总纲](../1008-workspace工作台/README.md)（先读总纲「已收敛的全局决策」）。
> **依赖**：[`1010-workbench基座`](../archived/1010-workbench基座/README.md)（diff/file-diff API 契约）、[`1012-工作台代码阅读面板`](../1012-工作台代码阅读面板/README.md)（文件树组件、CodeMirror 只读渲染底座）。

## 背景与目标

diff 面板对比**两个任意 TreeSource**（分支 vs 分支、commit vs worktree 目录、任意组合），提供目录树级对比 + 文件级双栏对比（类似 Beyond Compare），支持筛选项——特别的「是否包含 `.gitignore` 忽略的文件」，这决定了它有**两种对比模式**：

- **git 模式**：`git diff` 语义（快、准），但天然跳过 ignored/untracked；
- **fs 扫描模式**：文件系统层逐文件对比（Beyond Compare 的做法），全量可控、支持 ignored 开关，代价是慢（需并发读文件比较）。

**本提案不做**：三方合并/冲突解决、编辑 diff 内容、目录同步（Beyond Compare 的左右复制方向操作）。

## 方案

### 1. 后端：diffTrees / readFileDiff 实现（1010 已定契约）

**`GET /api/workbench/diff?path=&left=…&right=…&filters…`**

返回目录级 diff 结果（递归全树，平铺为变更路径列表）：

- **git 模式**（两侧都是 commit/ref 时）：`git diff --name-status <A> <B>` 解析，每项 `{path, status, oldPath?}`。
- **fs 扫描模式**（**任一侧为 worktree 即走 fs**——定稿简化，不再按 ignored/untracked 开关细分；要对比工作区含未提交改动本来就必须 fs）：
  - 真实目录 walk（非 ignored 场景走 `git ls-files` 提速，跳过 `.git`）；commit/ref 源用 `git ls-tree -r` 平铺。按相对路径对齐，比较存在性 + 内容 hash（**统一用 git blob sha1**（`sha1("blob <len>\0"+content)`），与 ls-tree 的 blob sha 直比，无需自造指纹算法）。
  - 同步算完，无并发 worker（文档允许的 MVP 降级）。
- **模式显式返回**：`DiffTreesResult.Mode`（"git"/"fs"）+ `IgnoredFilters`，前端不猜。
- **筛选项**（query 参数）：`showIgnored`（仅 fs 模式有效）、`showUntracked`（fs 模式恒含 untracked，该参数当前不生效——定稿口径：untracked 是工作区对比的本体，不做减法）、`statusFilter`（逗号分隔）、`pathPrefix`。响应里注明实际生效的模式与被忽略的筛选。产品层面已**不考虑 ignored 文件**，showIgnored 属保留参数。

**`GET /api/workbench/file-diff?path=&left…&right…&file=`**

- 单文件统一 diff：任何组合统一走 `git diff --no-color`（两侧 tree-ish 直接比；涉及 fs 的组合把两侧内容落临时文件后 `git diff --no-index`——**未引入 Go LCS 库**，算法与展示语义和 git 完全一致）。二进制文件返回 `{binary: true}`。
- 返回结构化结果（分块 hunks：旧/新行号范围 + 行内容标记 +/-/ctx，`parseUnifiedDiff`），前端渲染，不返回原始 patch 文本让前端解析。

### 2. 前端：面板组件（`web/src/pages/workbench/panels/diff-view/`）

- **入口**：git 树面板双选（1011 写入 `left…/right…` URL 参数）后内容区切到本面板；面板顶部显示两个源的徽标（左右），各带一个源切换下拉（改写 URL，与 1012 的快捷切换同款）。
- **目录级对比视图**：变更文件列表（按目录分组或平铺+路径过滤框），每行：状态图标（+新增 / −删除 / ~修改 / →重命名）+ 路径 + 筛选器工具条（状态多选、showIgnored/showUntracked 开关——开关会触发 fs 模式请求，慢时显示 loading）。
- **文件级对比视图**：点某文件 → 双栏或统一视图（MVP 先做 **side-by-side 双栏**，对齐行渲染；用 1012 的 CodeMirror 只读封装（`@codemirror/merge` 的 `MergeView` 可评估，若满足则直接用，注意与只读封装的整合方式），行内 diff 粒度到行，字符级高亮可后置）。
- **数据 hook**：`useTreesDiff(left, right, filters)` / `useFileDiff(left, right, file)`，filter 变化即换 key 重新请求；URL 只存 left/right 与选中 file，筛选项放组件内 state（不进 URL，避免参数爆炸）。
- 复用 1012 的文件树组件展示「diff 后的树形结构」可选做——MVP 用平铺列表 + 路径过滤即可，树形视图后置。

## 验收标准

1. `--name-status` 解析、fs walk 对齐、hash 指纹函数有表驱动测试；diffTrees（git 模式、fs 模式、worktree 工作区 vs commit）、fileDiff（结构化 hunks、二进制）有 httptest 用例（用 `internal/testfixture` 建真实仓库构造：改文件/删文件/新增 ignored 文件/untracked 文件）。
2. 打开 `/workbench?path=…&leftType=ref&leftId=main&rightType=worktree&leftId=<wt目录>`：目录对比正确；开 showIgnored 后 ignored 文件出现；文件级双栏 diff 与 `git diff` 结果一致。
3. 大目录（如本 cube 仓库自身）diff 请求在秒级返回或给出可理解的处理（loading/超时提示）。
4. `cd server && go vet ./... && go test ./...`、`pnpm -C web build` 通过。

## 实施注意

- diff 核心逻辑沉淀在 `server/workbench/diff.go`；纯解析/对齐算法写成纯函数便于表驱动测试。
- 降级规则（worktree 工作区对比自动 fs 模式）在接口响应中显式返回模式，前端不猜。
- fs 模式遍历要跳过 `.git` 目录；路径拼接做逃逸校验（同 1012）。
