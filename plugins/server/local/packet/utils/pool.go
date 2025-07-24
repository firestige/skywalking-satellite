package utils

import (
	"sync/atomic"
)

// PoolStats 池统计信息
type PoolStats struct {
	Hits   int64 // 池命中次数
	Misses int64 // 池未命中次数
	Puts   int64 // 放入池次数
	Drops  int64 // 丢弃次数
}

// GetStats 获取统计信息的副本
func (ps *PoolStats) GetStats() map[string]int64 {
	return map[string]int64{
		"hits":   atomic.LoadInt64(&ps.Hits),
		"misses": atomic.LoadInt64(&ps.Misses),
		"puts":   atomic.LoadInt64(&ps.Puts),
		"drops":  atomic.LoadInt64(&ps.Drops),
	}
}

// GetHitRate 获取命中率
func (ps *PoolStats) GetHitRate() float64 {
	hits := atomic.LoadInt64(&ps.Hits)
	misses := atomic.LoadInt64(&ps.Misses)
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}

// BoundedPool 有界对象池实现
type BoundedPool struct {
	pool    chan interface{}
	factory func() interface{}
	reset   func(interface{})
	stats   *PoolStats
}

// NewBoundedPool 创建有界对象池
func NewBoundedPool(size int, factory func() interface{}, reset func(interface{})) *BoundedPool {
	return &BoundedPool{
		pool:    make(chan interface{}, size),
		factory: factory,
		reset:   reset,
		stats:   &PoolStats{},
	}
}

// NewBoundedPoolWithStats 创建带统计信息的有界对象池
func NewBoundedPoolWithStats(size int, factory func() interface{}, reset func(interface{}), stats *PoolStats) *BoundedPool {
	return &BoundedPool{
		pool:    make(chan interface{}, size),
		factory: factory,
		reset:   reset,
		stats:   stats,
	}
}

// Get 从池中获取对象
func (p *BoundedPool) Get() interface{} {
	select {
	case obj := <-p.pool:
		if p.stats != nil {
			atomic.AddInt64(&p.stats.Hits, 1)
		}
		return obj
	default:
		if p.stats != nil {
			atomic.AddInt64(&p.stats.Misses, 1)
		}
		return p.factory()
	}
}

// Put 将对象归还到池
func (p *BoundedPool) Put(obj interface{}) {
	if obj == nil {
		return
	}

	if p.reset != nil {
		p.reset(obj)
	}

	if p.stats != nil {
		atomic.AddInt64(&p.stats.Puts, 1)
	}

	select {
	case p.pool <- obj:
		// 成功放入池中
	default:
		// 池已满，丢弃对象让GC回收
		if p.stats != nil {
			atomic.AddInt64(&p.stats.Drops, 1)
		}
	}
}

// GetStats 获取池统计信息
func (p *BoundedPool) GetStats() map[string]int64 {
	if p.stats == nil {
		return map[string]int64{
			"hits":   0,
			"misses": 0,
			"puts":   0,
			"drops":  0,
		}
	}
	return p.stats.GetStats()
}

// GetHitRate 获取命中率
func (p *BoundedPool) GetHitRate() float64 {
	if p.stats == nil {
		return 0
	}
	return p.stats.GetHitRate()
}

// Size 获取池的容量
func (p *BoundedPool) Size() int {
	return cap(p.pool)
}

// Len 获取池中当前对象数量
func (p *BoundedPool) Len() int {
	return len(p.pool)
}

// Close 关闭对象池
func (p *BoundedPool) Close() {
	close(p.pool)
}
