package capture

import (
	"fmt"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
)

// FrameBuilder 帧数据构建器（专注传输层数据组装）
type FrameBuilder struct {
	options *Options
	parser  *TransportParser
}

// NewFrameBuilder 创建帧构建器
func NewFrameBuilder(options *Options) *FrameBuilder {
	return &FrameBuilder{
		options: options,
		parser:  NewTransportParser(options),
	}
}

// BuildUDPFrame 构建UDP帧（传输层载荷）
func (fb *FrameBuilder) BuildUDPFrame(parsed *ParsedTransport) *types.TransportFrame {
	if parsed.TransportType != types.TransportUDP || len(parsed.Payload) == 0 {
		return nil
	}

	transportInfo := fb.parser.GetTransportInfo(parsed)

	frame := &types.TransportFrame{
		RawPacket: parsed.Raw,
		Timestamp: parsed.Raw.Metadata().Timestamp,
		Layers:    parsed.Layers,
		Transport: transportInfo,
		Payload:   parsed.Payload,
		State:     types.StateReady,
		Meta: map[string]string{
			"transport":    "UDP",
			"payload_size": fmt.Sprintf("%d", len(parsed.Payload)),
			"capture_time": fmt.Sprintf("%d", parsed.Timestamp),
		},
	}

	return frame
}

// BuildTCPFrame 构建TCP帧（重组后的TCP流数据）
func (fb *FrameBuilder) BuildTCPFrame(streamData []byte, transportInfo types.TransportInfo, timestamp int64) *types.TransportFrame {
	frame := &types.TransportFrame{
		Transport: transportInfo,
		Payload:   streamData,
		State:     types.StateReady,
		Meta: map[string]string{
			"transport":    "TCP",
			"stream_size":  fmt.Sprintf("%d", len(streamData)),
			"reassembled":  "true",
			"capture_time": fmt.Sprintf("%d", timestamp),
		},
	}

	return frame
}
