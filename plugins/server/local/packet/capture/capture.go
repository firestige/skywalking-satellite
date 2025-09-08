package capture

import (
	"context"
	"sync"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/google/gopacket/afpacket"
)

// networkCapture 网络抓包实现
type networkCapture struct {
	// 配置信息
	config *CaptureConfig

	// 核心组件 - 网络层
	handle *afpacket.TPacket // AF_PACKET句柄

	// 并发控制
	ctx context.Context
	wg  *sync.WaitGroup
	mu  *sync.RWMutex

	// 数据通道
	tcpChan chan *types.RawFrameData // TCP重整器输出通道
	udpChan chan *types.RawFrameData // UDP处理输出通道

	// 处理器
	frameConsumer func(*types.RawFrameData)
}

// newNetworkCapture 创建网络抓包实例
func newNetworkCapture(config *CaptureConfig, ctx context.Context) *networkCapture {
	return &networkCapture{
		config: config,
		mu:     &sync.RWMutex{},
		ctx:    ctx,
		wg:     &sync.WaitGroup{},
	}
}

// Prepare 准备抓包环境
func (nc *networkCapture) Prepare() error {
	nc.tcpChan = make(chan *types.RawFrameData, nc.config.TCPChanSize)
	nc.udpChan = make(chan *types.RawFrameData, nc.config.UDPChanSize)
	nc.frameConsumer = nc.config.handler
	return nil
}

// Start 启动抓包
func (nc *networkCapture) Start() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	log.Logger.Info("Starting network capture...")

	// first start worker goroutines
	// tcp
	// for i := 0; i < nc.config.TCPWorkers; i++ {
	// 	nc.wg.Add(1)
	// 	go nc.tcpProcessLoop()
	// }

	// for i := 0; i < nc.config.UDPWorkers; i++ {
	// 	nc.wg.Add(1)
	// 	go nc.udpProcessLoop()
	// }

	go nc.udpProcessLoop()

	// second start main capture loop
	nc.wg.Add(1)
	go nc.captureLoop()

	log.Logger.Info("Network capture started successfully")
	return nil
}

// Close 关闭抓包
func (nc *networkCapture) Close() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	log.Logger.Info("Closing network capture...")

	close(nc.tcpChan)
	close(nc.udpChan)

	// 等待所有协程结束
	nc.wg.Wait()

	// 关闭AF_PACKET句柄
	if nc.handle != nil {
		nc.handle.Close()
		nc.handle = nil
	}

	log.Logger.Info("Network capture closed successfully")
	return nil
}
