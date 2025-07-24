package capture

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
)

// NetworkCaptureBuilder 构建器
type NetworkCaptureBuilder struct {
	options *Options
	ctx     context.Context
}

// NewNetworkCaptureBuilder 创建构建器
func NewNetworkCaptureBuilder(ctx context.Context) *NetworkCaptureBuilder {
	return &NetworkCaptureBuilder{
		options: DefaultOptions(),
		ctx:     ctx,
	}
}

// WithInterface 设置网络接口
func (b *NetworkCaptureBuilder) WithInterface(iface string) *NetworkCaptureBuilder {
	b.options.Interface = iface
	return b
}

// WithBPFFilter 设置BPF过滤器
func (b *NetworkCaptureBuilder) WithFilter(filter string) *NetworkCaptureBuilder {
	b.options.Filter = filter
	return b
}

// WithRingSize 设置环形缓冲区大小
func (b *NetworkCaptureBuilder) WithRingSize(size int) *NetworkCaptureBuilder {
	b.options.RingSize = size
	return b
}

// WithWorkerCount 设置工作协程数量
func (b *NetworkCaptureBuilder) WithWorkerCount(count int) *NetworkCaptureBuilder {
	b.options.WorkerCount = count
	return b
}

// WithMTU 设置MTU
func (b *NetworkCaptureBuilder) WithMTU(mtu int) *NetworkCaptureBuilder {
	b.options.MTU = mtu
	return b
}

// WithLocalAddresses 设置本机地址
func (b *NetworkCaptureBuilder) WithLocalAddresses(addresses []string) *NetworkCaptureBuilder {
	b.options.LocalAddresses = addresses
	return b
}

// Build 构建DataSource
func (b *NetworkCaptureBuilder) Build() (types.DataSource, error) {
	// 验证配置
	if err := b.options.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}
	return newNetworkCapture(b.options, b.ctx), nil
}

func (b *NetworkCaptureBuilder) String() string {
	jsonBytes, err := json.Marshal(b.options)
	if err != nil {
		// 如果JSON序列化失败，返回错误信息
		return fmt.Sprintf(`{"error":"failed to marshal options: %v"}`, err)
	}

	return string(jsonBytes)
}
