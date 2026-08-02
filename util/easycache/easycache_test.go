package easycache

import (
	"sync"
	"testing"
)

func TestItem_GetLoadsOnce(t *testing.T) {
	calls := 0
	item := NewItem(func() int { calls++; return 42 })

	// 首次 Get 触发 loader
	if v := item.Get(); v != 42 {
		t.Fatalf("Get 返回 %d，期望 42", v)
	}
	if calls != 1 {
		t.Fatalf("loader 应调用 1 次，实际 %d", calls)
	}
	// 二次 Get 不触发 loader（命中缓存）
	if v := item.Get(); v != 42 {
		t.Fatalf("二次 Get 返回 %d，期望 42", v)
	}
	if calls != 1 {
		t.Fatalf("缓存命中后 loader 仍被调用，总次数 %d", calls)
	}
}

func TestItem_Peek(t *testing.T) {
	calls := 0
	item := NewItem(func() string { calls++; return "x" })

	// 未加载前 Peek 不触发 loader，返回零值+false
	if v, ok := item.Peek(); ok || v != "" {
		t.Fatalf("未加载 Peek 应返回零值+false，实际 v=%q ok=%v", v, ok)
	}
	if calls != 0 {
		t.Fatalf("Peek 不应触发 loader，实际调用 %d 次", calls)
	}
	// 加载后 Peek 返回数据
	item.Get()
	if v, ok := item.Peek(); !ok || v != "x" {
		t.Fatalf("加载后 Peek 应返回 x+true，实际 v=%q ok=%v", v, ok)
	}
}

func TestItem_Clear(t *testing.T) {
	item := NewItem(func() int { return 7 })
	item.Get()
	item.Clear()

	if _, ok := item.Peek(); ok {
		t.Fatalf("Clear 后 Peek 应为未命中")
	}
	// 再次 Get 触发 loader
	calls := 0
	item.loader = func() int { calls++; return 8 }
	if v := item.Get(); v != 8 {
		t.Fatalf("Clear 后 Get 返回 %d，期望 8", v)
	}
	if calls != 1 {
		t.Fatalf("Clear 后 Get 应触发新 loader，调用 %d 次", calls)
	}
}

func TestItem_Reload(t *testing.T) {
	val := 1
	item := NewItem(func() int { val++; return val })

	// 首次 Get
	if v := item.Get(); v != 2 {
		t.Fatalf("首次 Get 返回 %d，期望 2", v)
	}
	// Peek 命中
	if v, _ := item.Peek(); v != 2 {
		t.Fatalf("Peek 返回 %d，期望 2", v)
	}
	// Reload 强制刷新
	if v := item.Reload(); v != 3 {
		t.Fatalf("Reload 返回 %d，期望 3", v)
	}
}

func TestNewItem_PanicsOnNilLoader(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("nil loader 应 panic")
		}
	}()
	NewItem[int](nil)
}

func TestItem_ConcurrentGetLoadsOnce(t *testing.T) {
	// 并发 Get 只应触发一次 loader（写锁 + 双重检查）
	calls := 0
	var mu sync.Mutex
	item := NewItem(func() int {
		mu.Lock()
		calls++
		mu.Unlock()
		return 99
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v := item.Get(); v != 99 {
				t.Errorf("并发 Get 返回 %d", v)
			}
		}()
	}
	wg.Wait()

	if calls != 1 {
		t.Fatalf("并发 Get 下 loader 应仅调用 1 次，实际 %d", calls)
	}
}
