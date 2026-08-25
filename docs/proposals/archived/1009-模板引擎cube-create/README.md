# 模板引擎（cube create）

> **状态**：✅ 已交付（2026-08-25 归档）。三步全部落地（本地单模板 → 模板集 → git 来源），domain 包 `server/create`，测试 50+ 用例全绿。
> 实施与设计的偏差均已回写：协议加 `version` 字段；模板集判定改**收纳式**（模板统一在 `templates/` 下）；CLI 从「位置参数切语义」改为**单位置参数 + flag**（`cube create <目标路径> [--tpl] [--tpl-name] [--var]`）；config 加 `create.templateSource` 默认来源（`--tpl` 缺省交互输入框预填）；目标路径校验提前到一切交互之前。
> 留白：init 失败处理策略 `on_fail`（默认中止，够用）。

## 说明

启动 `cube create` 时直接回看本提案下的设计文档：

- [`design/README.md`](design/README.md) —— 结论综述
- [`design/discussion.md`](design/discussion.md) —— 讨论详情（为什么这么定）
- [`design/glob-rules.md`](design/glob-rules.md) —— glob 规则细节

## 核心设计（一句话）

**引擎是机制，模板是数据。** 引擎对技术栈/目录结构/变量含义一无所知，所有策略由模板通过协议（`template.yaml`）声明。

## 关键设计决策（已在上述文档定稿）

- 占位符由模板自定义，引擎不认识任何固定占位符。
- `${var}` 只在 template.yaml 内生效，不泄漏到模板文件。
- 精确字符串替换，不用正则。
- glob 匹配的文件，路径（含文件名）+ 内容统一替换。
- 不做条件裁剪（差异靠多模板），不做变量派生。
- 引擎无隐藏行为（不自动 git init，交给 init 声明）。
- glob 规则以 doublestar 语义为准。

## 启动时的待决项

- 失败处理策略细节。

## 已决项（后续讨论补定）

- **模板不内置，全部外置**。来源只支持两种：本地目录 / git 仓库（clone 到临时目录，不缓存）。理由：内置模板改一次要重编译二进制，太难修改；外置让模板可以独立于 cube 版本演进。
- **支持模板集（收纳式）**：模板统一放在来源的 `templates/` 子目录下（一级各含 template.yaml），根目录其他内容不参与判定；CLI 形态：`cube create <目标路径> [--tpl 模板来源] [--tpl-name 模板名] [--var k=v]`，位置参数仅目标路径，其余缺省交互补齐（--tpl 预填 config `create.templateSource` 默认来源）。git 来源遍历时跳过 `.git`。
- **开发顺序**：① 本地目录单模板 → ② 本地目录模板集 → ③ git 来源。
