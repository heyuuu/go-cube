# cube 文档

> cube —— 面向个人开发者的本地项目管理工具（CLI 优先 + 本地 Web）。

## 目录索引

### [spec/](spec/) — 项目现状

事实快照，描述当前代码「是什么」。

- [`spec/现状.md`](spec/现状.md) — 定位、分层架构、关键机制、命令列表、API 列表、数据模型、配置文件、测试策略

### [proposals/](proposals/) — 待办提案

未来需求的提案，每个提案一个目录（`YYMMDD-提案名/`）。

- [`260811-前端栈迁移/`](proposals/260811-前端栈迁移/) — Alpine.js → Vite+React+TS+React Query 整体迁移
- [`260811-模板引擎cube-create/`](proposals/260811-模板引擎cube-create/) — `cube create` 模板引擎（设计已完整）
- [`260811-workspace工作台/`](proposals/260811-workspace工作台/) — workspace 工作台 + diff / 伪终端
- [`260811-web层测试补全/`](proposals/260811-web层测试补全/) — 为零测试的 web 层补测试基建
- [`260811-history清理API/`](proposals/260811-history清理API/) — history 数据清理（暂不做）
- [`260811-sqlc代替gorm/`](proposals/260811-sqlc代替gorm/) — sqlc 代替 gorm（暂不做，未来方向）

### [archived/](archived/) — 已完成提案归档

已实现的需求总结（从 proposals 移入，记录最终落地形态与方案演变）。

- [`260811-server后台常驻与HTTP管理/`](archived/260811-server后台常驻与HTTP管理/) — server 后台常驻（`start -d`）+ 基于 HTTP API 的进程管理（whoami/shutdown）

### [references/](references/) — 参考文献目录

同类工具的深度分析。

- [`mani.md`](references/mani.md) — alajmo/mani 竞品分析
- [`gitbatch.md`](references/gitbatch.md) — isacikgoz/gitbatch 竞品分析（含批量操作避坑点）

### [misc/](misc/) — 杂项

暂时不好划分的文件。

- [`project-template-spec.md`](misc/project-template-spec.md) — 从 cube 提炼的通用项目设计规范模板
- [`alpine-intro.html`](misc/alpine-intro.html) — Alpine.js 可交互演示

## 文档规则

- 目录下最重要的索引文件叫 `README.md`，其他文件一律小写无大写。
- 除 `README.md` 外，所有 markdown 文件名用小写（如 `现状.md` 可保留中文但不用大写英文）。
- 提案目录命名：`YYMMDD-提案名/`，内部主文件叫 `README.md`，细节放其他文件引用。
