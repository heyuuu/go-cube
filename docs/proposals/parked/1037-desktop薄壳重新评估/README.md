# desktop 薄壳重新评估

> **状态**：⏸️ 挂起（选型讨论已完成，倾向定为 **Swift 原生通用参数化壳**；不启动——Swift/SwiftUI 个人工具线尚未推进到成熟，解挂主条件见文末）

## 与 1028 的关系

[1028-desktop壳与wails评估](../1028-desktop壳与wails评估/)（挂起，结论「不做」）的立论前提是「cube 没有添加 desktop 壳的强需求——开 web 已可用，壳的增量均为锦上添花」。**本轮出现了推翻该前提的新事实**（见下），壳的需求成立；但 1028 关于 wails bindings 模型与 cube「本地 server + 通用 HTTP API」结构性不匹配的推理仍然有效——薄壳恰好绕开了那个矛盾（见「薄壳形态」），可视为对 1028 的「解挂条件」部分回应，但壳的具体实现路线尚未定案。

## 新事实（为什么重新评估）

1. **快捷键冲突**：工作台在浏览器中运行，页面永远抢不到浏览器保留快捷键（`Cmd+W` / `Cmd+数字` 等），且快捷键规划越多冲突面越大——这条路在浏览器形态下是封死的。原生窗口（webview）中键盘事件先到 webview，冲突面大幅缩小。这是壳的**硬需求**，非锦上添花。
2. **终端保持**：曾考虑「关闭工作台后 pty 不释放」需要壳提供「彻底杀后台」的宿主。**该问题已通过超时回收独立解决**，不再构成壳的论据；壳维持薄壳语义即可，不接管 server 生命周期（server 仍由 launchd 常驻，见 1036）。

## 前提约束（决策边界）

- **自用**：不考虑分发、签名公证、跨用户兼容等。
- **macOS 独占**：不考虑跨平台，可使用平台原生能力（WKWebView、Carbon/Cocoa 快捷键 API）。
- **薄壳**：壳 = 原生窗口加载 `http://127.0.0.1:<port>`（连已有 cube server），前端代码零改动、server 生命周期不变、launchd/CLI/alfred 全部照旧。**不做厚壳**（壳拥有 server 进程 / 嵌入前端 / bindings 代理）。

## 需求清单

1. 原生窗口承载现有 web UI（WKWebView 即可），快捷键不再与浏览器冲突。
2. **全局快捷键**：系统级（app 不在前台也触发），用于全局呼出——这是 v2/v3 选型的分水岭能力（见下）。
3. **多窗口**（可能需要，优先级次之）：工作台多实例或主窗 + 辅窗。
4. `cube ui` 改为默认启动壳，**保留浏览器打开方式**（浏览器 debug 更方便，如 `--browser` 旗标分流）。

## wails 评估（2026-08 事实）

- 版本现状：v2 稳定但停止演进；v3 beta（beta.10，官方标注 "beta software with a stable desktop API"）。
- **全局快捷键与多窗口均为 v3 独有**：v2 单窗口是架构性限制（声明式 API，团队明确不为 v2 加）；v2 内置全局快捷键的 issue 挂起未做。选 v2 = 两个核心需求直接落空；选 v3 = 接受 beta。
- 薄壳的 Wails API 消费面极小（窗口 + external URL + 全局快捷键），本是消费 beta 的低风险姿势，但有未验证点：**Wails 主路径是嵌入资产 + bindings，直接加载 external URL（尤其 WebSocket）不是一等公民**，v3 也需 spike 验证行为。
- **保留意见（本轮倾向）**：1028 已论证 bindings 模型与 cube 互斥，薄壳虽然绕开了 bindings，但意味着引入一整个胶水框架只用其 5% 功能，且 v3 beta 期 API 仍在动。与「自用 + macOS 独占 + 薄壳」的前提组合下，**Swift 原生（WKWebView + 少量 AppKit）可能更匹配**——恰好也是 1028 给个人 Mac 工具线定的投入方向（Swift/SwiftUI，平台级技能可迁移）。

## 候选壳路线（待对比，本文不决策）

| 路线 | 一句话画像 | 主要待验证/顾虑点 |
| --- | --- | --- |
| Swift 原生（WKWebView + AppKit） | 零框架，窗口 + 全局快捷键（`MASShortcut`/`HotKey` 或 Carbon `RegisterEventHotKey`）都是成熟原生能力；与 1028 定的个人工具线方向一致 | 需引入 Swift 构建与 Xcode 工具链，`cube ui` 要跨语言 spawn |
| Wails v3（beta） | Go 同语言、生态内文档多；全局快捷键/多窗口内置 | beta 风险、external URL + WebSocket 行为未验证、只用 5% 功能 |
| 零框架 Go webview（`webview/webview_go`） | 最薄，一个函数开窗 | 无全局快捷键/多窗口/菜单，能力天花板低 |
| PWA / 用户脚本方案 | 不真正脱离浏览器 | 快捷键冲突仍在，仅作对照，基本排除 |

## 对比讨论结论（2026-08-29）

- **Tauri 补入对比**：薄壳形态下加载 external URL / 全局快捷键 / 多窗口都可行，但要拖进 Rust + Node 双工具链，且其独占卖点（跨平台）在本前提（macOS 独占）下价值为零——账与 wails 同构（为一扇窗拖一整套框架）。Electron 直接出局（重且无独占能力）。排序：**Swift 原生 > Tauri ≈ Wails v3 > Electron**。
- **Swift 命令线工具链已确认可行**：SPM（`swift build`）纯命令行编译 AppKit 程序，全程无需 Xcode GUI；`.app` bundle（LSUIElement 等）用十几行脚本手拼 + `codesign --sign -` ad-hoc 签名即可进 make/CI。
- **倾向形态：通用参数化壳**——壳做成零 cube 逻辑的通用程序（启动参数 `--url --hotkey`，或读极简配置：url / hotkey / 窗口尺寸），职责仅三件事：注册全局快捷键、单例激活（已开 `NSApp.activate` 不重开）、WKWebView 窗口加载 url。**壳与 cube 彻底解耦**：壳是独立仓库的通用工具，任何本地 web 服务可复用（与 1028 定的个人 Swift 工具线合流）；cube 侧改动收敛为 `cube ui` 加 spawn 分支（保留浏览器形态分流）。Swift 原生在各候选中无短板项：全局快捷键（Carbon `RegisterEventHotKey` 系，成熟）与多窗口（`NSWindow`）皆原生直取，external URL 是 WKWebView 一等公民。

## 解挂条件

- **主条件：Swift/SwiftUI 个人工具线推进到成熟**（对 AppKit 窗口/快捷键/打包链路有实际手感）——壳本体不依赖 cube 任何进展，工具线成熟即可动手；
- 次条件（可并行观察）：若期间 Wails v3 转正且 external URL 路径被官方一等公民化，可重新对比一次，但不改变当前倾向；
- 若最终结论仍是「不做壳」，则本文并入 1028 作为其增补事实（快捷键冲突论据 + 薄壳形态分析），仅保留提案不删。
