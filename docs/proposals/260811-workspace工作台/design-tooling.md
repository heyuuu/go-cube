# 工作台设计工具与「设计稿即契约」讨论纪要（260818）

> 本文件是 workspace 工作台设计启动前的**工具选型与方法论讨论**完整沉淀，供后续对话直接续接。
> 背景：工作台页面要开始设计了。核心约束：**用户要亲自主导布局设计（直接让 AI 做会偏离想象），但想法尚未定型，主要纠结页面布局**。

## 一、需求方的关键诉求（讨论中逐步澄清）

1. **用户亲手设计**，AI 配合——不能全交给 AI 生成
2. 样式必须与项目协调：新前端是 **base-mira 风味 shadcn（Base UI，非 Radix）**，最怕设计工具把自带样式带进来
3. 很多时候**只想要布局**，不要求高保真样式
4. 需要**交互原型**：点击展开、点击跳转页面——比样式更优先
5. 可能想 **fork 一个开源工具深度定制**：要求快速读懂源码核心、快速剪除不需要的功能、扩展自定义格式（如把原型连线存进文件让 AI 能读到）
6. 长期愿景：**设计稿成为契约**——渐进式同步（工具里改 → AI 感知变化位置 → 更新代码）+ 自动验证生成页面的合规性（E2E）

## 二、候选工具全景与结论演变

### 2.1 初期框架：按保真度递进选工具

- **Excalidraw / 纸笔拍照**：探索期草图，AI 可读图
- **ASCII 变体对比**：AI 出 2~3 个 ASCII 线框供挑选（AskUserQuestion preview），对「没想清楚」的场景比空白画布高效
- **工程内静态线框**：真实 Tailwind token + shadcn 组件搭无交互布局页，浏览器里指摘迭代
- Figma：明确不推荐（单页布局探索太重，AI 只能看导出图，往返成本高）

### 2.2 Pen.dev（原 Pencil，商业化改名）

- **形态**：`pen` CLI（`@pen.dev/cli`）是 headless 设计生成器：`pen --prompt "..." --out design.pen --export design.png`，内置云端 designer agent（默认 Claude Opus，规划布局→创建元素→视觉自检），一张稿 2~5 分钟
- **迭代**：`pen --in design.pen --prompt "把右栏拆两块" --out v2.pen`；`.pen` 是开放 JSON 格式、可进 git
- **参考图锚定**：`--prompt-file` 可附现有页面截图，让它在视觉上模仿项目风格
- **集成方式**：SKILL.md（非 MCP），放入 agent 技能目录即可；需登录 / `PEN_CLI_KEY`；**设计生成服务在云端**
- 安装：`npm install -g @pen.dev/cli`；技能文件 curl 到 `~/.agents/skills/pen-design/SKILL.md`

### 2.3 OpenDesign（nexu-io/open-design）

- **形态**：开源（3.4 万星）、本地优先（本地守护进程 + SQLite），以 **Skills + CLI + MCP Server** 注入 coding agent——不是画布，是给 agent 的设计引擎
- 自带 36 主题 / 31 页面布局 / 71 设计系统模板
- **优势**：设计产物由 agent 直接在仓库里生成 → 天然用项目组件，零样式污染
- **劣势**：设计质量 = agent 自己的能力，布局探索出多方案较慢；无独立画布

### 2.4 OpenPencil（open-pencil/open-pencil）——最终修正推荐

- **形态**：开源设计编辑器（Tauri ~7MB + Web PWA），旧版 Pencil 的开源延续，打开 `.pen` / `.fig`
- **CLI 全家桶**（与 AI 配合的核心）：
  ```bash
  openpencil import projects.html --tailwind "flex ..." -o layout.pen  # 现有页面 → 设计稿
  openpencil query layout.pen "//FRAME[@width < 300]"                   # XPath 查节点
  openpencil export layout.pen -f png -s 2                              # 出图给人看
  openpencil export layout.pen -f jsx --style tailwind                  # 导 JSX/Tailwind
  openpencil lint / analyze colors|typography / convert                 # 质检与转换
  ```
