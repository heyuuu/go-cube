// Package tui 提供常用的命令行交互与渲染能力。
//
// 内部基于 charm.land 系列库实现：
//   - 交互能力 (Select / MultiSelect / Confirm / Input / PasswordInput)
//     使用 charm.land/huh/v2，每次交互即一次表单运行，会临时接管终端；
//   - 渲染能力 (RenderTable) 使用 charm.land/lipgloss/v2 渲染为字符串，
//     由调用方自行输出。
//
// # 核心分类：TTY-only vs 环境无关
//
// 本包按「是否必须 TTY 才能运行」把 API 分成两类，调用环境要求天差地别：
//
//   - 交互函数 (TTY-only) ：Select / MultiSelect / Confirm / Input / PasswordInput。
//     必须在真实终端里运行——它们要接管终端 raw mode、读取按键。
//     非 TTY 环境（管道 echo x | prog、输入重定向 prog < file、CI、IDE 运行控制台）
//     下会直接返回 ErrNotTTY，绝不静默降级。这避免了 CI/脚本里
//     「无人输入却静默选了第一项」的隐患，降级策略交由调用方决定。
//
//   - 渲染函数（环境无关）：RenderTable。
//     纯字符串计算，零副作用——管道、CI、重定向里照常工作，
//     返回的字符串由调用方决定输出到哪里。
//
// # 命名约定
//
// 配合上面的分类，靠命名即可辨识，无需查文档：
//
//   - 名字以 Render 开头 → 纯渲染（返回单个 string，不碰终端，环境无关）；
//   - 其它公开函数     → 交互（需要 TTY，返回 (业务值, error)，
//     非 TTY 时返回 ErrNotTTY，用户中断时返回 ErrUserAborted）。
//
// 配置类辅助（TableOption / WithBorder 等）仅服务于渲染函数，本身不产生输出。
package tui
