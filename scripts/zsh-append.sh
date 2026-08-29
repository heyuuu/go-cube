
# ---- cube shell 扩展（由 make install 追加，勿手改；source 本文件后可用） ----

# p <args>：以本目录模式跑 cube（等价 cube <args> --local，query 缺省以 cwd 定位项目），
# 交互手敲的形态；脚本/Alfred 等非 shell 上下文仍直接用 cube --local
p() {
	cube "$@" --local
}

# pz <query>：模糊选项目并 cd 到其打开目标（主目录/worktree/workspace）
# cube path 的 TUI 画在 /dev/tty，stdout 只有干净的一行路径，可安全被 $( ) 捕获；
# 取消/非 TTY 多目标时 cube 以非零码退出，此处直接返回不 cd
pz() {
	local dir
	dir=$(cube path "$@") || return
	[[ -n $dir ]] && cd -- "$dir"
}
