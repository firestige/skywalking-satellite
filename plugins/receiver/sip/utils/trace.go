package utils

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

type SegmentIDGenerator struct {
	instanceId string
	sequence   int64
	mutex      *sync.Mutex
}

func NewSegmentIDGenerator(instanceId string) *SegmentIDGenerator {
	return &SegmentIDGenerator{
		instanceId: instanceId,
		sequence:   0,
		mutex:      &sync.Mutex{},
	}
}

func (g *SegmentIDGenerator) Generate() string {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// 获取当前goroutine ID (或使用固定ID)
	goroutineID := runtime.NumGoroutine() // 简化版本

	// 获取当前时间戳 (毫秒)
	timestamp := time.Now().UnixNano() / 1e6

	// 递增序列号
	g.sequence++

	// 生成segment ID: instanceId.goroutineId.timestamp.sequence
	return fmt.Sprintf("%s.%d.%d.%d",
		g.instanceId,
		goroutineID,
		timestamp,
		g.sequence)
}
