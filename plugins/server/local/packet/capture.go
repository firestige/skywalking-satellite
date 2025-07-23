package packet

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/bpf"
)

// CaptureConfig 配置结构
type CaptureConfig struct {
	Interface      string               // 网络接口名称
	BPFFilter      []bpf.RawInstruction // BPF过滤规则
	SnapLen        int                  // 抓包长度
	RingSize       int                  // 环形缓冲区大小
	WorkerCount    int                  // 工作协程数量
	MTU            int                  // 最大传输单元
	BlockSize      int                  // AF_PACKET块大小
	NumBlocks      int                  // AF_PACKET块数量
	FlushTimeout   time.Duration        // 超时时间
	LocalAddresses []string             // 本机地址列表
}

// DefaultCaptureConfig 默认配置
// todo builder写的有问题，配置没生效，生效的是这里的配置
func DefaultCaptureConfig() *CaptureConfig {
	return &CaptureConfig{
		Interface:      "eth0",
		SnapLen:        65536,
		RingSize:       1024,
		WorkerCount:    4,
		MTU:            1500,
		BlockSize:      1024 * 1024,
		NumBlocks:      128,
		FlushTimeout:   pcap.BlockForever,
		LocalAddresses: GetLocalAddresses(),
	}
}

// networkCapture 网络抓包实现
type networkCapture struct {
	config *CaptureConfig

	// AF_PACKET句柄
	handle *afpacket.TPacket

	// 数据处理组件
	ringBuffer   *utils.RingBuffer
	tcpAssembler *TCPAssembler

	// 工作协程管理
	ctx context.Context
	wg  *sync.WaitGroup

	// 数据通道
	packetChan chan *types.PacketInfo
	frameChan  chan *types.RawFrameData

	// 对象池
	packetPool *sync.Pool
	bufferPool *sync.Pool

	// 状态
	started bool
	mu      *sync.RWMutex
}

// newNetworkCapture 创建网络抓包实例
func newNetworkCapture(config *CaptureConfig, ctx context.Context) *networkCapture {
	nc := &networkCapture{
		config:       config,
		ringBuffer:   utils.NewRingBuffer(config.RingSize),
		packetChan:   make(chan *types.PacketInfo, config.RingSize),
		frameChan:    make(chan *types.RawFrameData, config.RingSize),
		mu:           &sync.RWMutex{},
		ctx:          ctx,
		wg:           &sync.WaitGroup{},
		started:      false,
		tcpAssembler: NewTCPAssembler(config.WorkerCount, config.LocalAddresses),
	}

	// 初始化对象池
	nc.initPools()

	return nc
}

// initPools 初始化对象池
func (nc *networkCapture) initPools() {
	nc.packetPool = &sync.Pool{
		New: func() interface{} {
			return &types.PacketInfo{
				Payload: make([]byte, 0, nc.config.MTU),
			}
		},
	}

	nc.bufferPool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, nc.config.MTU)
		},
	}
}

// Prepare 准备抓包环境
func (nc *networkCapture) Prepare() error {
	log.Logger.WithField("capture", nc.config.Interface).Debug("Preparing network capture...")
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if nc.started {
		return fmt.Errorf("capture already started")
	}

	log.Logger.Infof("Preparing network capture on interface: %s", nc.config.Interface)

	// 创建AF_PACKET句柄
	// OptFrameSize: 设置每个帧的最大大小，相当于传统抓包中的snap length，控制每个数据包的最大捕获长度
	// OptBlockSize: 设置环形缓冲区中每个块的大小，影响内存使用和性能
	// OptNumBlocks: 设置环形缓冲区中块的数量，总缓冲区大小 = BlockSize × NumBlocks
	handle, err := afpacket.NewTPacket(
		afpacket.OptInterface(nc.config.Interface),
		afpacket.OptFrameSize(nc.config.SnapLen), // 使用OptFrameSize代替OptSnapLen
		afpacket.OptNumBlocks(nc.config.NumBlocks),
		afpacket.OptBlockSize(nc.config.BlockSize),
		afpacket.OptPollTimeout(nc.config.FlushTimeout),
		afpacket.TPacketVersion3,
	)
	if err != nil {
		log.Logger.Errorf("Failed to create AF_PACKET handle: %v", err)
		return fmt.Errorf("failed to create AF_PACKET handle: %w", err)
	}

	// 设置BPF过滤器
	if len(nc.config.BPFFilter) > 0 {
		if err := handle.SetBPF(nc.config.BPFFilter); err != nil {
			log.Logger.Errorf("Failed to set BPF filter: %v", err)
			handle.Close()
			return fmt.Errorf("failed to set BPF filter: %w", err)
		}
	}

	nc.handle = handle
	log.Logger.Info("Network capture prepared successfully")
	return nil
}

