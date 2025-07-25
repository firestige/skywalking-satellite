package types

import (
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const (
	TCP = "TCP"
	UDP = "UDP"
	SIP = "SIP"
	ESL = "ESL"
)

// Direction represents packet direction
type Direction string

const (
	Inbound  Direction = "inbound"
	Outbound Direction = "outbound"
	Unknown  Direction = "unknown"
)

type RawFrameData struct {
	Data       []byte
	Meta       map[string]string
	Connection Connection
	Timestamp  int64
	Direction  Direction // "inbound" or "outbound"
}

var EmptyRawFrameData = &RawFrameData{}

type Connection struct {
	SrcHost  string
	SrcPort  int
	DestHost string
	DstPort  int
	Protocol layers.IPProtocol
}

// PacketInfo 包信息
type PacketInfo struct {
	Timestamp time.Time
	Packet    gopacket.Packet

	// 零拷贝字段
	EthLayer *layers.Ethernet
	IPLayer  *layers.IPv4
	TCPLayer *layers.TCP
	Payload  []byte
}

type DataSource interface {
	Prepare() error
	Start() error
	Close() error
}

type FrameFilter interface {
	Filter(frame *RawFrameData, chain FrameFilterChain)
}

type FrameFilterChain interface {
	Filter(frame *RawFrameData)
}

type FrameHandler interface {
	Handle(frame *RawFrameData) error
}
