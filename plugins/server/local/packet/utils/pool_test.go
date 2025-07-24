package utils

import (
	"sync"
	"testing"
	"time"
)

func TestBoundedPool(t *testing.T) {
	// 测试基本功能
	pool := NewBoundedPool(2, func() interface{} {
		return make([]byte, 10)
	}, func(obj interface{}) {
		if buf, ok := obj.([]byte); ok {
			for i := range buf {
				buf[i] = 0
			}
		}
	})

	// 测试获取对象
	obj1 := pool.Get()
	if obj1 == nil {
		t.Fatal("Expected object, got nil")
	}

	// 测试归还对象
	pool.Put(obj1)

	// 测试统计信息
	stats := pool.GetStats()
	if stats["misses"] != 1 {
		t.Errorf("Expected 1 miss, got %d", stats["misses"])
	}
	if stats["puts"] != 1 {
		t.Errorf("Expected 1 put, got %d", stats["puts"])
	}
}

func TestBoundedPoolConcurrency(t *testing.T) {
	pool := NewBoundedPool(10, func() interface{} {
		return make([]byte, 100)
	}, nil)

	var wg sync.WaitGroup
	goroutines := 100
	operations := 1000

	// 并发获取和归还
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				obj := pool.Get()
				time.Sleep(time.Microsecond)
				pool.Put(obj)
			}
		}()
	}

	wg.Wait()

	stats := pool.GetStats()
	totalOps := int64(goroutines * operations)
	if stats["puts"] != totalOps {
		t.Errorf("Expected %d puts, got %d", totalOps, stats["puts"])
	}
}

func TestPoolStats(t *testing.T) {
	stats := &PoolStats{}

	// 测试初始状态
	if stats.GetHitRate() != 0 {
		t.Errorf("Expected 0 hit rate, got %f", stats.GetHitRate())
	}

	// 模拟一些操作
	stats.Hits = 80
	stats.Misses = 20

	expectedRate := 80.0
	if rate := stats.GetHitRate(); rate != expectedRate {
		t.Errorf("Expected hit rate %f, got %f", expectedRate, rate)
	}
}