// Start 启动抓包
func (nc *networkCapture) Start() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if nc.started {
		return fmt.Errorf("capture already started")
	}

	if nc.handle == nil {
		return fmt.Errorf("capture not prepared")
	}

	nc.started = true

	log.Logger.Info("Starting network capture...")

	// 启动TCP重整器
	nc.tcpAssembler.Start(nc.ctx)

	// 启动工作协程
	for i := 0; i < nc.config.WorkerCount; i++ {
		nc.wg.Add(1)
		go nc.packetWorker()
	}

	// 启动帧处理协程
	nc.wg.Add(1)
	go nc.frameWorker()

	// 启动主抓包协程
	nc.wg.Add(1)
	go nc.captureLoop()

	log.Logger.Info("Network capture started successfully")
	return nil
}

// captureLoop 主抓包循环
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

			log.Logger.Debugf("Captured packet: %s", packet)
			if packet == nil {
				continue
			}

			// 从对象池获取PacketInfo
			packetInfo := nc.packetPool.Get().(*types.PacketInfo)
			packetInfo.Timestamp = packet.Metadata().Timestamp
			packetInfo.Packet = packet

			// 解析并缓存各层信息
			nc.parsePacketLayers(packetInfo)

			// 发送到处理队列
			select {
			case nc.packetChan <- packetInfo:
			case <-nc.ctx.Done():
				nc.packetPool.Put(packetInfo)
				return
			default:
				// 队列满，丢弃包
				nc.packetPool.Put(packetInfo)
			}
		}
	}
}

// parsePacketLayers 解析数据包各层信息
func (nc *networkCapture) parsePacketLayers(packetInfo *types.PacketInfo) {
	packet := packetInfo.Packet

	// 解析以太网层
	if ethLayer := packet.Layer(layers.LayerTypeEthernet); ethLayer != nil {
		if eth, ok := ethLayer.(*layers.Ethernet); ok {
			packetInfo.EthLayer = eth
		}
	}

	// 解析IPv4层
	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		if ip, ok := ipLayer.(*layers.IPv4); ok {
			packetInfo.IPLayer = ip
		}
	}

	// 解析TCP层
	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		if tcp, ok := tcpLayer.(*layers.TCP); ok {
			packetInfo.TCPLayer = tcp
			// 获取TCP载荷
			payload := tcp.Payload
			if len(payload) > 0 {
				packetInfo.Payload = packetInfo.Payload[:len(payload)]
				copy(packetInfo.Payload, payload)
			}
		}
	}

	// 解析UDP层（如果需要）
	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		if udp, ok := udpLayer.(*layers.UDP); ok {
			// 获取UDP载荷
			payload := udp.Payload
			if len(payload) > 0 {
				packetInfo.Payload = packetInfo.Payload[:len(payload)]
				copy(packetInfo.Payload, payload)
			}
		}
	}
}

// packetWorker 包处理工作协程
func (nc *networkCapture) packetWorker() {
	defer nc.wg.Done()

	for {
		select {
		case <-nc.ctx.Done():
			return
		case packetInfo, ok := <-nc.packetChan:
			if !ok {
				return
			}

			nc.processPacket(packetInfo)

			// 归还到对象池
			nc.resetPacketInfo(packetInfo)
			nc.packetPool.Put(packetInfo)
		}
	}
}

