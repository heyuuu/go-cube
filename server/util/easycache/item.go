package easycache

import (
	"sync"
	"time"
)

type Item[T any] struct {
	lock       sync.RWMutex
	hasData    bool
	data       T
	computedAt time.Time // 最近一次 loader 计算完成的时间（零值 = 从未计算）；时间戳跟数据走，由缓存自身维护
	loader     func() T
}

// UpdatedAt 返回最近一次 loader 计算完成时间；从未计算过（含 Clear 后）返回零值。
// 使用方不得另设平行时间戳字段记录同一信息——靠约定同步是历史上多类不一致的根源。
// 未来 TTL 能力（计算后超时自动失效）也以此时间为基准，暂不实现。
func (c *Item[T]) UpdatedAt() time.Time {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.computedAt
}

func NewItem[T any](loader func() T) *Item[T] {
	if loader == nil {
		panic("loader 不能为 nil")
	}
	return &Item[T]{
		loader: loader,
	}
}

// Peek 直接读取缓存数据，不触发加载
// 如果无数据，返回零值和 false
func (c *Item[T]) Peek() (T, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.data, c.hasData
}

// Get 获取数据，不存在则调用 loader 加载（写锁）
func (c *Item[T]) Get() T {
	// 先尝试读锁快速路径
	if data, ok := c.Peek(); ok {
		return data
	}

	// 未命中，获取写锁加载
	c.lock.Lock()
	defer c.lock.Unlock()

	// 双重检查：可能在获取写锁期间已被其他 goroutine 加载
	if !c.hasData {
		c.data = c.loader()
		c.hasData = true
		c.computedAt = time.Now()
	}
	return c.data
}

// Clear 清空缓存数据（写锁）
func (c *Item[T]) Clear() {
	c.lock.Lock()
	defer c.lock.Unlock()

	var zero T
	c.data = zero
	c.hasData = false
	c.computedAt = time.Time{}
}

// Reload 强制加载，无论是否有缓存都重新获取（写锁）
func (c *Item[T]) Reload() T {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.data = c.loader()
	c.hasData = true
	c.computedAt = time.Now()
	return c.data
}
