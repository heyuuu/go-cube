
# ---- cube jump（由 make install 追加，勿手改；source 本文件后可用） ----

# cj <query>：模糊选项目并 cd 到其打开目标（主目录/worktree/workspace）
# cube path 的 TUI 画在 /dev/tty，stdout 只有干净的一行路径，可安全被 $( ) 捕获；
# 取消/非 TTY 多目标时 cube 以非零码退出，此处直接返回不 cd
cj() {
	local p
	p=$(cube path "$@") || return
	[[ -n $p ]] && cd -- "$p"
}