// resetPacketInfo 重置PacketInfo到初始状态
func (nc *networkCapture) resetPacketInfo(packetInfo *types.PacketInfo) {
	packetInfo.Packet = nil
	packetInfo.EthLayer = nil
	packetInfo.IPLayer = nil
	packetInfo.TCPLayer = nil
	packetInfo.Payload = packetInfo.Payload[:0]
}

// processPacket 处理单个数据包
func (nc *networkCapture) processPacket(packetInfo *types.PacketInfo) {
	// 检查是否有IPv4层
	if packetInfo.IPLayer == nil {
		return
	}

	ip := packetInfo.IPLayer

	switch ip.Protocol {
	case layers.IPProtocolTCP:
		nc.processTCPPacket(packetInfo)
	case layers.IPProtocolUDP:
		nc.processUDPPacket(packetInfo)
	}
}

// processTCPPacket 处理TCP数据包
func (nc *networkCapture) processTCPPacket(packetInfo *types.PacketInfo) {
	if packetInfo.TCPLayer == nil {
		return
	}

	// 发送到TCP重整器
	nc.tcpAssembler.ProcessPacket(packetInfo)
}

// processUDPPacket 处理UDP数据包
func (nc *networkCapture) processUDPPacket(packetInfo *types.PacketInfo) {
	packet := packetInfo.Packet
	ip := packetInfo.IPLayer

	udpLayer := packet.Layer(layers.LayerTypeUDP)
	if udpLayer == nil {
		return
	}

	udp, ok := udpLayer.(*layers.UDP)
	if !ok {
		return
	}

	// 获取UDP载荷
	payload := udp.Payload
	if len(payload) == 0 {
		return
	}

	// 判断数据方向
	direction := nc.determineDirection(ip.SrcIP.String(), ip.DstIP.String())

	// 创建连接信息
	connection := types.Connection{
		SrcHost:  ip.SrcIP.String(),
		SrcPort:  int(udp.SrcPort),
		DestHost: ip.DstIP.String(),
		DstPort:  int(udp.DstPort),
		Protocol: types.UDP,
	}

	// 创建RawFrameData
	frameData := types.RawFrameData{
		Data: payload,
		Meta: map[string]string{
			"protocol": types.UDP,
			"length":   fmt.Sprintf("%d", len(payload)),
		},
		Connection: connection,
		Timestamp:  packetInfo.Timestamp.UnixNano() / 1e6,
		Direction:  direction,
	}

	// 发送到输出通道
	select {
	case nc.frameChan <- &frameData:
	case <-nc.ctx.Done():
		return
	default:
		// 通道满，丢弃帧
	}
}

// frameWorker 帧处理工作协程
func (nc *networkCapture) frameWorker() {
	defer nc.wg.Done()

	// 获取TCP重整器的输出通道
	tcpFrameChan := nc.tcpAssembler.GetFrameChannel()

	for {
		select {
		case <-nc.ctx.Done():
			return
		case frame := <-tcpFrameChan:
			log.Logger.WithField("capture", nc.config.Interface).WithField("frame", frame).Debugf("Processing TCP frame: %s", frame.Meta["protocol"])
			// 转发TCP帧
			select {
			case nc.frameChan <- frame:
			case <-nc.ctx.Done():
				return
			default:
				// 通道满，丢弃帧
			}
		}
	}
}

// determineDirection 判断数据方向
func (nc *networkCapture) determineDirection(srcIP, dstIP string) string {
	for _, localAddr := range nc.config.LocalAddresses {
		if srcIP == localAddr {
			return "outbound"
		}
		if dstIP == localAddr {
			return "inbound"
		}
	}
	return "inbound" // 默认为入站
}

