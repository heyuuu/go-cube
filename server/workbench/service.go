// Package workbench 是工作台领域包（提案 docs/proposals/1010-workbench基座）。
// 工作台以任意本机 git 目录为输入（不依赖 project scan、不走 gitcache），
// 信息全部直接调 git 获取（util/git 读能力），实时性由前端缓存控制。
// 分层注意：本包不 import project 包，git 读能力沉淀在 util/git。
package workbench

// Service 工作台领域服务。纯读、无状态、无后台任务——每次调用直接调 git。
type Service struct{}

func NewService() *Service { return &Service{} }
