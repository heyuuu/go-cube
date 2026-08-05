# Glob 规则细节

> 本文档定义模板引擎协议中 `patterns` 的 glob 通配符语义。
>
> **基准**：以 [doublestar](https://github.com/bmatcuk/doublestar)（Go 生态最成熟的 `**` 实现）语义为准，尽量兼容 [minimatch](https://github.com/isaacs/minimatch)（Node 生态）。下方列出已知不兼容点。

---

## 一、背景：glob 没有 RFC

glob 起源于 1970s Unix shell，是 POSIX 定义的 shell 机制之一。但：

- **POSIX 只定义了 `*` `?` `[...]`** 三种基础通配符，且明确把 `**`（递归匹配）排除在外。
- `**` 是后来 bash（globstar）、各库各自加的扩展，**行为不一致**。
- **没有任何 RFC 统一规定 glob 应该怎么匹配**，尤其 `**` 的语义。

因此协议不能引用「某个规范」，只能自行定义一个子集，并锚定一个具体实现（doublestar）作为行为基准。

---

## 二、Go 标准库 `filepath.Glob` 为什么不行

`filepath.Glob` / `filepath.Match` 支持的语法：

| 写法 | 含义 | filepath.Glob |
|---|---|---|
| `*` | 匹配**非路径分隔符**的任意字符序列（不跨 `/`） | ✅ |
| `?` | 单个非分隔符字符 | ✅ |
| `[abc]` | 字符集 | ✅ |
| `[^abc]` | 字符集取反（用 `^`） | ✅ |
| `[a-z]` | 字符范围 | ✅ |
| `**` | 递归跨目录匹配 | ❌ **不支持**（当作两个 `*`） |
| `{a,b}` | 花括号分组 | ❌ **不支持** |

**致命缺陷**：`**` 不支持。模板引擎最核心的需求「递归匹配所有 `.go` 文件」做不到。这是 Go 生态的工具（Hugo、代码生成器等）都用 doublestar 而非标准库的原因。

→ **协议不引用 filepath.Glob，实现用 doublestar 库。**

---

## 三、协议采用的 glob 子集

以下通配符由本协议定义，以 doublestar 语义为基准：

| 写法 | 含义 | 示例 |
|---|---|---|
| `*` | 匹配除路径分隔符 `/` 外的任意字符序列（单层） | `src/*.go` 匹配 `src/a.go`，不匹配 `src/nested/b.go` |
| `**` | 匹配**零或多层**目录（跨 `/`） | `**/*.go` 匹配所有层级的 `.go`；`src/**/a.go` 匹配 `src/a.go`（零层）和 `src/x/a.go` |
| `?` | 匹配单个字符（非 `/`） | `???.go` 匹配 `abc.go` |
| `[abc]` | 字符集，匹配其中任一字符 | `[abc].go` 匹配 `a.go`/`b.go`/`c.go` |
| `[^abc]` | 字符集取反 | `[^x].go` 匹配 `a.go`，不匹配 `x.go` |
| `[a-z]` | 字符范围 | `[a-z].go` 匹配 `a.go`~`z.go` |
| `{a,b}` | 分组，匹配 `a` 或 `b`（花括号展开） | `**/*.{go,ts}` 匹配所有 `.go` 和 `.ts` |

---

## 四、必须明确的边界行为（争议点）

这些是各实现分歧最大的地方，本协议明确如下：

### 4.1 `**` 匹配零层目录

```
src/**/a.go  能匹配  src/a.go       （中间零层）✅
             能匹配  src/x/a.go     （中间一层）✅
             能匹配  src/x/y/a.go   （中间多层）✅
```

> ⚠️ bash 严格 globstar 下 `src/**/a.go` 要求至少一层目录（不匹配 `src/a.go`）。本协议采用 doublestar 语义：**零层也算**。

### 4.2 默认匹配 dotfile（`.` 开头的文件）

```
**/*.go  会匹配  .hidden.go   ✅（默认匹配）
```

> ⚠️ minimatch 默认**不**匹配 dotfile，需 `{dot:true}` 才匹配。本协议采用 doublestar 语义：**默认匹配**。如需排除 dotfile，请在 `template.yaml` 里单独处理（如 gitignore 模板文件本身）。

### 4.3 取反语法用 `^`

```
[^abc]   ← 本协议采用（与 doublestar、POSIX 正则一致）
[!abc]   ← bash 风格，本协议不采用
```

> ⚠️ bash 用 `[!abc]`，minimatch 两种都支持。本协议统一用 `[^abc]`。

### 4.4 路径分隔符统一用 `/`

```
src/**/*.go       ← 协议里一律写 /
```

引擎内部处理平台差异（Windows 下 `/` 与 `\` 互转）。模板作者只需用 `/`。

### 4.5 全路径匹配

glob 必须匹配**相对于模板根的完整路径**，不是子串。

```
模板结构:
  template/
    src/a.go
    src/nested/b.go

glob: src/*.go        → 只匹配 src/a.go
glob: src/**/*.go     → 匹配 src/a.go 和 src/nested/b.go
glob: **/*.go         → 匹配所有 .go
```

---

## 五、与 minimatch 的不兼容点汇总

minimatch 是 Node 生态（npm/pnpm）的 glob 实现。本协议以 doublestar 为准，以下是与 minimatch 的已知差异：

| 行为 | 本协议（= doublestar） | minimatch 默认 |
|---|---|---|
| `**` 零层匹配 | ✅ `src/**/a.go` 匹配 `src/a.go` | ✅ 一致 |
| dotfile 默认匹配 | ✅ 默认匹配 `.hidden.go` | ❌ 默认不匹配，需 `{dot:true}` |
| 取反语法 | `[^abc]` | `[^abc]` 和 `[!abc]` 都支持 |
| 花括号 `{a,b}` | ✅ | ✅ 一致 |

**实践影响**：如果模板只在 Go + doublestar 环境下使用，无需关心 minimatch。如果模板可能跨工具复用（比如同一套模板既给 cube 用，又给某个 Node 脚手架用），注意 dotfile 行为差异。

---

## 六、glob 写法速查（模板作者参考）

```
匹配所有 .go 文件（任意层级）        **/*.go
匹配 src 下所有 .go（含子目录）       src/**/*.go
匹配多种扩展名                       **/*.{go,ts,tsx}
匹配 src 直接子文件                   src/*
匹配特定前缀                          **/test_*.go
排除某目录（需配合多条规则或在引擎层处理）  无原生排除语法
```

> 注意：本协议的 glob **不支持 `!` 排除语法**（如 `!**/vendor/**`）。如需排除，通过拆分多条正向 glob 实现，或在引擎未来版本加 exclude 字段。

---

## 七、实现建议（Go）

协议的 Go 实现推荐用 `github.com/bmatcuk/doublestar/v4`：

```go
import "github.com/bmatcuk/doublestar/v4"

// 匹配：判断文件路径是否命中 glob
matched, err := doublestar.Match("src/**/*.go", "src/nested/a.go")
// → matched = true

// 遍历：直接用 doublestar.Glob(walk in FS) 或自己 Walk + Match
```

doublestar v4 的行为与本协议定义的子集一致（`**` 零层、dotfile 默认匹配、`[^abc]` 取反）。
