package capture

import (
	"context"
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
)

// poolManager 对象池管理器（简化版）
type poolManager struct {
	options   *Options
	framePool *utils.BoundedPool // 统一的帧池
	poolStats *utils.PoolStats
}

// newPoolManager 创建池管理器
func newPoolManager(options *Options) *poolManager {
	return &poolManager{
		options:   options,
		poolStats: &utils.PoolStats{},
	}
}

// init 初始化对象池
func (pm *poolManager) init() {
	poolSize := pm.calculatePoolSize()

	pm.framePool = utils.NewBoundedPoolWithStats(
		poolSize,
		func() interface{} {
			return &types.TransportFrame{
				Meta: make(map[string]string),
			}
		},
		func(obj interface{}) {
			if frame, ok := obj.(*types.TransportFrame); ok {
				pm.resetTransportFrame(frame)
			}
		},
		pm.poolStats,
	)

	log.Logger.Infof("Initialized unified frame pool with size: %d", poolSize)
}

// calculatePoolSize 计算合理的池大小
func (pm *poolManager) calculatePoolSize() int {
	baseSize := pm.options.WorkerCount * 2
	channelSize := pm.options.RingSize

	poolSize := baseSize
	if channelSize > baseSize {
		poolSize = channelSize
	}

	const (
		minPoolSize = 16
		maxPoolSize = 1024
	)

	if poolSize < minPoolSize {
		poolSize = minPoolSize
	} else if poolSize > maxPoolSize {
		poolSize = maxPoolSize
	}

	return poolSize
}

// getTransportFrame 从池中获取TransportFrame
func (pm *poolManager) getTransportFrame() *types.TransportFrame {
	return pm.framePool.Get().(*types.TransportFrame)
}

// putTransportFrame 将TransportFrame归还到池
func (pm *poolManager) putTransportFrame(frame *types.TransportFrame) {
	if frame == nil {
		return
	}
	pm.framePool.Put(frame)
}

// resetTransportFrame 重置TransportFrame
func (pm *poolManager) resetTransportFrame(frame *types.TransportFrame) {
	frame.RawPacket = nil
	frame.Timestamp = time.Time{}
	frame.Layers = types.TransportLayers{}
	frame.Transport = types.TransportInfo{}
	frame.Payload = frame.Payload[:0] // 保留容量
	frame.State = types.StateRaw

	// 清空Meta
	for k := range frame.Meta {
		delete(frame.Meta, k)
	}
}

// GetStats 获取池统计信息
func (pm *poolManager) GetStats() map[string]int64 {
	if pm.poolStats != nil {
		return pm.poolStats.GetStats()
	}
	return map[string]int64{}
}

// GetHitRate 获取池命中率
func (pm *poolManager) GetHitRate() float64 {
	if pm.poolStats != nil {
		return pm.poolStats.GetHitRate()
	}
	return 0
}

// startStatsLogger 启动统计日志
func (pm *poolManager) startStatsLogger(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hitRate := pm.GetHitRate()
			stats := pm.GetStats()
			log.Logger.Infof("Pool stats - Hit rate: %.2f%%, Hits: %d, Misses: %d, Puts: %d, Drops: %d",
				hitRate, stats["hits"], stats["misses"], stats["puts"], stats["drops"])
		}
	}
}

// close 关闭对象池
func (pm *poolManager) close() {
	if pm.framePool != nil {
		pm.framePool.Close()
	}
}
