package app

// service 生命周期钩子（均为可选实现，调度处用类型断言，未实现即跳过）：
// - OnServerStart: 常驻 server 启动后台任务时调用（CLI 短命进程不调用）。
// - OnServerStop: server 退出停止后台任务时调用。

type serverStartHook interface {
	OnServerStart()
}

type serverStopHook interface {
	OnServerStop()
}
