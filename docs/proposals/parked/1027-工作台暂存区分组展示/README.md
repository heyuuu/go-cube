# 工作台暂存区分组展示

> **状态**：⏸️ 挂起（需求已想清楚轮廓，暂不实施；触发条件见文末）

## 背景

工作台内容面板在 `current = worktree://<path>`、`base = 空` 时，diff 实际是 **HEAD vs 磁盘** 的混合态——暂存区（staged）和未暂存（unstaged）的内容合并成一个 diff 展示。参考 SourceTree 的交互：变更文件清单分两组（上方暂存区、下方未进暂存区），通过 UI 直接 add / unstage 文件。

现状与目标的差距：

1. `/api/workbench/changes`（workbench 源）一锅端 staged + unstaged + untracked，分组需要拆成两次对比：staged 组 = HEAD vs index（`git diff --cached`），unstaged 组 = index vs 工作区（裸 `git diff` + untracked）。一个文件可同时出现在两组（暂存后又改）。
2. 点开单文件的行级 diff 基准须随分组变：staged 组显示 HEAD↔index，unstaged 组显示 index↔磁盘。后端 `readSide` 需能读出「暂存区版本」（`git show :path`），可藏在 workbench 内部实现，不动 `TreeSource` 序列化协议。
3. add / unstage（`git add` / `git restore --staged`）是**写 index 操作**——workbench 目前纯读（+文件保存），加按钮后从只读变可写，是定性变化。

## 改动量评估（若实施）

- **后端纯加法，量不大**：`util/git` 加约 4 个类型化函数（staged/unstaged 清单、暂存区读文件、add/unstage 写）；workbench 加分组变更清单方法 + stage/unstage 动作；web 层按惯例 GET 清单 + POST 动作（如 `workbench/git/stage`）。
- **前端是改动大头**：`tree-pane.tsx` 变更树拆两个分组区 + 每组操作按钮；点文件 diff 基准按组切换；操作后 react-query invalidate 刷新。

## 为什么挂起

- 与现有逻辑有定性偏差：workbench 定位是「查看/对比」，引入写 index 操作后「误点改变仓库状态」进入面板设计考量，需先想清楚定位（轻 SourceTree 演进 vs 纯只读、写操作放 `cmd` 层如 `cube git stage`）。
- 当前无紧迫使用场景，属于「有更好、没有不痛」的增强。

## 建议实施路径（解挂时参考）

拆两步，第一步很小、且不引入写操作：

1. **分组展示**（纯读）：清单拆 staged/unstaged 两组，点文件显示对应侧 diff。不动 `TreeSource`，index 侧在 workbench 内部处理。
2. **stage/unstage 按钮**（可写）：基于分组视图增量加 POST 动作 + 按钮。

## 解挂条件

- 实际工作流中频繁需要「看哪些进了暂存区」或 UI 上直接暂存/移出暂存；
- 或对 workbench 定位（是否做轻 SourceTree）有了明确结论。
