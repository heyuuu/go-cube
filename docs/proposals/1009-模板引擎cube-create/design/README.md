# 模板引擎设计（结论综述）

> 本目录记录「cube create」模板引擎的设计讨论与最终协议。
>
> - `README.md`（本文）：**结论综述**，给人快速阅读，只讲「是什么、怎么用」。
> - `discussion.md`：**讨论详情**，给 AI 理解上下文，记录「为什么这么定、排除了什么」。
> - `glob-rules.md`：**glob 规则细节**，明确协议采用的通配符语义。
>
> 与 [`docs/project-template-spec.md`](../../../misc/project-template-spec.md) 的区别：那份是**模板内容**（某一套技术栈的目录结构、分层规范）；本目录是**引擎协议**（引擎和模板之间的契约），是更上层的元规则。两者互补。

---

## 一、定位

模板引擎是 cube 的一个子系统，对外暴露 `cube create` 命令。

核心思想一句话：**引擎是「数据驱动的复制 + 替换 + 执行器」，模板是数据。**

引擎对技术栈、目录结构、变量含义一无所知。所有「策略」由模板通过协议声明，引擎只管「机制」。这样换技术栈、改目录结构都只动模板，引擎不变。

---

## 二、调用方式

```bash
cube create <模板来源> [模板名] <目标路径> [--key=value ...]
```

- **模板来源（二选一，不内置任何模板）**：
  - 本地目录路径：`cube create ~/templates/go-service my-app`
  - git 仓库地址：`cube create https://github.com/xxx/tpl-repo go-service my-app`
- 模板名：来源是**模板集**（目录下无 template.yaml、一级子目录各有）时用于选子模板；来源本身就是模板时可省略；来源是模板集但没给模板名 → 交互式列出可选模板让用户选。
- 目标路径：生成的项目位置。
- 变量：命令行参数优先；缺的交互式提问补齐。

参数和交互都支持：`cube create ~/templates/go-service my-app --author=heyu`，缺的变量会逐个问。

**为什么不内置**：内置模板改一次要重新 build/install 二进制，迭代太重。外置让模板独立演进，且引擎统一面向「一个模板目录」——本地目录直接用，git 仓库 clone 到临时目录再用，对引擎核心透明。

**目录判定规则**：根目录有 `template.yaml` → 单模板；没有但一级子目录有 → 模板集（只扫一级，不递归）；两者都不是 → 报错。

---

## 三、协议（template.yaml）

模板根目录放一个 `template.yaml`，这是引擎和模板之间的**唯一契约**。三个顶层 key：

```yaml
# 1. 变量声明：引擎据此收集输入
variables:
  project-name:
    prompt: 项目名
    required: true
  author:
    prompt: 作者
    default: heyu

# 2. 替换规则：对匹配 glob 的文件，路径+内容统一应用
patterns:
  "**/*.go":
    - pattern: __MODULE__
      replace: github.com/${author}/${project-name}
    - pattern: __PROJECT__
      replace: ${project-name}
  "**/*.ts":
    - pattern: __PROJECT__
      replace: ${project-name}

# 3. 初始化动作：文件生成后执行（命令里的变量也替换）
init:
  - git init -b master
  - cd ${project-name} && go mod tidy
```

### 三个 key 各自的职责

| key | 方向 | 职责 |
|---|---|---|
| `variables` | 模板 → 引擎 | 声明「需要哪些输入」，引擎据此交互提问 / 取命令行参数 |
| `patterns` | 引擎读 | 声明「对哪些文件做哪些替换」，按 glob 分组，每组一组替换规则 |
| `init` | 引擎执行 | 声明「生成后跑哪些命令」，逐条执行，任一失败中止 |

---

## 四、关键设计决定

### 4.1 占位符由模板自定义，引擎不认识任何固定占位符

模板文件里写 `__MODULE__`、`@@PROJECT@@` 还是 `%%%NAME%%%`，完全由模板作者自己定。引擎只认识 `template.yaml` 里声明的 `pattern → replace` 规则。

→ **结果：占位符语法的争论（`{{}}` vs `__XX__` vs `${}`）在引擎层面消失了**，因为引擎不关心。

