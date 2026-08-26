# opener 改造：icon + web 形态 + settings.json 与 Web 配置

> **状态**：✅ 已交付（2026-08-26 验收归档；实施定稿与偏差见文末「实施偏差备忘」）

## 背景与目标

opener 现状是纯「exec 命令模板」（`Opener{name, cmd, roles, slotCount}`，来自 config.json 的 openers 节），Web UI 出现后暴露三个问题：

1. **无 icon**：前端只能硬编码图标（`web/src/pages/projects/shared.tsx` 按 `finder`/`stree` 名字写死 lucide 图标）。
2. **形态表达不了**：新的打开方式不是启动子进程——「打开 cube 工作台页面」也应是一种文件夹打开方式；`{name, cmd, roles}` 结构表达不了。
3. **config.json 配置方式到顶了**：需要 Web 配置界面做增删改。把 opener 数据从 config.json 迁到**独立的 settings.json**（与 config.json 切割：无需启动时加载、可在运行中变更的用户可管理数据），Service **每次直接读文件、不做内存缓存**（openers 个数个位数，本地文件毫秒级；Web 改完立刻生效，CLI/Web 永远一致，省掉缓存失效逻辑）。settings.json 大部分时间由程序读写，相当于显化的一张表，且天然可 diff / 跨环境人工合并（曾评估落 sqlite 表，因跨环境人工合并困难而放弃，见 [1026-环境分离](../1026-环境分离/README.md)）。

**本提案不做**：非 macOS 的 app 图标提取（darwin-only，其他平台留 TODO）；workbench 以外的 web target（target 是白名单枚举，后续加值是纯加法）；不做打开方式使用统计增强。

## 已收敛的设计决策

### 1. Opener 接口化：exec / web 两个实现

与其在 struct 里放 type 字段再 switch，不如直接接口 + 多实现：

```go
type Opener interface {
    Name() string
    Roles() []Role
    Icon() Icon
    Open(role Role, slotArgs ...string) error
}
```

- `execOpener`：持 cmd 模板 + Executor（现状逻辑全部平移进来，占位符校验、slotCount 推导收进它的构造函数 `InitXxxOpener`）。
- `webOpener`：持 target（白名单枚举，v1 只有 `workbench`）。CLI 场景打开浏览器到 server 工作台页（path 带项目）；Web 场景前端识别后直接路由跳转，不发 open 请求。
- `Open` 带 role 参数：调用方本来就知道自己要的 role（现在也是先 RoleOpeners 过滤再 Open），实现方可按 role 细分派（如 web 类型未来 open-file 跳别的页面），把「Open 后再由调用方核对 HasRole」的前置校验移进实现内。
- role 体系（open-dir/open-file/diff-dir/diff-file）与 slot 语义（`$0`= 主路径）完全不动，web 类型吃同一套。

### 2. icon：判别「前端怎么渲染」，不管图片来源

```jsonc
{ "type": "lucide", "value": "folder-open" }   // 前端按名渲染 lucide-react 图标
{ "type": "image", "value": "<base64 png>" }   // 前端直接渲染
```

- 从本地 .app 提取只是**配置页选图的一种辅助入口**（与上传图片并列），提取后缩放到 64px 存 base64。提取是 darwin-only 纯函数，放能力层（如 `util/iconkit`：.app → icns → PNG bytes）。
- 前端 lucide 按名动态渲染，同时干掉 shared.tsx 的 finder/stree 硬编码。

### 3. settings.json + 迁移 + 直读文件

- opener 数据落到配置目录下的 `settings.json`（openers 节），不引入 sqlite / gorm model / AutoMigrate——sqlite 回归到只放 history。
- **实施定稿：独立 `settings` 包（与 config 平级）+ 节级 API**（`LoadSection` / `SaveSection` / `HasSection`，函数 + file string 形态，无 Store 对象、无接口——测试用 testfixture 真文件即可，不 mock）。API 粒度是**节**而非整文档：调用方（opener.Service）从头到尾拿不到完整 Doc，杜绝「读整文档→改→写回」覆盖其他节的 lost update，多节是立项前提。节名常量归域包（`opener.settingsSection`）。
- **校验收敛在读写边界**：读时逐条走领域构造函数（parseRoles / icon 解析，失败按现有「单条跳过不阻断」降级）；写时（save API → `Service.SaveOpener`）先走领域构造函数校验，失败中文错误，落不了文件。
- **写入原子**：包级锁内「读全文档→仅换该节→临时文件 + rename」；坏 JSON 拒写（防静默覆盖手工搞坏的其他节）；读一律降级（文件不存在/坏 JSON/节缺失/节坏→空值 + 日志，读路径永不阻塞）。
- **单写者假设**：每个环境一个 server，写路径都走它的 API；CLI 直接手改 settings.json 属人工干预，先不做 flock。
- **迁移偏差：不做代码迁移**——一次性迁移改为手动执行（jq 抽 openers 节到 settings.json + 删 config 节）；config 包已删 `Openers` 字段与 `OpenerConfig` DTO（残留旧键被 json 静默忽略）。
- Service 的 `AllOpeners()`/`FindByName()` 等每次读 settings.json。
- **透传措辞修正**：非目标节为「语义级透传」（JSON 值不变），非字节级——`json.Unmarshal` 落 `RawMessage` 会压缩空白，存盘统一缩进，首次写入会规范化其他节格式。

