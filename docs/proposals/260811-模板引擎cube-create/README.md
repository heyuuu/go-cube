# 模板引擎（cube create）

> **状态**：📋 待办（设计已完整，启动时直接用）
> **来源**：cube-next 吸收讨论（详见 [`docs/tech-notes/cube-next-absorption.md`](../../tech-notes/cube-next-absorption.md) 第 13.1 条）

## 说明

模板引擎的**设计产出在 cube 自己的仓库**，cube-next 只是引用说「旧设计可直接继承」，无新增内容。启动 `cube create` 时直接回看本提案下的设计文档：

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

- 模板内置还是外拉（`--template-url`）。
- 失败处理策略细节。
