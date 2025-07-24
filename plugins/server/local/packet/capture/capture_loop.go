package capture

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// captureLoop 主抓包循环（重构版）
func (nc *networkCapture) captureLoop() {
	defer nc.wg.Done()

	packetSource := gopacket.NewPacketSource(nc.handle, layers.LinkTypeEthernet)
	packetSource.DecodeOptions.Lazy = true
	packetSource.DecodeOptions.NoCopy = false

	for {
		select {
		case <-nc.ctx.Done():
			log.Logger.Info("Capture loop stopping...")
			return
		case packet, ok := <-packetSource.Packets():
			if !ok {
				log.Logger.Info("Packet source closed")
				return
			}

			if packet == nil {
				continue
			}

			// 直接创建TransportFrame并发送到统一管道
			frame := nc.createTransportFrame(packet)
			if frame != nil {
				nc.sendToFramePipeline(frame)
			}
		}
	}
}
