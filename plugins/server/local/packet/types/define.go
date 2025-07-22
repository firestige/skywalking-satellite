package types

import (
	"context"
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

type Lifecycle interface {
	Prepare(ctx context.Context) error
	Start() error
	Close() error
}

type RawFrameData struct {
	Data       []byte
	Meta       map[string]string
	Connection Connection
	Timestamp  int64
	Direction  string // "inbound" or "outbound"
}

var EmptyRawFrameData = &RawFrameData{}

type Connection struct {
	SrcHost  string
	SrcPort  int
	DestHost string
	DstPort  int
	Protocol string
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
	Lifecycle
	Fetch(ctx context.Context) (*RawFrameData, error)
}

type PacketStream interface {
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