- **AI**：BYO 模型（OpenRouter/Anthropic/Z.ai 等）+ 90+ 节点操作工具；有 MCP server 与桌面 agent 集成
- **命中诉求的三点**：① GUI 画布用户可亲手拖拽，AI 经 CLI/MCP 操作同一文件（唯一支持人机同文件双向）② `import` 能把现有页面 HTML+Tailwind 导入成设计稿——样式协调从根上解决 ③ 全本地开源
- 安装：`brew install openpencil` 或 `npm install -g @open-pencil/cli`

### 2.5 结论演变（重要，避免重新纠结）

```
初判（信息不全）→ 推 Pen（布局出稿快）+ OpenDesign（样式协调）
openpencil README 补全后 → 修正为：工作台场景推 OpenPencil
  （用户亲手设计 + import 继承现有样式 + CLI/MCP 配合，三点全命中）
Pen 保留优势场景：完全不动手、AI 出 3 张稿挑一张
```

## 三、交互原型：三工具都不适合，结论是「工程内真原型」

- 查证结论：Pen / OpenPencil / OpenDesign 均无 prototype 交互连线能力（静态设计/代码工具）
- 传统交互原型是 Figma prototype mode / Penpot（开源）的主场，但**连线数据存在它们那边，AI 读不到**，验证不了「点 A 是否跳 B」
- **定论：交互逻辑验证用工程内真原型**——`web/src/pages/workbench-proto/`（一次性原型区，灰色线框 + mock 数据 + 真路由真跳转真展开折叠）。优点：交互 100% 真实、验证完骨架直接演进成实现、AI 可自动化验证交互链路
- 分工：**布局和视觉用设计工具，交互逻辑用真原型**——布局稿管「长什么样」，真原型管「怎么动」

## 四、深度定制 fork 的选型（若走改造路线）

| | 栈 | 快速读懂 | 快速剪除 | 自定义格式+AI 可见 |
|---|---|---|---|---|
| OpenPencil | Tauri+Vue+TS | 中 | 中 | 好 |
| Penpot | Clojure | 差（否决） | 差 | 有（自带 prototype）但改不动 |
| **tldraw** | React+TS SDK | 好 | 天生最小 | **最好**（自定义 shape+props 一等公民） |
| Excalidraw | React+TS | 好 | 中 | 好（MIT） |
| **react-flow(xyflow)** | React+TS | 极好 | 极好 | 连线（edge）是第一概念（MIT） |

- **推荐自建而非 fork**：约束「快速剪除不需要的功能」指向「本来就没什么可剪的东西」——tldraw 起点即最小画布内核；fork OpenPencil 要先读懂 .fig 兼容/向量编辑/协作等一堆不要的东西
- 前置分叉问题：**原型形态是「自由画布」还是「屏幕方块+箭头流程图」**——流程图选 react-flow（连线原生、核心一两天读完），自由画布选 tldraw（注意 source-available 许可：带水印免费用、商业去水印付费；MIT 备选 Excalidraw）
- 自建的核心优势：**连线=数据**（shape/edge 上存 `onTap → 跳 frame-3`），整图导出为完全掌控的 JSON，AI 读/写/校验无障碍；可长成 cube 子能力（数据进 sqlite、配 `cube proto` CLI）

## 五、设计稿即契约 + 渐进同步（业内调研）

### 路线 A：消灭第二事实源（同步问题不复存在）

**Onlook**（onlook-dev/onlook，开源）：code 是唯一事实源——画布渲染运行中的真 app，可视化改动直接写成仓库代码 diff；`git diff` 即「AI 感知变化位置」的答案。同类：Claude Code `/design-sync`。

