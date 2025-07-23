package capture

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/reassembly"
)

// TCPAssembler TCP流重整器
type TCPAssembler struct {
	assembler     *reassembly.Assembler
	streamPool    *reassembly.StreamPool
	streamFactory *LengthFieldBaseStreamFactory

	// 零拷贝缓冲区
	packetChan chan *types.PacketInfo
	workers    int

	// 本机地址配置
	localAddresses map[string]bool

	// 输出通道
	frameChan chan *types.RawFrameData

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// LengthFieldBaseStream 实现 reassembly.Stream 接口
type LengthFieldBaseStream struct {
	key        string
	net        gopacket.Flow
	transport  gopacket.Flow
	connection types.Connection

	// 基于长度字段的解码器
	decoder *LengthFieldBaseDecoder

	// 本机地址映射
	localAddresses map[string]bool

	// 流状态
	lastSeen time.Time
	closed   bool

	// 零拷贝缓冲区
	ringBuffer *utils.RingBuffer

	// 输出通道
	frameChan chan *types.RawFrameData

	// 同步
	mu sync.Mutex
}

// LengthFieldBaseStreamFactory 实现 reassembly.StreamFactory
type LengthFieldBaseStreamFactory struct {
	localAddresses map[string]bool
	frameChan      chan *types.RawFrameData
}

// NewTCPAssembler 创建TCP重整器
func NewTCPAssembler(workers int, localAddresses []string) *TCPAssembler {
	// 构建本机地址映射
	localAddrMap := make(map[string]bool)
	for _, addr := range localAddresses {
		localAddrMap[addr] = true
	}

	// 创建输出通道
	frameChan := make(chan *types.RawFrameData, 1000)

	// 创建 StreamFactory
	factory := &LengthFieldBaseStreamFactory{
		localAddresses: localAddrMap,
		frameChan:      frameChan,
	}

	ta := &TCPAssembler{
		streamFactory:  factory,
		packetChan:     make(chan *types.PacketInfo, 1000),
		frameChan:      frameChan,
		workers:        workers,
		localAddresses: localAddrMap,
	}

	ta.streamPool = reassembly.NewStreamPool(factory)
	ta.assembler = reassembly.NewAssembler(ta.streamPool)

	// 配置assembler
	ta.assembler.MaxBufferedPagesPerConnection = 16
	ta.assembler.MaxBufferedPagesTotal = 1000

	return ta
}

// GetFrameChannel 获取帧输出通道
func (ta *TCPAssembler) GetFrameChannel() <-chan *types.RawFrameData {
	return ta.frameChan
}

// ProcessPacket 处理数据包
func (ta *TCPAssembler) ProcessPacket(packet *types.PacketInfo) {
	select {
	case ta.packetChan <- packet:
	default:
		// 通道满，丢弃包
	}
}

// Start 启动工作协程
func (ta *TCPAssembler) Start(ctx context.Context) {
	ta.ctx, ta.cancel = context.WithCancel(ctx)

	// 启动工作协程
	for i := 0; i < ta.workers; i++ {
		ta.wg.Add(1)
		go ta.worker()
	}

	// 启动清理协程
	ta.wg.Add(1)
	go ta.cleanup()
}

// Stop 停止重整器
func (ta *TCPAssembler) Stop() {
	if ta.cancel != nil {
		ta.cancel()
	}
	ta.wg.Wait()
	close(ta.frameChan)
}

// worker 工作协程
func (ta *TCPAssembler) worker() {
	defer ta.wg.Done()

	for {
		select {
		case <-ta.ctx.Done():
			return
		case packet, ok := <-ta.packetChan:
			if !ok {
				return
			}
			ta.processPacket(packet)
		}
	}
}

// processPacket 处理单个数据包
func (ta *TCPAssembler) processPacket(packet *types.PacketInfo) {
	if packet.TCPLayer == nil || packet.IPLayer == nil {
		return
	}

	// 创建包含时间戳的 AssemblerContext
	ctx := &assemblerSimpleContext{
		Timestamp: packet.Timestamp,
	}

	// 使用 AssembleWithContext 方法
	ta.assembler.AssembleWithContext(
		packet.IPLayer.NetworkFlow(),
		packet.TCPLayer,
		ctx,
	)
}

// assemblerSimpleContext 实现 AssemblerContext 接口
type assemblerSimpleContext struct {
	Timestamp time.Time
}

func (asc *assemblerSimpleContext) GetCaptureInfo() gopacket.CaptureInfo {
	return gopacket.CaptureInfo{
		Timestamp: asc.Timestamp,
	}
}

// cleanup 清理过期连接
func (ta *TCPAssembler) cleanup() {
	defer ta.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ta.ctx.Done():
			return
		case <-ticker.C:
			// 更精细的清理选项
			ta.assembler.FlushWithOptions(reassembly.FlushOptions{
				T:  time.Now().Add(-2 * time.Minute), // 2分钟前的数据强制flush
				TC: time.Now().Add(-5 * time.Minute), // 5分钟前的连接关闭
			})
		}
	}
}

