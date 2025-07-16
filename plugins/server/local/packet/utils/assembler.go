package utils

import (
	"context"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/reassembly"
)

// TCPAssembler TCP流重整器
type TCPAssembler struct {
	assembler       *reassembly.Assembler
	streamPool      *reassembly.StreamPool
	httpLikeStreams map[string]*HTTPLikeStream
	streamsMutex    sync.RWMutex

	// 零拷贝缓冲区
	packetChan chan *PacketInfo
	workers    int

	ctx context.Context
	wg  sync.WaitGroup
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

// HTTPLikeStreams HTTP流状态
type HTTPLikeStream struct {
	net, transport gopacket.Flow

	// 零拷贝缓冲区
	clientBuffer []byte
	serverBuffer []byte

	// 状态
	isClient bool

	// HTTP解析器
	parser *HTTPLikeParser
}

// NewTCPAssembler 创建TCP重整器
func NewTCPAssembler(workers int) *TCPAssembler {
	streamPool := reassembly.NewStreamPool(&HTTPStreamFactory{})

	ta := &TCPAssembler{
		assembler:       reassembly.NewAssembler(streamPool),
		streamPool:      streamPool,
		httpLikeStreams: make(map[string]*HTTPLikeStream),
		packetChan:      make(chan *PacketInfo, 1000),
		workers:         workers,
	}

	// 配置assembler
	ta.assembler.MaxBufferedPagesPerConnection = 16
	ta.assembler.MaxBufferedPagesTotal = 1000

	return ta
}

// ProcessPacket 处理数据包
func (ta *TCPAssembler) ProcessPacket(packet *PacketInfo) {
	select {
	case ta.packetChan <- packet:
	default:
		// 通道满，丢弃包
	}
}

// Start 启动工作协程
func (ta *TCPAssembler) Start(ctx context.Context) {
	ta.ctx = ctx

	// 启动工作协程
	for i := 0; i < ta.workers; i++ {
		ta.wg.Add(1)
		go ta.worker()
	}

	// 启动清理协程
	ta.wg.Add(1)
	go ta.cleanup()
}

// worker 工作协程
func (ta *TCPAssembler) worker() {
	defer ta.wg.Done()

	for {
		select {
		case <-ta.ctx.Done():
			return
		case packet := <-ta.packetChan:
			ta.processPacket(packet)
		}
	}
}

// processPacket 处理单个数据包
func (ta *TCPAssembler) processPacket(packet *PacketInfo) {
	if packet.TCPLayer == nil {
		return
	}

	// 使用零拷贝方式获取数据
	ta.assembler.AssembleWithTimestamp(
		packet.IPLayer.NetworkFlow(),
		packet.TCPLayer,
		packet.Timestamp,
	)
}

// HTTPStreamFactory 实现reassembly.StreamFactory
type HTTPStreamFactory struct{}

func (hsf *HTTPStreamFactory) New(net, transport gopacket.Flow) reassembly.Stream {
	stream := &HTTPLikeStream{
		net:       net,
		transport: transport,
		parser:    NewHTTPParser(),
	}

	// 预分配缓冲区
	stream.clientBuffer = make([]byte, 0, 64*1024)
	stream.serverBuffer = make([]byte, 0, 64*1024)

	return stream
}

// Reassembled 实现reassembly.Stream接口
func (hs *HTTPLikeStream) Reassembled(reassembled []reassembly.Reassembly) {
	for _, r := range reassembled {
		if r.Seen.Before(time.Now().Add(-5 * time.Minute)) {
			continue // 忽略过期数据
		}

		// 零拷贝追加数据
		if hs.isClient {
			hs.clientBuffer = append(hs.clientBuffer, r.Bytes...)
		} else {
			hs.serverBuffer = append(hs.serverBuffer, r.Bytes...)
		}

		// 尝试解析HTTP
		hs.parser.Parse(r.Bytes, hs.isClient)
	}
}

func (hs *HTTPLikeStream) ReassemblyComplete() {
	// 流结束，清理资源
	hs.clientBuffer = hs.clientBuffer[:0]
	hs.serverBuffer = hs.serverBuffer[:0]
}