**Onlook 机制拆解**（为什么「复杂源码也改得不错」）：
1. 跑项目 dev server，预览=应用实时渲染
2. 构建插件给 DOM 元素注入源码坐标（文件+AST 位置）——点选即知来源
3. **编辑分流**：展示层（色/距/字号/插入 div/拖拽重排）= **AST 级确定性 codemod**（改 Tailwind 类、动 JSX 节点，不经 AI 不改错语义）；语义改动（逻辑/数据流）= AI chat agent；git 分支 + checkpoints 兜底

**Onlook 真实数据 ≠ Storybook（相反哲学）**：Storybook 把组件摘出来 + 受控 fixtures；Onlook 整个 app 原地跑、数据该从哪来从哪来（对 cube = 真实 131 项目、真实 git 状态）。代价：后端不跑页面就空；组件边界状态不如 Storybook 方便。

**硬限制**：开源版仅支持 **Next.js + Tailwind**；cube 的 `web/` 是 Vite + React Router——**当前不可用**。结论：Onlook 思路值得借鉴（即「真原型」路线的成品化），工具本身进不了 cube 栈。

### 路线 B：设计稿即基线（契约验证）

- 视觉回归两流派：**build-to-build**（Chromatic/Percy，防代码漂移，不验设计符合度）vs **design-vs-implementation**（设计文件作基线比对实现页）
- 商业：Applitools for Figma、Sauce Labs Visual
- **开源代表：uimatch**（kosaki08/uimatch）——Figma REST API 拉设计帧 + Playwright 截实现页 + CI 结构化 diff 报告 + **实验性 AI 修复回路**（发现偏差→AI 自动改）。最接近「设计契约自动验证」的现成实现
- 结构层实践：Figma MCP + Playwright（agent 读设计生成断言）

### 关键洞察（本次讨论的原创综合）

**契约化的前提是设计稿是文本、在 git 里、结构化**。Figma 类天然不满足（二进制+云端），才有 REST API/MCP 补丁。自建 JSON 格式下三层契约全通：

```
design.proto.json（仓库里的结构化设计稿）
  ├─ 渐进同步：git diff 它 → AI 读 JSON diff → 定位组件 → 改代码（无翻译层）
  ├─ 结构契约：JSON → 自动生成 Playwright 断言（"工作台页应含侧栏，宽 < 300"）
  ├─ 交互契约：连线数据 → 自动生成 E2E 用例（点击 A → 断言路由到 B）
  │             ★ 自建格式独有：Figma 的 prototype 连线在 API 里是二等公民，做不到
  └─ 视觉契约：设计导出 PNG 作 Playwright 视觉基线（uimatch 模式）
```

## 六、当前状态与下一步选项

**尚未安装任何工具，未写任何代码。** 悬而未决：

1. **原型形态**（自由画布 vs 流程图）→ 决定 react-flow / tldraw / 或纯工程内真原型
2. 工作台首屏内容：一屏里同时看什么（文件树 / diff / 终端 / 分支图……）——用户尚未口述
3. 三条已提出、待用户点头的启动方案：
   - a) **工程内真原型**：`workbench-proto/` + mock 数据 + 真交互，浏览器走流程
   - b) **示例契约验证**：拿现有 projects 页写一份 `proto.json` + 自动生成 Playwright 用例，验证契约形态是否符合想象
   - c) **设计工具出稿**：装 OpenPencil（或 Pen）后，从用户口述/AI 出稿开始布局探索

## 附：相关上下文

- 工作台需求本体：[`README.md`](./README.md)（三聚合方向：代码阅读 / Git 操作 / 命令行 PTY；真 PTY 需 WebSocket 长连接是架构临界点）
- 代码阅读方向可参考已归档的 [md渲染提案](../../proposals/260811-md渲染/)（其依赖已全部满足，可先行落地作为工作台代码阅读方向的探路石）
- 新前端工程：`web/`（Vite+React19+React Compiler+tsgo+Tailwind4+Base UI shadcn+React Query），详见 `docs/spec/现状.md` 前端工程章节
- ZCode 技能目录：`~/.agents/skills/`（Pen SKILL.md 放这里）；ZCode 是 MCP client（可接 OpenDesign/OpenPencil MCP）
