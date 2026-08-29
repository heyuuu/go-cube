# 1038 opener 默认

## 状态

活动，准备开发。

## 背景

opener 列表目前是纯能力清单，没有任何「默认」概念：CLI `open` / `diff` 不传 `-o` 时进入交互选择，Web `opener/open` / `project/open` 不传 opener 时前端弹全量下拉。对固定工作流（如 diff 恒用同一工具）是重复操作。

## 需求

每个 role 一个默认 opener，未显式指定时自动使用：

- CLI：`open` / `open-path` / `diff` 不传 `-o` 时先查默认，默认缺失或指向已删 opener 时回退现有交互流程。
- Web：`opener/open` / `project/open` 的 opener 参数可选化（或前端先查默认），默认失效时回退全量下拉。

## 设计

### 数据：settings.json 新增 `openerDefaults` 节

```json
{
  "openerDefaults": {
    "open-dir": "finder",
    "open-file": "typora",
    "diff-dir": "beyond-compare",
    "diff-file": "kaleidoscope"
  }
}
```

- 键 = role 枚举值（4 个固定枚举），值 = opener name。
- 存 settings.json 而非 config.json：与 openers 节同源——运行中可变更的用户可管理数据，跟随节级 API 体系（`LoadSection` / `SaveSection`）。
- 全局配置，不按项目路径 keyed，不触碰「持久配置不记项目路径内容」红线。

### 领域逻辑（`opener` 包）

- `Service` 增默认读写方法：`DefaultOpener(role) (Opener, bool)`（直读不缓存，坏数据/失效名降级为无默认）+ `SaveDefaultOpener(role, name)` / `DeleteDefaultOpener(role)`（写侧校验：role 合法枚举、name 必须指向现存 opener，坏数据中文错误不落文件）。
- CLI 侧新增一个共用的「解析 opener」helper：`-o` 显式指定 > 默认 > 交互选择，三段回退收敛在单点，`open` / `open-path` / `diff` 三命令共用。

### Web API

- `GET /api/opener/defaults` — 读默认映射（含失效标记，供前端展示）。
- `POST /api/opener/default/save` — body: role + opener（校验同上）。
- `POST /api/opener/default/delete` — body: role。

### 前端

- 设置页 Opener 分区增加「默认」展示与编辑（每个 role 一行，下拉选 opener，可选「无默认」）。
- `opener/open`、`project/open` 调用处：无 opener 参数时先查默认（读 defaults 接口或 list DTO 附带），命中直接打开，未命中/失效走现有全量下拉。

## 范围外（留给 1039）

- 按上下文（文件后缀 / 项目语言 / 仓库根 vs 普通目录）的条件路由——见 parked 提案 `1039-opener适用范围match`。本提案的 role 级默认是它的兜底层。

## 验证

- 纯函数 + settings 读写：表驱动测试（默认解析、失效回退、写侧校验）。
- CLI 回退链：手动验证三命令的 `-o` > 默认 > 交互。
