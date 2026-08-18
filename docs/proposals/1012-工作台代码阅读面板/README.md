# 工作台代码阅读面板：文件树 + CodeMirror6 查看 + 确认式轻编辑

> **状态**：✅ 已实现（2026-08-18）
>
> **所属**：[`1008-workspace工作台` 总纲](../1008-workspace工作台/README.md)（先读总纲「已收敛的全局决策」）。
> **依赖**：[`1010-workbench基座`](../1010-workbench基座/README.md)（`tree`/`file` API 契约、面板骨架）。

## 背景与目标

代码阅读面板浏览**某个 TreeSource 的文件内容**。两个数据源：**虚拟文件树**（从某 commit/ref 读，`git ls-tree` / `git show ref:path`）和**真实文件树**（某 worktree 目录的当前文件系统状态，含未提交改动）。虚拟树只读；真实树支持**轻编辑**——默认只读，开启编辑和保存都需弹窗确认。编辑器定为 **CodeMirror 6**（不用 Monaco，已定案）。

本面板同时是 1013（diff 面板）要复用的「文件树 + CodeMirror」底座，组件设计需考虑被 diff 复用（只读渲染模式）。

**本提案不做**：diff（1013）、git 写操作、stage/hunk 操作、多文件批量编辑。

## 方案

### 1. 后端：补齐 tree / file 两个接口的实现

1010 已定契约，本提案落地：

- `GET /api/workbench/tree?path=&sourceType=&sourceId=&dir=`
  - `sourceType=worktree`：读文件系统（`sourceId` 为目录绝对路径），返回一层子项（目录/文件、大小、是否目录）。**是否应用 .gitignore 过滤作为 query 参数**（`showIgnored`，默认过滤；不过滤时列出 ignored 项并标记——实现可用 `git status --ignored --porcelain` 或 `git check-ignore` 批量判定，实施时选性能好的）。目录内 `.git` 本身永远不列。
  - `sourceType=commit|ref`：`git ls-tree <ref> <dir>/` 解析（1010 可能已实现 `util/git.ListTreeAtRef`，有则直接用）。
  - 返回项含 `ignored` 标记（真实树、开启 showIgnored 时）。
- `GET /api/workbench/file?path=&sourceType=&sourceId=&file=`
  - worktree：`os.ReadFile`；commit/ref：`git show <ref>:<file>`。
  - 二进制检测（前 N 字节判 NULL / 常见二进制魔数）：二进制返回 `{binary: true}` 不带内容；超大文件（如 >2MB）拒绝并提示。
  - 需要文件语言/类型信息供前端高亮（前端按扩展名映射即可，后端不必返回）。
- **保存接口（唯一的写）**：`PUT /api/workbench/file?path=&dir=<worktree目录>&file=`
  - 只允许 `sourceType=worktree`（虚拟树不可编辑）。
  - 覆盖前做「内容未变化则跳过」检查；**不做**任何 git 操作（不 add 不 commit）。
  - 中文错误消息；保存成功返回简要信息。

### 2. 前端：面板组件（`web/src/pages/workbench/panels/code-view/`）

- **依赖**：安装 CodeMirror 6 包（`codemirror`、`@codemirror/lang-*` 按需、`@codemirror/theme` 类）。语言映射先覆盖常见类型（go/ts/tsx/js/json/md/css/html/py/sh/yaml），未知类型纯文本。
- **布局**：左窄栏文件树（懒加载子目录，逐层请求 tree 接口），右侧 CodeMirror 内容区。文件树视觉风格对齐 `web/src/pages/md/` 的树。
- **数据 hook**：`useTree(source, dir)` / `useFile(source, file)`，Query key 从 URL 参数 + source 派生（1010 面板铁律）。
- **只读模式（默认）**：CodeMirror `readOnly` + 顶部显示当前 TreeSource 徽标（如 `commit:abc1234` / `worktree:/path/to/dir`）。
- **轻编辑（仅 worktree 源）**：
  1. 点「编辑」按钮 → 弹窗确认（「将修改工作区文件 xxx」）→ 进入可编辑。
  2. 保存 → 再次弹窗确认 → 调 PUT 接口 → 成功后 invalidate 该文件与该目录 status 相关 query。
  3. 切换文件/TreeSource 时有未保存改动 → 阻断并提示（用 dirty state 拦路由/参数变化）。
- **分支切换快捷方式**（非主流程）：内容区顶部一个下拉，直接切换当前面板浏览的 TreeSource（改写 URL `sourceType/sourceId`），等价于从 git 树面板重新选一次。
- 选中文件进 URL（如 `&file=src/main.go`），刷新恢复。

### 3. 与 1013 的复用边界

- 文件树组件、CodeMirror 只读渲染封装（含语言映射）写成**可复用导出**（接收 source props，不读 URL），diff 面板以受控方式复用；URL 驱动的 hook 留在本面板。

## 验收标准

1. `git ls-tree` / `git show ref:path` 解析、二进制检测、ignored 判定有表驱动测试；tree/file/PUT file 有 httptest 用例（含：worktree 读取、commit 读取、二进制、保存、向虚拟源 PUT 被拒）。
2. 打开 `/workbench?path=…&sourceType=commit&sourceId=<sha>`：能看到该 commit 的虚拟文件树并读文件；切到 worktree 源可编辑+确认+保存，改动落盘（用 `git status` 验证工作区已 dirty）。
3. 未保存切换被阻断；刷新页面选中态恢复。
4. `cd server && go vet ./... && go test ./...`、`pnpm -C web build` 通过。

## 实施注意

- git 读能力沉淀 `util/git`（纯函数），workbench 包编排；写文件属于 workbench 服务职责（不是 git 操作）。
- PUT 接口是本工作台唯一落盘写路径，注意路径拼接安全（file 参数不得逃逸出目标目录，做 `filepath.Clean` + 前缀校验）。
- 弹窗用仓库内 shadcn Dialog（Base UI，`render` prop 而非 asChild）。
