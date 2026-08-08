package easycache

import (
	"sync"
)

type Item[T any] struct {
	lock    sync.RWMutex
	hasData bool
	data    T
	loader  func() T
}

func NewItem[T any](loader func() T) *Item[T] {
	if loader == nil {
		panic("nil loader")
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
}

// Reload 强制加载，无论是否有缓存都重新获取（写锁）
func (c *Item[T]) Reload() T {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.data = c.loader()
	c.hasData = true
	return c.data
}
