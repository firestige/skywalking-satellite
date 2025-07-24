package capture

import (
	"fmt"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/google/gopacket"
)

// startFramePipeline 启动统一的帧处理流水线
func (nc *networkCapture) startFramePipeline() {
	// 启动帧处理工作协程
	for i := 0; i < nc.options.WorkerCount; i++ {
		nc.wg.Add(1)
		go nc.frameProcessor()
	}
}

// createTransportFrame 创建传输层帧
func (nc *networkCapture) createTransportFrame(packet gopacket.Packet) *types.TransportFrame {
	frame := nc.poolManager.getTransportFrame() // 从池中获取

	// 基本信息
	frame.RawPacket = packet
	frame.Timestamp = packet.Metadata().Timestamp
	frame.State = types.StateRaw

	return frame
}

// frameProcessor 帧处理器（统一处理流水线）
func (nc *networkCapture) frameProcessor() {
	defer nc.wg.Done()

	for {
		select {
		case <-nc.ctx.Done():
			return
		case frame, ok := <-nc.framePipeline:
			if !ok {
				return
			}

			nc.processTransportFrame(frame)
		}
	}
}

// processTransportFrame 处理传输层帧
func (nc *networkCapture) processTransportFrame(frame *types.TransportFrame) {
	defer nc.poolManager.putTransportFrame(frame)

	switch frame.State {
	case types.StateRaw:
		// 解析原始数据包
		nc.parseTransportFrame(frame)
		nc.sendToFramePipeline(frame) // 重新放入管道继续处理

	case types.StateParsed:
		// 根据传输层类型处理
		switch frame.Transport.Type {
		case "TCP":
			// TCP需要重组，暂时直接输出（简化实现）
			frame.State = types.StateReady
			nc.sendToFramePipeline(frame)
		case "UDP":
			// UDP直接输出
			frame.State = types.StateReady
			nc.sendToFramePipeline(frame)
		}

	case types.StateReassembled:
		// TCP重组完成，准备输出
		frame.State = types.StateReady
		nc.sendToFramePipeline(frame)

	case types.StateReady:
		// 已准备好输出，不需要进一步处理
		// 由Fetch()方法消费
	}
}

// parseTransportFrame 解析传输层帧
func (nc *networkCapture) parseTransportFrame(frame *types.TransportFrame) {
	parsed := nc.transportParser.Parse(frame.RawPacket)

	// 复制解析结果
	frame.Layers = parsed.Layers
	frame.Transport = nc.transportParser.GetTransportInfo(parsed)
	frame.Payload = parsed.Payload
	frame.State = types.StateParsed

	// 设置元数据
	if frame.Meta == nil {
		frame.Meta = make(map[string]string)
	}
	frame.Meta["transport"] = frame.Transport.Type
	frame.Meta["payload_size"] = fmt.Sprintf("%d", len(frame.Payload))
	frame.Meta["capture_time"] = fmt.Sprintf("%d", frame.Timestamp.UnixNano()/1e6)
}

// sendToFramePipeline 发送帧到处理管道
func (nc *networkCapture) sendToFramePipeline(frame *types.TransportFrame) {
	select {
	case nc.framePipeline <- frame:
		// 成功发送
	case <-nc.ctx.Done():
		nc.poolManager.putTransportFrame(frame)
	default:
		// 管道满，归还到池并丢弃
		nc.poolManager.putTransportFrame(frame)
	}
}
