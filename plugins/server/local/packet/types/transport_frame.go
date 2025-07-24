package types

import (
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// TransportType 传输层类型
type TransportType int

const (
	TransportUnknown TransportType = iota
	TransportTCP
	TransportUDP
	TransportICMP
)

func (t TransportType) String() string {
	switch t {
	case TransportTCP:
		return "TCP"
	case TransportUDP:
		return "UDP"
	case TransportICMP:
		return "ICMP"
	default:
		return "UNKNOWN"
	}
}

// FrameState 帧处理状态
type FrameState int

const (
	StateRaw         FrameState = iota // 原始状态
	StateParsed                        // 已解析
	StateReassembled                   // 已重组（TCP）
	StateReady                         // 准备输出
)

// TransportLayers 传输层解析信息
type TransportLayers struct {
	Ethernet *layers.Ethernet
	IPv4     *layers.IPv4
	IPv6     *layers.IPv6
	TCP      *layers.TCP
	UDP      *layers.UDP
	ICMP     *layers.ICMPv4
}

// TransportInfo 传输层连接信息
type TransportInfo struct {
	Type      string // TCP, UDP, ICMP
	SrcIP     string
	DstIP     string
	SrcPort   int
	DstPort   int
	IPVersion int
	Direction string // inbound/outbound
}

// GetConnectionKey 获取连接标识
func (t *TransportInfo) GetConnectionKey() string {
	return fmt.Sprintf("%s:%d-%s:%d", t.SrcIP, t.SrcPort, t.DstIP, t.DstPort)
}

// TransportFrame 统一的传输层帧结构
type TransportFrame struct {
	// 原始数据
	RawPacket gopacket.Packet
	Timestamp time.Time

	// 解析后的层信息（可选，用于内部处理）
	Layers TransportLayers

	// 传输层信息
	Transport TransportInfo

	// 应用层载荷
	Payload []byte

	// 元数据
	Meta map[string]string

	// 处理状态
	State FrameState
}

// ToRawFrameData 转换为输出格式（兼容现有接口）
func (tf *TransportFrame) ToRawFrameData() *RawFrameData {
	return &RawFrameData{
		Data: tf.Payload,
		Meta: tf.Meta,
		Connection: Connection{
			SrcHost:  tf.Transport.SrcIP,
			SrcPort:  tf.Transport.SrcPort,
			DestHost: tf.Transport.DstIP,
			DstPort:  tf.Transport.DstPort,
			Protocol: tf.Transport.Type,
		},
		Timestamp: tf.Timestamp.UnixNano() / 1e6,
		Direction: tf.Transport.Direction,
	}
}
