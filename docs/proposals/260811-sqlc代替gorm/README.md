# sqlc 代替 gorm

> **状态**：⏸️ 暂不做（记录为未来方向，可能在新项目实验）
> **来源**：cube-next 吸收讨论（详见 [`docs/tech-notes/cube-next-absorption.md`](../../tech-notes/cube-next-absorption.md) 第 5 条）

## 背景

cube 当前数据层用 gorm（`server/history/`），cube-next 选择了 sqlc（SQL-first 代码生成）。

## 为什么暂不做

1. **触发条件不成立**：cube 数据层极简（2 表 3 字段、固定 CRUD、零动态查询）。sqlc 相对 gorm 的优势（编译期类型安全、AutoMigrate 改/删列不漂移、schema 显式）在当前规模几乎不触发；gorm 的两个短板（AutoMigrate 漂移、动态查询弱）当前都没踩到。
2. **AI 契约价值下降**：sqlc 在 cube-next 的核心价值是「schema 作为防 AI 出错的审查契约」，但那依赖 OpenSpec + 全程 AI 模式。当前 cube 是人在主导重构，契约价值减半。
3. **换的成本不低**：codegen 工具链 + 重写 history + migration runner + 调试体验变化 + 未来每张新表走 migration→schema→query→generate 四步。

## 未来方向

sqlc 的「SQL 作为显式契约、便于 AI 审计」方向值得探索，但不在 cube 当前阶段验证。两种触发场景：

- **cube 内**：数据层显著复杂化（新表多、字段频繁演进、复杂查询）。
- **新项目**：可能在新项目里实验 sqlc 作为「解决 AI 审计」的方向。

## 轻量改进备选

若仅担心 gorm AutoMigrate 漂移，可加 migration 文件 + CI 校验，不上 codegen 即可拿到「schema 显式」的一半好处。未实施，按需启用。

## 参考实现

cube-next 完整 sqlc 实现见：
- `docs/design.md`「SQLite 库选型」章节（在 cube-next 仓库 `/Users/heyu/Code/heyuuu/cube-next/docs/design.md`）
- cube-next `server/sql/`（migrations / schema / query 三目录）
- cube-next `server/storage/`（Open + Migrate runner）