### 4. Web API 与前端配置页

- 按规则 15 只用 GET/POST：现有 `opener/list`、`opener/info`、`opener/open` + 新增 `opener/save`、`opener/delete`（DTO 加 icon 字段）。
- 前端配置页（现只读表格）升级为可编辑表单：name / roles / cmd 或 target / icon 选择器（选 lucide 图名、从 .app 提取、上传图片三个入口）。

## 实施顺序（每步独立可验收）

1. **settings.json + 迁移 + Service 改读文件**：行为等价重构，`opener_test.go` 大部分平移（config 构造换成 settings.json 构造）。
2. **Opener 接口化**：exec/web 两实现，cmd 层与 handler 无感。
3. **icon 字段**：领域类型 + settings.json 存储 + 提取 util（darwin）。
4. **Web CRUD API + 前端配置页**。
5. **收尾**：干掉前端 finder/stree 硬编码；更新 `docs/spec/现状.md` 3.3/3.5/6.2 与 web API 表。

## 实施偏差备忘（2026-08-26 回写）

- **web 形态最终拆除（owner 评审决策）**：曾实现 `webOpener`（type=web + target 白名单 + 领域内拼 URL），评审后拆除——CLI 场景它与 exec 同为子进程拼参数，领域内重造命令构造器不如组合 cube 自身 CLI：新增 `cube web workbench <path>` 命令承载 URL 知识（端口/路由/转义），opener 配置成 exec cmd `["cube","web","ui","$0"]` 即可；前端「在工作台打开」入口改为应用内路由跳转，独立于 opener 体系。随之删除接口 `Kind()`、Spec/DTO 的 type/target 字段。原提案的「Opener 接口化 exec/web 两实现」收敛为「接口 + 唯一 exec 实现」，接口保留是为后续形态留的纯加法扩展点。

- 接口面：`Open(role, slotArgs...)` + `Summary()`（跨实现统一展示串）/ `Kind()`（exec|web，前端据此前端路由跳转）；`RolesString` 是 `Roles()` 纯派生，降为包级函数不上接口。`Icon()` 按提案加入接口。
- icon 提取（`util/iconkit`）：不解析二进制 Info.plist，直接扫 `Contents/Resources/*.icns` 取最大文件；容器内只取 PNG magic 命中的条目（老式 ARGB 位图条目跳过）；nearest 邻采样缩放（不引 x/image）。
- Web API：`opener/open` 的 body 字段 `app` 改名 `opener`（术语统一，前端同步）；新增 `opener/save`、`opener/delete`、`opener/extract-icon`。
- 前端：配置页 openers 区块换 live 数据源（`opener/list`）+ Sheet 编辑表单；快捷图标按 icon 声明动态渲染（lucide 按名 kebab→Pascal 查 `icons` 映射 / image base64），`shared.tsx` 的 finder/stree 硬编码降级为 fallback。
- 测试侧顺带修复：testfixture `sanitizeName` 按 rune 截断（原按字节截中文测试名会切坏 UTF-8）。

## 验收标准

1. config.json 的 openers 自动迁移进 settings.json，迁移后 config.json 无 openers 节；既有 `open`/`open-path`/`diff`/`alfred opener-search` 行为不退化。
2. Web 配置页可增删改 opener，保存后无需重启即生效（CLI `cube openers` 同步可见）。
3. 可配置一个 type=web(workbench) 的 opener，在项目列表「打开」里与 exec opener 并列可用；CLI 下调用它打开浏览器工作台页。
4. opener 可配 icon（lucide 名 / .app 提取 / 上传图片），前端正确渲染；无 icon 时 fallback 默认图标。
5. 坏配置（未知 role、占位符越界、畸形 JSON）经 save API 返回中文错误，不写文件；文件内历史脏数据单条跳过不阻断启动。
