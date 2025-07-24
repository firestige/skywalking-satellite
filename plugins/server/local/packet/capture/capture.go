package capture

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
	"github.com/google/gopacket/afpacket"
	"golang.org/x/net/bpf"
)

// CaptureState 表示抓包器的状态
type CaptureState int32

const (
	StateInit CaptureState = iota
	StateReady
	StateRunning
	StateClosed
)

func (s CaptureState) String() string {
	switch s {
	case StateInit:
		return "Init"
	case StateReady:
		return "Ready"
	case StateRunning:
		return "Running"
	case StateClosed:
		return "Closed"
	default:
		return "Unknown"
	}
}

// networkCapture 网络抓包实现
type networkCapture struct {
	options *Options
	state   int32

	// 核心组件
	bpfFilter       []bpf.RawInstruction
	handle          *afpacket.TPacket
	transportParser *TransportParser

	// 并发控制
	ctx context.Context
	wg  *sync.WaitGroup
	mu  *sync.RWMutex

	// 统一的数据管道
	framePipeline chan *types.TransportFrame // 唯一管道

	// 对象池管理器
	poolManager *poolManager
}

// newNetworkCapture 创建网络抓包实例
func newNetworkCapture(options *Options, ctx context.Context) *networkCapture {
	return &networkCapture{
		options:         options,
		state:           int32(StateInit),
		mu:              &sync.RWMutex{},
		ctx:             ctx,
		wg:              &sync.WaitGroup{},
		transportParser: NewTransportParser(options),
		framePipeline:   make(chan *types.TransportFrame, options.RingSize), // 单一管道
		poolManager:     newPoolManager(options),
	}
}

// GetState 获取当前状态
func (nc *networkCapture) GetState() CaptureState {
	return CaptureState(atomic.LoadInt32(&nc.state))
}

// setState 设置状态
func (nc *networkCapture) setState(newState CaptureState) {
	oldState := CaptureState(atomic.SwapInt32(&nc.state, int32(newState)))
	log.Logger.WithField("capture", "state").
		Infof("State transition: %s -> %s", oldState, newState)
}

// checkStateTransition 检查状态转换
func (nc *networkCapture) checkStateTransition(from, to CaptureState) error {
	currentState := nc.GetState()
	if currentState != from {
		return fmt.Errorf("invalid state transition: expected %s, got %s", from, currentState)
	}
	return nil
}

// Prepare 准备抓包环境
func (nc *networkCapture) Prepare() error {
	if err := nc.checkStateTransition(StateInit, StateReady); err != nil {
		return err
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	log.Logger.Info("Preparing network capture...")

	// 初始化BPF过滤器
	if nc.options.Filter != "" {
		bpfFilter, _ := utils.NewBPFCompiler(nc.options.SnapLen).CompileFilter(nc.options.Filter)
		nc.bpfFilter = bpfFilter
	}

	// 初始化对象池
	nc.poolManager.init()

	nc.setState(StateReady)
	log.Logger.Info("Network capture prepared successfully")
	return nil
}

// Start 启动抓包
func (nc *networkCapture) Start() error {
	if err := nc.checkStateTransition(StateReady, StateRunning); err != nil {
		return err
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	log.Logger.Info("Starting network capture...")

	// 启动帧处理流水线
	nc.startFramePipeline()

	// 创建并配置句柄
	handle, err := nc.createHandle()
	if err != nil {
		return err
	}
	nc.handle = handle

	// 启动抓包循环
	nc.wg.Add(1)
	go nc.captureLoop()

	nc.setState(StateRunning)
	log.Logger.Info("Network capture started successfully")
	return nil
}

// Close 关闭抓包
func (nc *networkCapture) Close() error {
	currentState := nc.GetState()
	if currentState == StateClosed {
		return nil
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	log.Logger.Info("Closing network capture...")
	nc.setState(StateClosed)

	// 等待协程结束
	nc.wg.Wait()

	// 清理资源
	nc.cleanup()

	log.Logger.Info("Network capture closed successfully")
	return nil
}

// Fetch 获取处理后的帧数据（统一接口）
func (nc *networkCapture) Fetch() (*types.RawFrameData, error) {
	if nc.GetState() != StateRunning {
		return types.EmptyRawFrameData, fmt.Errorf("capture not running, current state: %s", nc.GetState())
	}

	select {
	case <-nc.ctx.Done():
		return types.EmptyRawFrameData, nc.ctx.Err()
	case frame, ok := <-nc.framePipeline:
		if !ok {
			return types.EmptyRawFrameData, fmt.Errorf("frame pipeline closed")
		}
		// 只有状态为Ready的帧才输出
		if frame.State == types.StateReady {
			return frame.ToRawFrameData(), nil
		}
		// 如果不是Ready状态，继续等待下一个帧
		return nc.Fetch()
	}
}
