package capture

import (
	"fmt"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/google/gopacket/afpacket"
)

// createHandle 创建并配置AF_PACKET句柄
func (nc *networkCapture) createHandle() (*afpacket.TPacket, error) {
	handle, err := afpacket.NewTPacket(
		afpacket.OptInterface(nc.options.Interface),
		afpacket.OptFrameSize(nc.options.SnapLen),
		afpacket.OptNumBlocks(nc.options.NumBlocks),
		afpacket.OptBlockSize(nc.options.BlockSize),
		afpacket.OptPollTimeout(nc.options.FlushTimeout),
		afpacket.TPacketVersion3,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AF_PACKET handle: %w", err)
	}

	// 设置BPF过滤器
	if nc.bpfFilter == nil {
		log.Logger.Warn("No BPF filter set, capturing all packets")
	} else {
		log.Logger.Infof("Setting BPF filter: %s", nc.options.Filter)
		if err := handle.SetBPF(nc.bpfFilter); err != nil {
			handle.Close()
			return nil, fmt.Errorf("failed to set BPF filter: %w", err)
		}
	}

	return handle, nil
}

// cleanup 清理资源
func (nc *networkCapture) cleanup() {
	if nc.handle != nil {
		nc.handle.Close()
		nc.handle = nil
	}

	if nc.poolManager != nil {
		nc.poolManager.close()
	}

	if nc.framePipeline != nil {
		close(nc.framePipeline)
	}

	// 打印最终统计信息
	if nc.poolManager != nil {
		hitRate := nc.poolManager.GetHitRate()
		stats := nc.poolManager.GetStats()
		log.Logger.Infof("Final pool stats - Hit rate: %.2f%%, Hits: %d, Misses: %d, Puts: %d, Drops: %d",
			hitRate, stats["hits"], stats["misses"], stats["puts"], stats["drops"])
	}
}