// New 实现 reassembly.StreamFactory 接口
func (lsf *LengthFieldBaseStreamFactory) New(net, transport gopacket.Flow, tcp *layers.TCP, ac reassembly.AssemblerContext) reassembly.Stream {
	// 解析端口号
	srcPort := int(transport.Src().Raw()[0])<<8 + int(transport.Src().Raw()[1])
	dstPort := int(transport.Dst().Raw()[0])<<8 + int(transport.Dst().Raw()[1])

	// 创建连接信息
	connection := types.Connection{
		SrcHost:  net.Src().String(),
		SrcPort:  srcPort,
		DestHost: net.Dst().String(),
		DstPort:  dstPort,
		Protocol: types.TCP,
	}

	stream := &LengthFieldBaseStream{
		key:            net.String() + ":" + transport.String(),
		net:            net,
		transport:      transport,
		connection:     connection,
		localAddresses: lsf.localAddresses,
		frameChan:      lsf.frameChan,
		lastSeen:       time.Now(),
		ringBuffer:     utils.NewRingBuffer(1024), // 每个流独立的零拷贝缓冲区
	}

	// 创建基于长度字段的解码器
	stream.decoder = NewLengthFieldBaseDecoder(connection)

	// 启动帧转发协程
	go stream.handleFrames()

	return stream
}

// Accept 实现 reassembly.Stream 接口
func (s *LengthFieldBaseStream) Accept(tcp *layers.TCP, ci gopacket.CaptureInfo, dir reassembly.TCPFlowDirection, nextSeq reassembly.Sequence, start *bool, ac reassembly.AssemblerContext) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return false
	}

	// 更新最后访问时间
	s.lastSeen = ci.Timestamp

	// 接受所有TCP数据包
	return true
}

// ReassembledSG 实现 reassembly.Stream 接口
func (s *LengthFieldBaseStream) ReassembledSG(sg reassembly.ScatterGather, ac reassembly.AssemblerContext) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	// 获取重组数据的长度
	length, _ := sg.Lengths()
	if length == 0 {
		return
	}

	// 使用零拷贝获取数据
	data := sg.Fetch(length)
	if len(data) == 0 {
		return
	}

	// 使用 RingBuffer 进行零拷贝写入
	dataPtr := s.ringBuffer.ZeroCopyWrite(data)
	if dataPtr == nil {
		return
	}

	// 判断数据方向
	direction := s.determineDirection()

	// 向解码器输入数据流
	s.decoder.Feed(*dataPtr, direction)

	// 更新最后访问时间
	s.lastSeen = ac.GetCaptureInfo().Timestamp
}

// ReassemblyComplete 实现 reassembly.Stream 接口
func (s *LengthFieldBaseStream) ReassemblyComplete(ac reassembly.AssemblerContext) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true

	// 关闭解码器
	if s.decoder != nil {
		s.decoder.Close()
	}

	// 返回 true 表示可以从连接池中移除
	return true
}

// handleFrames 处理解码器输出的帧
func (s *LengthFieldBaseStream) handleFrames() {
	for frame := range s.decoder.FrameChan {
		select {
		case s.frameChan <- frame:
		default:
			// 输出通道满，丢弃帧
		}
	}
}

// determineDirection 基于本机地址判断数据方向
func (s *LengthFieldBaseStream) determineDirection() string {
	srcIP := s.net.Src().String()
	dstIP := s.net.Dst().String()

	// 检查源地址是否为本机地址
	if s.localAddresses[srcIP] {
		// 源地址是本机，这是出站数据
		return "outbound"
	}

	// 检查目标地址是否为本机地址
	if s.localAddresses[dstIP] {
		// 目标地址是本机，这是入站数据
		return "inbound"
	}

	// 都不是本机地址，可能是转发的数据包，默认为入站
	return "inbound"
}

// GetStats 获取流统计信息
func (s *LengthFieldBaseStream) GetStats() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := s.decoder.GetStats()
	stats["connection"] = s.connection
	stats["last_seen"] = s.lastSeen
	stats["closed"] = s.closed
	stats["key"] = s.key
	return stats
}

// GetLocalAddresses 获取本机IP地址
func GetLocalAddresses() []string {
	var addresses []string

	interfaces, err := net.Interfaces()
	if err != nil {
		return addresses
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // 接口未启用
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					addresses = append(addresses, ipnet.IP.String())
				}
			}
		}
	}

	// 添加回环地址
	addresses = append(addresses, "127.0.0.1", "::1")

	return addresses
}