// Fetch 获取处理后的帧数据
func (nc *networkCapture) Fetch() (*types.RawFrameData, error) {
	select {
	case <-nc.ctx.Done():
		return types.EmptyRawFrameData, nc.ctx.Err()
	case frame, ok := <-nc.frameChan:
		if !ok {
			return types.EmptyRawFrameData, fmt.Errorf("frame channel closed")
		}
		log.Logger.WithField("capture", nc.config.Interface).Debugf("Fetched frame: %s", frame.Meta["protocol"])
		return frame, nil
	}
}

// Close 关闭抓包
func (nc *networkCapture) Close() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	log.Logger.Info("Closing network capture...")

	// 等待所有协程结束
	nc.wg.Wait()

	// 停止TCP重整器
	if nc.tcpAssembler != nil {
		nc.tcpAssembler.Stop()
	}

	// 关闭AF_PACKET句柄
	if nc.handle != nil {
		nc.handle.Close()
		nc.handle = nil
	}

	// 关闭通道
	close(nc.packetChan)
	close(nc.frameChan)

	nc.started = false

	log.Logger.Info("Network capture closed successfully")
	return nil
}

// NetworkCaptureBuilder 构建器
type NetworkCaptureBuilder struct {
	config *CaptureConfig
	ctx    context.Context
}

// NewNetworkCaptureBuilder 创建构建器
func NewNetworkCaptureBuilder(ctx context.Context) *NetworkCaptureBuilder {
	return &NetworkCaptureBuilder{
		config: DefaultCaptureConfig(),
		ctx:    ctx,
	}
}

// WithInterface 设置网络接口
func (b *NetworkCaptureBuilder) WithInterface(iface string) *NetworkCaptureBuilder {
	b.config.Interface = iface
	return b
}

// WithBPFFilter 设置BPF过滤器
func (b *NetworkCaptureBuilder) WithBPFFilter(filter []bpf.RawInstruction) *NetworkCaptureBuilder {
	b.config.BPFFilter = filter
	return b
}

// WithRingSize 设置环形缓冲区大小
func (b *NetworkCaptureBuilder) WithRingSize(size int) *NetworkCaptureBuilder {
	b.config.RingSize = size
	return b
}

// WithWorkerCount 设置工作协程数量
func (b *NetworkCaptureBuilder) WithWorkerCount(count int) *NetworkCaptureBuilder {
	b.config.WorkerCount = count
	return b
}

// WithMTU 设置MTU
func (b *NetworkCaptureBuilder) WithMTU(mtu int) *NetworkCaptureBuilder {
	b.config.MTU = mtu
	return b
}

// WithLocalAddresses 设置本机地址
func (b *NetworkCaptureBuilder) WithLocalAddresses(addresses []string) *NetworkCaptureBuilder {
	b.config.LocalAddresses = addresses
	return b
}

// WithSIPFilter 设置SIP过滤器
func (b *NetworkCaptureBuilder) WithSIPFilter(port uint32) *NetworkCaptureBuilder {
	filter, err := utils.PrebuildFilter.SIP(port)
	if err == nil {
		b.config.BPFFilter = filter
	}
	return b
}

// Build 构建DataSource
func (b *NetworkCaptureBuilder) Build() (types.DataSource, error) {
	// 验证配置
	if b.config.Interface == "" {
		return nil, fmt.Errorf("interface is required")
	}

	if b.config.WorkerCount <= 0 {
		b.config.WorkerCount = 4
	}

	if b.config.RingSize <= 0 {
		b.config.RingSize = 1024
	}

	if len(b.config.LocalAddresses) == 0 {
		b.config.LocalAddresses = GetLocalAddresses()
	}

	return newNetworkCapture(b.config, b.ctx), nil
}

func (b *NetworkCaptureBuilder) String() string {
	return fmt.Sprintf("NetworkCaptureBuilder{Interface: %s, RingSize: %d, WorkerCount: %d, MTU: %d, LocalAddresses: %v}",
		b.config.Interface, b.config.RingSize, b.config.WorkerCount, b.config.MTU, b.config.LocalAddresses)
}
