// Package workbench 是工作台领域包（提案 docs/proposals/1010-workbench基座）。
// 工作台以任意本机 git 目录为输入（不依赖 project scan、不走 gitcache），
// 信息全部直接调 git 获取（util/git 读能力），实时性由前端缓存控制。
// 分层注意：本包不 import project 包，git 读能力沉淀在 util/git。
package workbench

import (
	"context"
	"sync"
)

// Service 工作台领域服务。git 读路径无状态（每次调用直接调 git）；
// 唯一的运行期状态是 PTY 会话注册表（server 停机时统一回收，见 pty.go）。
type Service struct {
	ptyMu      sync.Mutex
	ptySeq     int
	ptyCancels map[int]context.CancelFunc
}

func NewService() *Service {
	return &Service{ptyCancels: map[int]context.CancelFunc{}}
}
