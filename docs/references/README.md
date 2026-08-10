# 参考项目分析

> 本目录收录对 cube 有借鉴价值的同类工具的深度分析。
> 从它们的设计里提取「做对了什么 / 做错了什么」，为 cube 的决策提供依据。

## 收录的参考项目

- [`mani.md`](./mani.md) — [alajmo/mani](https://github.com/alajmo/mani)：声明式配置 + 团队共享项目拓扑 + 跨项目命令执行
- [`gitbatch.md`](./gitbatch.md) — [isacikgoz/gitbatch](https://github.com/isacikgoz/gitbatch)：TUI + 多仓库批量同步器

## 来源说明

这两份分析来自 cube-next（已废弃的重构项目）的调研产出。正文中的「cube-next」是历史称呼，分析内容本身（竞品事实 + 避坑点）与项目名无关，可直接为 cube 所用。

最值得参考的部分：
- **gitbatch.md 的「E. 批量操作的并发/失败策略」和「F. 避坑点」**——cube 未来做 gitx 批量、多项目 push/pull 时的现成设计依据（超时、可配并发、fetch 可重试/push 不自动重试、best-effort + fails map）。
- **mani.md 的实体建模和命令正交设计**——tags 过滤、exec/run 二分、flag 跨命令复用等思路。
