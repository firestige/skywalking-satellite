package capture

import (
	"context"
	"fmt"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
	"golang.org/x/net/bpf"
)

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