### 4.2 `${var}` 只在 template.yaml 内生效

`template.yaml` 里用 `${project-name}` 引用收集到的变量值，这只是一种「yaml 内引用变量」的写法，**不泄漏到模板文件里**。模板文件里该用什么占位符，由 `patterns` 的 `pattern` 字段定义。

### 4.3 精确字符串替换，不用正则

`pattern` 是精确字符串，100% 匹配才替换。避免正则的 escape 问题。如果将来需要正则，再加可选字段 `regex: true`。

### 4.4 glob 匹配到的文件，路径（含文件名）+ 内容统一替换

同一条替换规则同时作用于「文件路径」和「文件内容」。这样目录名 `__PROJECT__/main.go` → `my-app/main.go` 自动处理，不用单独的 rename 机制。

### 4.5 不做条件裁剪，差异靠多模板

不搞「要不要 Docker / 要不要 CI」的条件开关（`when` 字段）。需要差异就维护不同模板（`go-service-minimal` / `go-service-full`）。引擎更简单。

### 4.6 不做变量派生（camelCase 等）

引擎只拿原始值。需要 `projectName` / `ProjectName` / `project_name` 等变体，模板自己多声明几个变量让用户输入，或在 init 脚本里算。协议不背派生规则。

### 4.7 引擎无隐藏行为

引擎只做「复制 + 替换 + 跑 init 声明」。不自动 `git init`（交给 init 声明）、不自动装依赖（交给 init 声明）。所有初始化行为都显式写在模板里。

---

## 五、替换规则的数据结构

`patterns` 按 glob 分组，每个 glob 下挂一组替换规则：

```yaml
patterns:
  "<glob>":           # glob 作为分组键
    - pattern: <精确字符串>
      replace: "<替换值，可含 ${var}>"
    - pattern: <精确字符串>
      replace: "..."
```

之所以按 glob 分组（而不是每条规则各自带 glob）：现实中「同一类文件往往有多个占位符」，分组后 glob 只写一次，更贴合心智模型。

---

## 六、引擎固定流程

```
cube create <模板来源> [模板名] <目标路径> [--key=value ...]

1. 解析模板来源：
   a. 本地目录 → 直接用；git url → clone --depth 1 到临时目录（不缓存，用完删；遍历时跳过 .git）
   b. 判定目录类型：根目录有 template.yaml → 单模板；否则扫一级子目录构成模板集
   c. 模板集：给了模板名 → 校验并选中（不存在则报错并列出可用名）；没给 → 交互选择
2. 读 template.yaml
2. 收集变量：命令行参数优先，缺的交互式提问（用 variables 的 prompt）
3. 对 template.yaml 做变量插值（${var} → 实际值）
4. 遍历模板目录所有文件：
   a. 找到匹配该文件路径的所有 glob 分组
   b. 对路径（含文件名）应用这些分组的所有替换规则
   c. 复制文件到目标位置（用替换后的路径）
   d. 对文件内容应用同样的替换规则
5. 执行 init 命令列表（每条先做变量插值，逐条执行，任一失败中止）
6. 打印「下一步」提示
```

---

## 七、待定 / 留白

以下暂不纳入协议，将来按需再加：

- **变量派生**：自动生成 camelCase / PascalCase 变体。先不做，痛点出现再说。
- **条件执行**：init 命令带 `when` 条件。先不做，用多模板解决。
- **失败处理策略**：`on_fail: continue/abort`。默认中止，够用。
- **协议文件格式**：YAML。将来若嫌 yaml 库重，可换 TOML 或自定义格式。

已定（原留白，后补决策）：

- **模板来源：不内置，全部外置**（本地目录 / git 仓库二选一），git 来源 clone 到临时目录、**不缓存**。支持一个 git 仓库 / 目录内放**多个模板**（模板集，一级子目录各含 template.yaml），用可选位置参数「模板名」选择，缺省交互选择。见 discussion.md 9 节。
- **开发顺序**：① 本地目录单模板 → ② 本地目录模板集 → ③ git 来源（clone 到临时目录后按 ①/② 执行，遍历跳过 `.git`）。
